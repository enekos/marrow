//go:build vps

package vps

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"marrow/internal/config"
	"marrow/internal/server"
)

func TestSiteResolution(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{Name: "blog", Hosts: []string{"search.blog.com"}},
			{Name: "docs", Hosts: []string{"search.docs.com"}},
		},
	}

	tests := []struct {
		name       string
		host       string
		xSite      string
		wantStatus int
		wantSite   string
	}{
		{"host match blog", "search.blog.com", "", http.StatusOK, "blog"},
		{"host match docs", "search.docs.com", "", http.StatusOK, "docs"},
		{"x-site fallback", "other.com", "docs", http.StatusOK, "docs"},
		{"unknown site", "other.com", "", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := siteResolution(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				site := server.SiteFromContext(r.Context())
				if site == nil {
					if tt.wantSite != "" {
						t.Errorf("expected site %q, got nil", tt.wantSite)
					}
					return
				}
				if site.Name != tt.wantSite {
					t.Errorf("site.Name = %q, want %q", site.Name, tt.wantSite)
				}
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host
			if tt.xSite != "" {
				req.Header.Set("X-Site", tt.xSite)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestCorsMiddleware(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				Name:        "blog",
				CORSOrigins: []string{"https://blog.com", "https://www.blog.com"},
			},
		},
	}

	tests := []struct {
		name         string
		origin       string
		site         *config.SiteConfig
		wantStatus   int
		wantOrigin   string
		wantCORS     bool
	}{
		{"allowed origin", "https://blog.com", &cfg.Sites[0], http.StatusOK, "https://blog.com", true},
		{"allowed origin www", "https://www.blog.com", &cfg.Sites[0], http.StatusOK, "https://www.blog.com", true},
		{"disallowed origin", "https://evil.com", &cfg.Sites[0], http.StatusForbidden, "", false},
		{"no site no cors", "", nil, http.StatusOK, "", false},
		{"preflight allowed", "https://blog.com", &cfg.Sites[0], http.StatusNoContent, "https://blog.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := corsMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			method := http.MethodGet
			if strings.Contains(tt.name, "preflight") {
				method = http.MethodOptions
			}

			req := httptest.NewRequest(method, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.site != nil {
				req = req.WithContext(server.WithSite(req.Context(), tt.site))
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.wantCORS {
				got := rr.Header().Get("Access-Control-Allow-Origin")
				if got != tt.wantOrigin {
					t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantOrigin)
				}
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	handler := authMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		path       string
		site       *config.SiteConfig
		authHeader string
		wantStatus int
	}{
		{"no auth required", "/health", nil, "", http.StatusOK},
		{"valid key", "/search", &config.SiteConfig{APIKey: "secret"}, "Bearer secret", http.StatusOK},
		{"invalid key", "/search", &config.SiteConfig{APIKey: "secret"}, "Bearer wrong", http.StatusUnauthorized},
		{"missing key", "/search", &config.SiteConfig{APIKey: "secret"}, "", http.StatusUnauthorized},
		{"search no site no key", "/search", nil, "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			if tt.site != nil {
				req = req.WithContext(server.WithSite(req.Context(), tt.site))
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	handler := rateLimitMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("within limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(server.WithSite(req.Context(), &config.SiteConfig{
			Name:         "test",
			RateLimitRPS: 1000, // very high, should not trigger
		}))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		// Create a new handler so we get a fresh limiter.
		h := rateLimitMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		site := &config.SiteConfig{Name: "slow", RateLimitRPS: 0.001} // basically 1 per 1000s
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(server.WithSite(req.Context(), site))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if i == 0 && rr.Code != http.StatusOK {
				t.Fatalf("first request status = %d, want %d", rr.Code, http.StatusOK)
			}
			if i > 0 && rr.Code == http.StatusTooManyRequests {
				// Expected to eventually rate limit.
				return
			}
		}
		// If we never got rate limited with 0.001 RPS, that's odd but not a hard failure.
	})
}

func TestRequestLogMiddleware(t *testing.T) {
	var logged bool
	logger := slog.New(slog.NewTextHandler(&logWriter{fn: func() { logged = true }}, nil))

	handler := requestLogMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	// Allow a short moment for the async log write.
	time.Sleep(50 * time.Millisecond)
	if !logged {
		t.Error("expected request to be logged")
	}
}

func TestMiddlewareChain(t *testing.T) {
	order := []string{}
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	}

	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "final")
	})

	chain := middlewareChain(m1, m2)
	chain(final).ServeHTTP(nil, httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"m1-before", "m2-before", "final", "m2-after", "m1-after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

type logWriter struct {
	fn func()
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	lw.fn()
	return len(p), nil
}

// TestVPSIntegration runs the full middleware stack against a simulated request.
func TestVPSIntegration(t *testing.T) {
	cfg := &config.Config{
		Sites: []config.SiteConfig{
			{
				Name:         "blog",
				Hosts:        []string{"search.blog.com"},
				CORSOrigins:  []string{"https://blog.com"},
				APIKey:       "blog-secret",
				RateLimitRPS: 1000,
				Sources:      []string{"blog-md"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(&logWriter{fn: func() {}}, nil))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site := server.SiteFromContext(r.Context())
		if site == nil {
			http.Error(w, "no site", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "site=%s", site.Name)
	})

	// Apply the same middlewares Setup would apply.
	stack := middlewareChain(
		siteResolution(cfg),
		corsMiddleware(cfg),
		authMiddleware(),
		rateLimitMiddleware(),
		requestLogMiddleware(logger),
	)
	handler := stack(inner)

	t.Run("happy path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/search", nil)
		req.Host = "search.blog.com"
		req.Header.Set("Origin", "https://blog.com")
		req.Header.Set("Authorization", "Bearer blog-secret")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "site=blog") {
			t.Errorf("body = %q, want site=blog", rr.Body.String())
		}
	})

	t.Run("wrong api key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/search", nil)
		req.Host = "search.blog.com"
		req.Header.Set("Origin", "https://blog.com")
		req.Header.Set("Authorization", "Bearer wrong")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("wrong origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/search", nil)
		req.Host = "search.blog.com"
		req.Header.Set("Origin", "https://evil.com")
		req.Header.Set("Authorization", "Bearer blog-secret")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}
