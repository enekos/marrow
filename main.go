package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/githubapi"
	"marrow/internal/search"
	"marrow/internal/sync"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: marrow <sync|serve> [options]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "sync":
		doSync(logger)
	case "serve":
		doServe(logger)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func doSync(logger *slog.Logger) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory to crawl for markdown files")
	dbPath := fs.String("db", "marrow.db", "Path to SQLite database")
	source := fs.String("source", "local", "Source identifier for this directory")
	defaultLang := fs.String("default-lang", "en", "Default language for documents without frontmatter lang")
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

	orch := &sync.Orchestrator{
		DB:          database,
		EmbedFn:     embed.NewMock(),
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

func doServe(logger *slog.Logger) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	dbPath := fs.String("db", "marrow.db", "Path to SQLite database")
	repoURL := fs.String("repo-url", "", "GitHub repo URL to clone/pull")
	repoToken := fs.String("repo-token", "", "GitHub personal access token")
	webhookSecret := fs.String("webhook-secret", "", "Secret key for /webhook endpoint")
	source := fs.String("source", "github", "Source identifier for repo sync state")
	localPath := fs.String("local-path", "./repo", "Local directory to clone repo into")
	detectLang := fs.Bool("detect-lang", true, "Enable automatic language detection for search queries")
	defaultLang := fs.String("default-lang", "en", "Default language when detection is disabled or no lang hint is provided")

	// GitHub App flags
	ghAppID := fs.Int64("github-app-id", 0, "GitHub App ID")
	ghAppKeyPath := fs.String("github-app-private-key", "", "Path to GitHub App private key PEM file")
	ghInstallationID := fs.Int64("github-installation-id", 0, "GitHub App installation ID (auto-discover if 0)")
	ghRepoOwner := fs.String("github-repo-owner", "", "GitHub repo owner (defaults to parsing -repo-url)")
	ghRepoName := fs.String("github-repo-name", "", "GitHub repo name (defaults to parsing -repo-url)")
	ghWebhookSecret := fs.String("github-webhook-secret", "", "GitHub App webhook secret for signature verification")

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

	engine := search.NewEngine(database, embed.NewMock())
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
				EmbedFn:     embed.NewMock(),
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
				EmbedFn:     embed.NewMock(),
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

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Query string `json:"q"`
			Limit int    `json:"limit"`
			Lang  string `json:"lang"`
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

		results, err := engine.Search(ctx, req.Query, req.Lang, req.Limit)
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
			handleGitHubWebhook(w, r, logger, database, ghClient, owner, repoName, *ghWebhookSecret, *defaultLang)
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
				EmbedFn:     embed.NewMock(),
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

func handleGitHubWebhook(w http.ResponseWriter, r *http.Request, logger *slog.Logger, database *db.DB, ghClient *githubapi.Client, owner, repoName, secret, defaultLang string) {
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
		EmbedFn:     embed.NewMock(),
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
