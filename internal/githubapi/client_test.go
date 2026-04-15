package githubapi

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-github/v69/github"
)

// generateTestPrivateKey creates a minimal RSA private key PEM for testing.
func generateTestPrivateKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(block)
}

func TestParseRepo(t *testing.T) {
	cases := []struct {
		url    string
		owner  string
		repo   string
		hasErr bool
	}{
		{"https://github.com/owner/repo.git", "owner", "repo", false},
		{"https://github.com/owner/repo", "owner", "repo", false},
		{"http://github.com/owner/repo", "owner", "repo", false},
		{"owner/repo", "owner", "repo", false},
		{"invalid", "", "", true},
	}
	for _, c := range cases {
		owner, repo, err := parseRepo(c.url)
		if c.hasErr {
			if err == nil {
				t.Errorf("expected error for %q", c.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for %q: %v", c.url, err)
			continue
		}
		if owner != c.owner || repo != c.repo {
			t.Errorf("parseRepo(%q) = %q, %q; want %q, %q", c.url, owner, repo, c.owner, c.repo)
		}
	}
}

func TestContentHash(t *testing.T) {
	h1 := ContentHash("title", "body", []string{"c1", "c2"})
	h2 := ContentHash("title", "body", []string{"c1", "c2"})
	h3 := ContentHash("title", "body", []string{"c1"})
	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d", len(h1))
	}
}

func TestClientEnsureInstallation(t *testing.T) {
	// We can't fully test JWT signing with an invalid key, but we can verify
	// that NewClient rejects bad inputs.
	_, err := NewClient(0, []byte("key"), 0)
	if err == nil || !strings.Contains(err.Error(), "appID") {
		t.Errorf("expected appID error, got: %v", err)
	}
	_, err = NewClient(123, []byte{}, 0)
	if err == nil || !strings.Contains(err.Error(), "privateKey") {
		t.Errorf("expected privateKey error, got: %v", err)
	}
}

func TestFetchIssueAndPR(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/42", func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"number":42,"title":"Test Issue","body":"body here","updated_at":"2024-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/issues/42"}`)
	})
	mux.HandleFunc("/repos/owner/repo/issues/42/comments", func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"body":"comment 1"}]`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls/7", func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"number":7,"title":"Test PR","body":"pr body","updated_at":"2024-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/pull/7"}`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls/7/comments", func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	key := generateTestPrivateKey(t)
	client, err := NewClient(1, key, 1)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	// Override the underlying go-github client to talk to our test server.
	gh := github.NewClient(nil)
	gh.BaseURL = mustParseURL(server.URL + "/")
	gh.UploadURL = mustParseURL(server.URL + "/")
	client.setClient(gh)

	ctx := context.Background()
	issue, err := client.FetchIssue(ctx, "owner", "repo", 42)
	if err != nil {
		t.Fatalf("fetch issue: %v", err)
	}
	if issue.Number != 42 || issue.Title != "Test Issue" {
		t.Errorf("unexpected issue: %+v", issue)
	}
	if len(issue.Comments) != 1 || issue.Comments[0] != "comment 1" {
		t.Errorf("unexpected comments: %v", issue.Comments)
	}

	pr, err := client.FetchPullRequest(ctx, "owner", "repo", 7)
	if err != nil {
		t.Fatalf("fetch pr: %v", err)
	}
	if pr.Number != 7 || pr.Title != "Test PR" {
		t.Errorf("unexpected pr: %+v", pr)
	}
}

func TestListOpenIssuesAndPRs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{"number":1,"title":"Issue 1","body":"b1","updated_at":"2024-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/issues/1","pull_request":{}},
			{"number":2,"title":"Issue 2","body":"b2","updated_at":"2024-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/issues/2"}
		]`)
	})
	mux.HandleFunc("/repos/owner/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/repos/owner/repo/issues/2/comments", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"body":"c2"}]`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"number":3,"title":"PR 1","body":"pb1","updated_at":"2024-01-01T00:00:00Z","html_url":"https://github.com/owner/repo/pull/3"}]`)
	})
	mux.HandleFunc("/repos/owner/repo/pulls/3/comments", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	key := generateTestPrivateKey(t)
	client, err := NewClient(1, key, 1)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	gh := github.NewClient(nil)
	gh.BaseURL = mustParseURL(server.URL + "/")
	gh.UploadURL = mustParseURL(server.URL + "/")
	client.setClient(gh)

	ctx := context.Background()
	issues, err := client.ListOpenIssues(ctx, "owner", "repo")
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	// Issue 1 is skipped because it has a pull_request field.
	if len(issues) != 1 || issues[0].Number != 2 {
		t.Errorf("unexpected issues: %+v", issues)
	}

	prs, err := client.ListOpenPullRequests(ctx, "owner", "repo")
	if err != nil {
		t.Fatalf("list prs: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 3 {
		t.Errorf("unexpected prs: %+v", prs)
	}
}

func mustParseURL(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}

// jwtTokenFromString is unused but kept to satisfy any linter if needed.
func jwtTokenFromString(s string) (*jwt.Token, error) {
	return jwt.Parse(s, func(token *jwt.Token) (interface{}, error) {
		return nil, fmt.Errorf("not implemented")
	})
}

//nolint:all
func parsePrivateKey(pemBytes []byte) (interface{}, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no pem block")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func TestIssueDocumentFields(t *testing.T) {
	doc := IssueDocument{
		Number:    1,
		Title:     "t",
		Body:      "b",
		URL:       "url",
		UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Comments:  []string{"c"},
	}
	if doc.Number != 1 {
		t.Fail()
	}
}
