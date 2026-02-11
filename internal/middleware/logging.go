package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"sync.gateway/internal/auth"
	"sync.gateway/internal/service"
)

// NewLoggingMiddleware creates a logging middleware that logs request details.
// It uses the registry to find which service handles each request.
func NewLoggingMiddleware(registry *service.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		requestLogger := log.New(os.Stdout, "", 0)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := getOrCreateRequestID(r)
			w.Header().Set("X-Request-Id", requestID)

			serviceAlias, upstreamTarget := registry.FindForPath(r.URL.Path)

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			durationMs := time.Since(start).Milliseconds()
			remoteIP := extractRemoteIP(r.RemoteAddr)
			userAgent := r.UserAgent()
			xForwardedFor := r.Header.Get("X-Forwarded-For")
			xForwardedProto := r.Header.Get("X-Forwarded-Proto")
			xForwardedHost := r.Header.Get("X-Forwarded-Host")

			// Extract user ID from context if available (set by auth middleware)
			userID := ""
			if identity, ok := auth.FromContext(r.Context()); ok && identity != nil {
				userID = identity.UserID
			}

			fields := []string{
				fmt.Sprintf("timestamp=%q", time.Now().UTC().Format(time.RFC3339Nano)),
				fmt.Sprintf("request_id=%q", requestID),
				fmt.Sprintf("method=%q", r.Method),
				fmt.Sprintf("path=%q", r.URL.Path),
				fmt.Sprintf("query=%q", r.URL.RawQuery),
				fmt.Sprintf("status=%d", rec.status),
				fmt.Sprintf("duration_ms=%d", durationMs),
				fmt.Sprintf("bytes=%d", rec.bytes),
				fmt.Sprintf("service_alias=%q", serviceAlias),
				fmt.Sprintf("service_upstream=%q", upstreamTarget),
				fmt.Sprintf("remote_ip=%q", remoteIP),
				fmt.Sprintf("user_id=%q", userID),
				fmt.Sprintf("user_agent=%q", userAgent),
				fmt.Sprintf("x_forwarded_for=%q", xForwardedFor),
				fmt.Sprintf("x_forwarded_proto=%q", xForwardedProto),
				fmt.Sprintf("x_forwarded_host=%q", xForwardedHost),
			}

			requestLogger.Println(strings.Join(fields, " "))
		})
	}
}

// statusRecorder wraps http.ResponseWriter to capture status code and bytes written.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// getOrCreateRequestID returns the existing request ID from headers or generates a new one.
func getOrCreateRequestID(r *http.Request) string {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	if requestID != "" {
		return requestID
	}
	return generateRequestID()
}

// generateRequestID creates a random request ID.
func generateRequestID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// extractRemoteIP extracts the IP address from a remote address string.
func extractRemoteIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
