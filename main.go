package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"marrow/internal/config"
	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/githubapi"
	"marrow/internal/search"
	"marrow/internal/sync"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
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

	if *all {
		for _, s := range cfg.Sources {
			src := s.Name
			if src == "" {
				src = s.Type
			}
			lang := s.DefaultLang
			if lang == "" {
				lang = cfg.Search.DefaultLang
			}
			if lang == "" {
				lang = "en"
			}
			orch := &sync.Orchestrator{
				DB:          database,
				EmbedFn:     embedFn,
				Source:      src,
				DefaultLang: lang,
			}
			ctx := context.Background()
			switch s.Type {
			case "local":
				if err := orch.RunLocal(ctx, s.Dir); err != nil {
					logger.Error("sync source failed", "source", src, "err", err)
				}
			case "git":
				lp := s.LocalPath
				if lp == "" {
					lp = sync.LocalPathFromSource("./repo", src)
				}
				if err := orch.RunGit(ctx, s.RepoURL, s.Token, lp); err != nil {
					logger.Error("sync source failed", "source", src, "err", err)
				}
			default:
				logger.Warn("skipped unsupported source type", "source", src, "type", s.Type)
			}
		}
		logger.Info("sync all complete")
		return
	}

	orch := &sync.Orchestrator{
		DB:          database,
		EmbedFn:     embedFn,
		Source:      *source,
		DefaultLang: *defaultLang,
	}
	ctx := context.Background()
	if err := orch.RunLocal(ctx, *dir); err != nil {
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

	// Persist webhook secret / repo config
	if *webhookSecret != "" || *repoURL != "" {
		state := &db.SyncState{
			Source:    *source,
			SecretKey: *webhookSecret,
			RepoURL:   *repoURL,
			LocalPath: *localPath,
			Token:     *repoToken,
		}
		if err := database.UpsertSyncState(context.Background(), state); err != nil {
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
			orch := &sync.Orchestrator{
				DB:          database,
				EmbedFn:     embedFn,
				Source:      *source,
				DefaultLang: *defaultLang,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := orch.RunGit(ctx, *repoURL, *repoToken, *localPath); err != nil {
				logger.Error("initial git sync failed", "err", err)
			} else {
				logger.Info("initial git sync complete")
			}
		}()
	}

	// Optional initial GitHub API sync in background
	if ghClient != nil && owner != "" && repoName != "" {
		go func() {
			orch := &sync.Orchestrator{
				DB:          database,
				EmbedFn:     embedFn,
				Source:      "github-api",
				DefaultLang: *defaultLang,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if err := orch.RunGitHub(ctx, ghClient, owner, repoName); err != nil {
				logger.Error("initial github api sync failed", "err", err)
			} else {
				logger.Info("initial github api sync complete")
			}
		}()
	}

	// Background sync for all config sources
	for _, s := range cfg.Sources {
		src := s.Name
		if src == "" {
			src = s.Type
		}
		lang := s.DefaultLang
		if lang == "" {
			lang = *defaultLang
		}
		if lang == "" {
			lang = "en"
		}
		if s.Type == "local" && s.Dir != "" {
			go func(s config.SourceConfig, src, lang string) {
				orch := &sync.Orchestrator{
					DB:          database,
					EmbedFn:     embedFn,
					Source:      src,
					DefaultLang: lang,
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if err := orch.RunLocal(ctx, s.Dir); err != nil {
					logger.Error("config source sync failed", "source", src, "err", err)
				} else {
					logger.Info("config source sync complete", "source", src)
				}
			}(s, src, lang)
		}
		if s.Type == "git" && s.RepoURL != "" {
			go func(s config.SourceConfig, src, lang string) {
				lp := s.LocalPath
				if lp == "" {
					lp = sync.LocalPathFromSource("./repo", src)
				}
				orch := &sync.Orchestrator{
					DB:          database,
					EmbedFn:     embedFn,
					Source:      src,
					DefaultLang: lang,
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if err := orch.RunGit(ctx, s.RepoURL, s.Token, lp); err != nil {
					logger.Error("config source sync failed", "source", src, "err", err)
				} else {
					logger.Info("config source sync complete", "source", src)
				}
			}(s, src, lang)
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		stats, err := database.GetStats(r.Context())
		if err != nil {
			logger.Error("stats error", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Query   string `json:"q"`
			Limit   int    `json:"limit"`
			Lang    string `json:"lang"`
			Source  string `json:"source"`
			DocType string `json:"doc_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		if req.Limit <= 0 {
			req.Limit = 10
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := search.Filter{
			Source:  req.Source,
			DocType: req.DocType,
			Lang:    req.Lang,
		}
		results, err := engine.Search(ctx, req.Query, req.Lang, req.Limit, filter)
		if err != nil {
			logger.Error("search error", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": results,
		})
	})

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Detect GitHub App webhook by presence of X-GitHub-Event header
		if r.Header.Get("X-GitHub-Event") != "" {
			handleGitHubWebhook(w, r, logger, database, ghClient, owner, repoName, *ghWebhookSecret, *defaultLang, embedFn)
			return
		}

		// Legacy marrow webhook
		secret := r.Header.Get("X-Marrow-Secret")
		if secret == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		state, err := database.GetSyncState(r.Context(), *source)
		if err != nil || state.SecretKey != secret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Trigger background sync
		go func() {
			orch := &sync.Orchestrator{
				DB:          database,
				EmbedFn:     embedFn,
				Source:      *source,
				DefaultLang: *defaultLang,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := orch.RunGit(ctx, state.RepoURL, state.Token, state.LocalPath); err != nil {
				logger.Error("webhook sync failed", "err", err)
			} else {
				logger.Info("webhook sync complete")
			}
		}()

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"accepted"}`))
	})

	logger.Info("server listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
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

	stats, err := database.GetStats(context.Background())
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

	ctx := context.Background()
	sources := cfg.Sources
	if len(sources) == 0 {
		// Fallback to legacy single source
		sources = []config.SourceConfig{{
			Name:   cfg.Sync.Source,
			Type:   "local",
			Dir:    cfg.Sync.Dir,
			DefaultLang: cfg.Sync.DefaultLang,
		}}
	}

	for _, s := range sources {
		src := s.Name
		if src == "" {
			src = s.Type
		}
		lang := s.DefaultLang
		if lang == "" {
			lang = cfg.Search.DefaultLang
		}
		if lang == "" {
			lang = "en"
		}
		orch := &sync.Orchestrator{
			DB:          database,
			EmbedFn:     embedFn,
			Source:      src,
			DefaultLang: lang,
		}
		switch s.Type {
		case "local":
			if err := orch.RunLocal(ctx, s.Dir); err != nil {
				logger.Error("reindex local failed", "source", src, "err", err)
			}
		case "git":
			lp := s.LocalPath
			if lp == "" {
				lp = sync.LocalPathFromSource("./repo", src)
			}
			if err := orch.RunGit(ctx, s.RepoURL, s.Token, lp); err != nil {
				logger.Error("reindex git failed", "source", src, "err", err)
			}
		default:
			logger.Warn("skipped unsupported source type", "source", src, "type", s.Type)
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
	return database, embedFn
}

func handleGitHubWebhook(w http.ResponseWriter, r *http.Request, logger *slog.Logger, database *db.DB, ghClient *githubapi.Client, owner, repoName, secret, defaultLang string, embedFn embed.Func) {
	if secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if sig == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		if !verifyGitHubSignature(payload, secret, sig) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Restore body for json decoding
		r.Body = io.NopCloser(strings.NewReader(string(payload)))
	}

	eventType := r.Header.Get("X-GitHub-Event")
	delivery := r.Header.Get("X-GitHub-Delivery")

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	logger.Info("github webhook received", "event", eventType, "delivery", delivery)

	if ghClient == nil || owner == "" || repoName == "" {
		logger.Warn("github webhook ignored: app not configured")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"ignored"}`))
		return
	}

	orch := &sync.Orchestrator{
		DB:          database,
		EmbedFn:     embedFn,
		Source:      "github-api",
		DefaultLang: defaultLang,
	}

	switch eventType {
	case "issues":
		action, _ := payload["action"].(string)
		issue, _ := payload["issue"].(map[string]any)
		number := extractNumber(issue)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			switch action {
			case "opened", "edited", "reopened":
				if err := orch.IndexSingleIssue(ctx, ghClient, owner, repoName, number); err != nil {
					logger.Error("index issue webhook failed", "number", number, "err", err)
				} else {
					logger.Info("indexed issue", "number", number)
				}
			case "closed":
				if err := orch.DeleteGitHubDocument(ctx, owner, repoName, "issues", number); err != nil {
					logger.Error("delete issue webhook failed", "number", number, "err", err)
				} else {
					logger.Info("deleted issue", "number", number)
				}
			}
		}()

	case "pull_request":
		action, _ := payload["action"].(string)
		pr, _ := payload["pull_request"].(map[string]any)
		number := extractNumber(pr)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			switch action {
			case "opened", "edited", "reopened", "synchronize":
				if err := orch.IndexSinglePullRequest(ctx, ghClient, owner, repoName, number); err != nil {
					logger.Error("index pr webhook failed", "number", number, "err", err)
				} else {
					logger.Info("indexed pull request", "number", number)
				}
			case "closed":
				if err := orch.DeleteGitHubDocument(ctx, owner, repoName, "pull", number); err != nil {
					logger.Error("delete pr webhook failed", "number", number, "err", err)
				} else {
					logger.Info("deleted pull request", "number", number)
				}
			}
		}()

	case "issue_comment":
		issue, _ := payload["issue"].(map[string]any)
		number := extractNumber(issue)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := orch.IndexSingleIssue(ctx, ghClient, owner, repoName, number); err != nil {
				logger.Error("index issue comment webhook failed", "number", number, "err", err)
			} else {
				logger.Info("indexed issue after comment", "number", number)
			}
		}()

	case "pull_request_review_comment":
		pr, _ := payload["pull_request"].(map[string]any)
		number := extractNumber(pr)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := orch.IndexSinglePullRequest(ctx, ghClient, owner, repoName, number); err != nil {
				logger.Error("index pr comment webhook failed", "number", number, "err", err)
			} else {
				logger.Info("indexed pull request after comment", "number", number)
			}
		}()
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))
}

func verifyGitHubSignature(payload []byte, secret, signature string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := prefix + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
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

func extractNumber(obj map[string]any) int {
	if obj == nil {
		return 0
	}
	n, _ := obj["number"].(float64)
	return int(n)
}

//go:embed index.html
var indexHTML []byte
