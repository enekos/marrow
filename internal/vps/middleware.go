//go:build vps

package vps

import (
	"log/slog"
	"net"
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

// securityHeadersMiddleware adds baseline security headers to every response.
func securityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			next.ServeHTTP(w, r)
		})
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

// corsMiddleware handles per-site or global CORS.
func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			site := server.SiteFromContext(r.Context())
			origin := r.Header.Get("Origin")

			var origins []string
			if site != nil && len(site.CORSOrigins) > 0 {
				origins = site.CORSOrigins
			} else if len(cfg.Server.CORSOrigins) > 0 {
				origins = cfg.Server.CORSOrigins
			}

			if len(origins) > 0 {
				allowed := false
				for _, o := range origins {
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

// authMiddleware enforces per-site or global API keys on search.
func authMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/search" {
				site := server.SiteFromContext(r.Context())
				apiKey := ""
				if site != nil && site.APIKey != "" {
					apiKey = site.APIKey
				} else if cfg.Server.APIKey != "" {
					apiKey = cfg.Server.APIKey
				}
				if apiKey != "" {
					auth := r.Header.Get("Authorization")
					const prefix = "Bearer "
					if !strings.HasPrefix(auth, prefix) || auth[len(prefix):] != apiKey {
						http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimiters holds named token buckets.
type rateLimiters struct {
	mu     sync.RWMutex
	limits map[string]*rate.Limiter
}

func newRateLimiters() *rateLimiters {
	return &rateLimiters{limits: make(map[string]*rate.Limiter)}
}

func (rl *rateLimiters) getLimiter(key string, rps float64, burst int) *rate.Limiter {
	rl.mu.RLock()
	lim, ok := rl.limits[key]
	rl.mu.RUnlock()
	if ok {
		return lim
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	lim, ok = rl.limits[key]
	if ok {
		return lim
	}
	lim = rate.NewLimiter(rate.Limit(rps), burst)
	rl.limits[key] = lim
	return lim
}

func burstForRPS(rps float64) int {
	if rps < 1 {
		return 1
	}
	return 5
}

func retryAfter(rps float64) string {
	return strconv.Itoa(int(1/rps) + 1)
}

// clientIP extracts the client IP from RemoteAddr, stripping the port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimitMiddleware applies IP-based search rate limiting and per-site token buckets.
func rateLimitMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	siteRL := newRateLimiters()
	ipRL := newRateLimiters()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// IP-based rate limiting for the search endpoint.
			if r.URL.Path == "/search" {
				ip := clientIP(r)
				searchRPS := cfg.Server.RateLimitSearchRPS
				if searchRPS <= 0 {
					searchRPS = cfg.Server.RateLimitRPS
				}
				if searchRPS > 0 {
					limiter := ipRL.getLimiter("search:"+ip, searchRPS, burstForRPS(searchRPS))
					if !limiter.Allow() {
						w.Header().Set("Retry-After", retryAfter(searchRPS))
						http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
						return
					}
				}
			}

			// Per-site rate limiting.
			site := server.SiteFromContext(r.Context())
			if site != nil && site.RateLimitRPS > 0 {
				limiter := siteRL.getLimiter(site.Name, site.RateLimitRPS, burstForRPS(site.RateLimitRPS))
				if !limiter.Allow() {
					w.Header().Set("Retry-After", retryAfter(site.RateLimitRPS))
					http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// bodyLimitMiddleware restricts request body size.
func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
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
