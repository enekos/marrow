package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

)

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Detect GitHub App webhook by presence of X-GitHub-Event header
	if r.Header.Get("X-GitHub-Event") != "" {
		s.handleGitHubWebhook(w, r)
		return
	}

	// Legacy marrow webhook
	secret := r.Header.Get("X-Marrow-Secret")
	if secret == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	state, err := s.StateRepo.Get(r.Context(), s.WebhookSource)
	if err != nil || state.SecretKey != secret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Trigger background sync
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.Syncer.SyncGit(ctx, state.Source, s.DefaultLang, state.RepoURL, state.Token, state.LocalPath); err != nil {
			s.Logger.Error("webhook sync failed", "source", state.Source, "err", err)
		} else {
			s.Logger.Info("webhook sync complete", "source", state.Source)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))
}

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if s.GHWebhookSecret != "" {
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
		if !verifyGitHubSignature(payload, s.GHWebhookSecret, sig) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Restore body for json decoding
		r.Body = io.NopCloser(bytes.NewReader(payload))
	}

	eventType := r.Header.Get("X-GitHub-Event")
	delivery := r.Header.Get("X-GitHub-Delivery")

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	s.Logger.Info("github webhook received", "event", eventType, "delivery", delivery)

	if s.GHClient == nil {
		s.Logger.Warn("github webhook ignored: app not configured")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"ignored"}`))
		return
	}

	owner, repo := s.GHRepoOwner, s.GHRepoName
	if owner == "" || repo == "" {
		s.Logger.Warn("github webhook ignored: owner/repo not configured")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"ignored"}`))
		return
	}

	switch eventType {
	case "issues":
		s.handleIssuesWebhook(payload, owner, repo)
	case "pull_request":
		s.handlePullRequestWebhook(payload, owner, repo)
	case "issue_comment":
		s.handleIssueCommentWebhook(payload, owner, repo)
	case "pull_request_review_comment":
		s.handlePRCommentWebhook(payload, owner, repo)
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))
}

func (s *Server) handleIssuesWebhook(payload map[string]any, owner, repo string) {
	action, _ := payload["action"].(string)
	issue, _ := payload["issue"].(map[string]any)
	number := extractNumber(issue)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		switch action {
		case "opened", "edited", "reopened":
			if err := s.Syncer.IndexGitHubIssue(ctx, "github-api", s.DefaultLang, s.GHClient, owner, repo, number); err != nil {
				s.Logger.Error("index issue webhook failed", "number", number, "err", err)
			} else {
				s.Logger.Info("indexed issue", "number", number)
			}
		case "closed":
			if err := s.Syncer.DeleteGitHubDocument(ctx, "github-api", s.DefaultLang, owner, repo, "issues", number); err != nil {
				s.Logger.Error("delete issue webhook failed", "number", number, "err", err)
			} else {
				s.Logger.Info("deleted issue", "number", number)
			}
		}
	}()
}

func (s *Server) handlePullRequestWebhook(payload map[string]any, owner, repo string) {
	action, _ := payload["action"].(string)
	pr, _ := payload["pull_request"].(map[string]any)
	number := extractNumber(pr)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		switch action {
		case "opened", "edited", "reopened", "synchronize":
			if err := s.Syncer.IndexGitHubPullRequest(ctx, "github-api", s.DefaultLang, s.GHClient, owner, repo, number); err != nil {
				s.Logger.Error("index pr webhook failed", "number", number, "err", err)
			} else {
				s.Logger.Info("indexed pull request", "number", number)
			}
		case "closed":
			if err := s.Syncer.DeleteGitHubDocument(ctx, "github-api", s.DefaultLang, owner, repo, "pull", number); err != nil {
				s.Logger.Error("delete pr webhook failed", "number", number, "err", err)
			} else {
				s.Logger.Info("deleted pull request", "number", number)
			}
		}
	}()
}

func (s *Server) handleIssueCommentWebhook(payload map[string]any, owner, repo string) {
	issue, _ := payload["issue"].(map[string]any)
	number := extractNumber(issue)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.Syncer.IndexGitHubIssue(ctx, "github-api", s.DefaultLang, s.GHClient, owner, repo, number); err != nil {
			s.Logger.Error("index issue comment webhook failed", "number", number, "err", err)
		} else {
			s.Logger.Info("indexed issue after comment", "number", number)
		}
	}()
}

func (s *Server) handlePRCommentWebhook(payload map[string]any, owner, repo string) {
	pr, _ := payload["pull_request"].(map[string]any)
	number := extractNumber(pr)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.Syncer.IndexGitHubPullRequest(ctx, "github-api", s.DefaultLang, s.GHClient, owner, repo, number); err != nil {
			s.Logger.Error("index pr comment webhook failed", "number", number, "err", err)
		} else {
			s.Logger.Info("indexed pull request after comment", "number", number)
		}
	}()
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

func extractNumber(obj map[string]any) int {
	if obj == nil {
		return 0
	}
	n, _ := obj["number"].(float64)
	return int(n)
}


