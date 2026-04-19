package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"marrow/internal/config"
	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/githubapi"
	"marrow/internal/search"
	"marrow/internal/server"
	"marrow/internal/service"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: marrow <sync|serve|status|reindex|maintain> [options]")
		os.Exit(1)
	}

	cmd := os.Args[1]
	workDir, _ := os.Getwd()
	cfg, _ := config.Load(workDir)
	if cfg == nil {
		cfg = &config.Config{}
	}

	logger := newLogger(cfg.Server.LogFormat)

	switch cmd {
	case "sync":
		doSync(logger, cfg)
	case "serve":
		doServe(logger, cfg)
	case "status":
		doStatus(logger, cfg)
	case "reindex":
		doReindex(logger, cfg)
	case "maintain":
		doMaintain(logger, cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func doSync(logger *slog.Logger, cfg *config.Config) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	dir := fs.String("dir", cfg.Sync.Dir, "Directory to crawl for markdown files")
	dbPath := fs.String("db", cfg.Server.DB, "Path to SQLite database")
	source := fs.String("source", cfg.Sync.Source, "Source identifier for this directory")
	defaultLang := fs.String("default-lang", cfg.Sync.DefaultLang, "Default language for documents without frontmatter lang")
	all := fs.Bool("all", false, "Sync all sources declared in config")
	if err := fs.Parse(os.Args[2:]); err != nil {
		logger.Error("parse flags", "err", err)
		os.Exit(1)
	}

	database, embedFn := openDBAndEmbed(logger, *dbPath, cfg)
	defer database.Close()

	syncer := &service.Syncer{DB: database, EmbedFn: embedFn}

	if *all {
		ctx := context.Background()
		for _, s := range cfg.Sources {
			if err := syncer.SyncSource(ctx, s, cfg.Search.DefaultLang); err != nil {
				logger.Error("sync source failed", "source", s.Name, "err", err)
			}
		}
		logger.Info("sync all complete")
		return
	}

	ctx := context.Background()
	if err := syncer.SyncLocal(ctx, *source, *defaultLang, *dir); err != nil {
		logger.Error("sync failed", "err", err)
		os.Exit(1)
	}
	logger.Info("sync complete", "dir", *dir, "source", *source)
}

func doServe(logger *slog.Logger, cfg *config.Config) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", cfg.Server.Addr, "HTTP listen address")
	dbPath := fs.String("db", cfg.Server.DB, "Path to SQLite database")
	repoURL := fs.String("repo-url", cfg.GitHub.RepoURL, "GitHub repo URL to clone/pull")
	repoToken := fs.String("repo-token", cfg.GitHub.RepoToken, "GitHub personal access token")
	webhookSecret := fs.String("webhook-secret", cfg.GitHub.WebhookSecret, "Secret key for /webhook endpoint")
	source := fs.String("source", cfg.GitHub.Source, "Source identifier for repo sync state")
	localPath := fs.String("local-path", cfg.GitHub.LocalPath, "Local directory to clone repo into")
	detectLang := fs.Bool("detect-lang", cfg.Search.DetectLang, "Enable automatic language detection for search queries")
	defaultLang := fs.String("default-lang", cfg.Search.DefaultLang, "Default language when detection is disabled or no lang hint is provided")

	ghAppID := fs.Int64("github-app-id", cfg.GitHubApp.AppID, "GitHub App ID")
	ghAppKeyPath := fs.String("github-app-private-key", cfg.GitHubApp.PrivateKey, "Path to GitHub App private key PEM file")
	ghInstallationID := fs.Int64("github-installation-id", cfg.GitHubApp.InstallationID, "GitHub App installation ID (auto-discover if 0)")
	ghRepoOwner := fs.String("github-repo-owner", cfg.GitHubApp.RepoOwner, "GitHub repo owner (defaults to parsing -repo-url)")
	ghRepoName := fs.String("github-repo-name", cfg.GitHubApp.RepoName, "GitHub repo name (defaults to parsing -repo-url)")
	ghWebhookSecret := fs.String("github-webhook-secret", cfg.GitHubApp.WebhookSecret, "GitHub App webhook secret for signature verification")

	if err := fs.Parse(os.Args[2:]); err != nil {
		logger.Error("parse flags", "err", err)
		os.Exit(1)
	}

	database, embedFn := openDBAndEmbed(logger, *dbPath, cfg)
	defer database.Close()

	engine := search.NewEngine(database, embedFn)
	engine.DetectLang = *detectLang
	engine.DefaultLang = *defaultLang

	searcher := &service.Searcher{Engine: engine}
	syncer := &service.Syncer{DB: database, EmbedFn: embedFn}

	// Persist webhook secret / repo config
	if *webhookSecret != "" || *repoURL != "" {
		state := &db.SyncState{
			Source:    *source,
			SecretKey: *webhookSecret,
			RepoURL:   *repoURL,
			LocalPath: *localPath,
			Token:     *repoToken,
		}
		if err := db.NewSyncStateRepo(database).Upsert(context.Background(), state); err != nil {
			logger.Error("save sync state", "err", err)
			os.Exit(1)
		}
	}

	// Resolve owner/repo for GitHub API
	owner, repoName := *ghRepoOwner, *ghRepoName
	if owner == "" || repoName == "" {
		parsedOwner, parsedRepo, err := parseRepoURL(*repoURL)
		if err == nil {
			if owner == "" {
				owner = parsedOwner
			}
			if repoName == "" {
				repoName = parsedRepo
			}
		}
	}

	var ghClient *githubapi.Client
	if *ghAppID > 0 && *ghAppKeyPath != "" {
		key, err := os.ReadFile(*ghAppKeyPath)
		if err != nil {
			logger.Error("read github app private key", "path", *ghAppKeyPath, "err", err)
			os.Exit(1)
		}
		ghClient, err = githubapi.NewClient(*ghAppID, key, *ghInstallationID)
		if err != nil {
			logger.Error("create github app client", "err", err)
			os.Exit(1)
		}
		logger.Info("github app client created", "app_id", *ghAppID, "installation_id", *ghInstallationID)
	}

	// Optional initial git sync in background
	if *repoURL != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := syncer.SyncGit(ctx, *source, *defaultLang, *repoURL, *repoToken, *localPath); err != nil {
				logger.Error("initial git sync failed", "err", err)
			} else {
				logger.Info("initial git sync complete")
			}
		}()
	}

	// Optional initial GitHub API sync in background
	if ghClient != nil && owner != "" && repoName != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if err := syncer.SyncGitHubAPI(ctx, "github-api", *defaultLang, ghClient, owner, repoName); err != nil {
				logger.Error("initial github api sync failed", "err", err)
			} else {
				logger.Info("initial github api sync complete")
			}
		}()
	}

	// Background sync for all config sources
	for _, s := range cfg.Sources {
		s := s
		src := s.Name
		if src == "" {
			src = s.Type
		}
		lang := s.DefaultLang
		if lang == "" {
			lang = *defaultLang
		}
		if lang == "" {
			lang = config.DefaultLang
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			cfg := s
			cfg.Name = src
			cfg.DefaultLang = lang
			if err := syncer.SyncSource(ctx, cfg, *defaultLang); err != nil {
				logger.Error("config source sync failed", "source", src, "err", err)
			} else {
				logger.Info("config source sync complete", "source", src)
			}
		}()
	}

	srv := server.New(logger, searcher, syncer, database, embedFn, indexHTML)
	srv.Config = cfg
	srv.GHClient = ghClient
	srv.GHRepoOwner = owner
	srv.GHRepoName = repoName
	srv.GHWebhookSecret = *ghWebhookSecret
	srv.DefaultLang = *defaultLang
	srv.WebhookSource = *source

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	setupVPSServer(ctx, srv, cfg, syncer)

	if err := srv.Run(ctx, *addr); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}

