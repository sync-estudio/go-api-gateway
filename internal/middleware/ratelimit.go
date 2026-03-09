package middleware

import (
	"context"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"sync.gateway/internal/service"
)

const (
	defaultRateLimitRequests = int64(200)
	defaultRateLimitWindow   = time.Minute
)

func NewRateLimiter(client *redis.Client, registry *service.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client == nil {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			requests := defaultRateLimitRequests
			window := defaultRateLimitWindow
			alias := "global"

			if registry != nil {
				svc := registry.FindServiceForPath(r.URL.Path)
				if svc != nil {
					alias = svc.Alias
					if svc.RateLimit.Requests > 0 {
						requests = svc.RateLimit.Requests
					}
					if svc.RateLimit.Window > 0 {
						window = svc.RateLimit.Window.Std()
					}
				}
			}

			key := "rl:" + alias + ":" + ip
			ctx := context.Background()

			countCmd := client.Incr(ctx, key)
			if err := countCmd.Err(); err != nil {
				log.Printf("[RATE-LIMITER] Redis INCR failed for ip=%s: %v", ip, err)
				next.ServeHTTP(w, r)
				return
			}

			count := countCmd.Val()
			if count == 1 {
				if err := client.Expire(ctx, key, window).Err(); err != nil {
					log.Printf("[RATE-LIMITER] Redis EXPIRE failed for ip=%s: %v", ip, err)
				}
			}

			if count > requests {
				w.Header().Set("Retry-After", retryAfterSeconds(window))
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func retryAfterSeconds(window time.Duration) string {
	secs := int(window.Seconds())
	if secs <= 0 {
		secs = 60
	}
	return strconv.Itoa(secs)
}

func clientIP(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
