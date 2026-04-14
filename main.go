package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"marrow/internal/db"
	"marrow/internal/embed"
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
		DB:      database,
		EmbedFn: embed.NewMock(),
		Source:  *source,
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

	// Optional initial git sync in background
	if *repoURL != "" {
		go func() {
			orch := &sync.Orchestrator{
				DB:      database,
				EmbedFn: embed.NewMock(),
				Source:  *source,
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
				DB:      database,
				EmbedFn: embed.NewMock(),
				Source:  *source,
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