func doStatus(logger *slog.Logger, cfg *config.Config) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dbPath := fs.String("db", cfg.Server.DB, "Path to SQLite database")
	if err := fs.Parse(os.Args[2:]); err != nil {
		logger.Error("parse flags", "err", err)
		os.Exit(1)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	stats, err := db.NewStatsRepo(database).Get(context.Background())
	if err != nil {
		logger.Error("get stats", "err", err)
		os.Exit(1)
	}

	fmt.Printf("Database: %s\n", *dbPath)
	fmt.Printf("Total docs: %d\n", stats.TotalDocs)
	fmt.Printf("DB size: %d bytes\n", stats.DBSizeBytes)
	if stats.LastSyncAt != nil {
		fmt.Printf("Last sync: %s\n", stats.LastSyncAt.Format(time.RFC3339))
	} else {
		fmt.Println("Last sync: never")
	}
	fmt.Println("By source:")
	for _, s := range stats.Sources {
		fmt.Printf("  %s: %d\n", s, stats.BySource[s])
	}
	fmt.Println("By doc type:")
	for dt, c := range stats.ByDocType {
		fmt.Printf("  %s: %d\n", dt, c)
	}
	if len(cfg.Sources) > 0 {
		fmt.Println("Configured sources:")
		for _, s := range cfg.Sources {
			name := s.Name
			if name == "" {
				name = s.Type
			}
			fmt.Printf("  %s (%s)\n", name, s.Type)
		}
	}
}

