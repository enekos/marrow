//go:build vps

package vps

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/server"
)

// middlewareChain applies a series of http.Handler middlewares.
func middlewareChain(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}

// siteResolution extracts the site from Host or X-Site header.
func siteResolution(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var site *config.SiteConfig
			if cfg.HasSites() {
				host := strings.ToLower(r.Host)
				site = cfg.SiteByHost(host)
				if site == nil {
					site = cfg.SiteByName(r.Header.Get("X-Site"))
				}
				if site == nil {
					http.Error(w, `{"error":"site not found"}`, http.StatusNotFound)
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(server.WithSite(r.Context(), site)))
		})
	}
}

// corsMiddleware handles per-site CORS.
func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			site := server.SiteFromContext(r.Context())
			origin := r.Header.Get("Origin")

			if site != nil && len(site.CORSOrigins) > 0 {
				allowed := false
				for _, o := range site.CORSOrigins {
					if o == origin || o == "*" {
						allowed = true
						break
					}
				}
				if !allowed {
					http.Error(w, `{"error":"origin not allowed"}`, http.StatusForbidden)
					return
				}
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Site")
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// authMiddleware enforces per-site API keys.
func authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/search" {
				site := server.SiteFromContext(r.Context())
				if site != nil && site.APIKey != "" {
					auth := r.Header.Get("Authorization")
					const prefix = "Bearer "
					if !strings.HasPrefix(auth, prefix) || auth[len(prefix):] != site.APIKey {
						http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimiters holds per-site token buckets.
type rateLimiters struct {
	mu     sync.RWMutex
	limits map[string]*rate.Limiter
}

func newRateLimiters() *rateLimiters {
	return &rateLimiters{limits: make(map[string]*rate.Limiter)}
}

func (rl *rateLimiters) getLimiter(siteName string, rps float64) *rate.Limiter {
	rl.mu.RLock()
	lim, ok := rl.limits[siteName]
	rl.mu.RUnlock()
	if ok {
		return lim
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	lim, ok = rl.limits[siteName]
	if ok {
		return lim
	}
	burst := 5
	if rps < 1 {
		burst = 1
	}
	lim = rate.NewLimiter(rate.Limit(rps), burst)
	rl.limits[siteName] = lim
	return lim
}

// rateLimitMiddleware applies per-site rate limiting.
func rateLimitMiddleware() func(http.Handler) http.Handler {
	rl := newRateLimiters()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			site := server.SiteFromContext(r.Context())
			if site != nil && site.RateLimitRPS > 0 {
				limiter := rl.getLimiter(site.Name, site.RateLimitRPS)
				if !limiter.Allow() {
					w.Header().Set("Retry-After", strconv.Itoa(int(1/site.RateLimitRPS)+1))
					http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestLogMiddleware logs every request with timing.
func requestLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wr := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wr, r)
			site := server.SiteFromContext(r.Context())
			siteName := ""
			if site != nil {
				siteName = site.Name
			}
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"site", siteName,
				"status", wr.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