func doReindex(logger *slog.Logger, cfg *config.Config) {
	fs := flag.NewFlagSet("reindex", flag.ExitOnError)
	dbPath := fs.String("db", cfg.Server.DB, "Path to SQLite database")
	if err := fs.Parse(os.Args[2:]); err != nil {
		logger.Error("parse flags", "err", err)
		os.Exit(1)
	}

	database, embedFn := openDBAndEmbed(logger, *dbPath, cfg)
	defer database.Close()

	syncer := &service.Syncer{DB: database, EmbedFn: embedFn}
	ctx := context.Background()

	sources := cfg.Sources
	if len(sources) == 0 {
		sources = []config.SourceConfig{{
			Name:        cfg.Sync.Source,
			Type:        "local",
			Dir:         cfg.Sync.Dir,
			DefaultLang: cfg.Sync.DefaultLang,
		}}
	}

	for _, s := range sources {
		if err := syncer.SyncSource(ctx, s, cfg.Search.DefaultLang); err != nil {
			logger.Error("reindex source failed", "source", s.Name, "err", err)
		}
	}
	logger.Info("reindex complete")
}

func doMaintain(logger *slog.Logger, cfg *config.Config) {
	fs := flag.NewFlagSet("maintain", flag.ExitOnError)
	dbPath := fs.String("db", cfg.Server.DB, "Path to SQLite database")
	backup := fs.Bool("backup", false, "Create a timestamped backup before maintenance")
	if err := fs.Parse(os.Args[2:]); err != nil {
		logger.Error("parse flags", "err", err)
		os.Exit(1)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if *backup {
		backupPath := *dbPath + "." + time.Now().Format("20060102_150405") + ".bak"
		if err := database.Backup(backupPath); err != nil {
			logger.Error("backup failed", "err", err)
			os.Exit(1)
		}
		logger.Info("backup created", "path", backupPath)
	}

	if err := database.Maintain(context.Background()); err != nil {
		logger.Error("maintenance failed", "err", err)
		os.Exit(1)
	}
	logger.Info("maintenance complete")
}

func newLogger(format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	switch format {
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	default:
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
}

func openDBAndEmbed(logger *slog.Logger, dbPath string, cfg *config.Config) (*db.DB, embed.Func) {
	database, err := db.Open(dbPath)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	embedFn, err := embed.NewProvider(cfg.Embedding.Provider, cfg.Embedding.Model, cfg.Embedding.BaseURL, cfg.Embedding.APIKey)
	if err != nil {
		logger.Error("create embed provider", "err", err)
		os.Exit(1)
	}
	if strings.EqualFold(cfg.Embedding.Provider, "mock") {
		logger.Warn("using mock embedding provider — vector search results are pseudo-random; configure embedding.provider for real semantic search")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := embed.Validate(ctx, embedFn); err != nil {
		logger.Error("embedding validation failed", "provider", cfg.Embedding.Provider, "err", err)
		os.Exit(1)
	}
	return database, embedFn
}

func parseRepoURL(repoURL string) (owner, repo string, err error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 3 {
		return parts[1], parts[2], nil
	}
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "", "", fmt.Errorf("invalid repo URL: %s", repoURL)
}

//go:embed index.html
var indexHTML []byte
