package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

var defaultCORSConfig = CORSConfig{
	AllowedOrigins:   []string{"*"},
	AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
	AllowedHeaders:   []string{"Content-Type", "Authorization"},
	ExposedHeaders:   []string{},
	AllowCredentials: false,
	MaxAge:           86400,
}

func NewCorsMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = &defaultCORSConfig
	}

	cfg := *config
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = defaultCORSConfig.AllowedOrigins
	}
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = defaultCORSConfig.AllowedMethods
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = defaultCORSConfig.AllowedHeaders
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = defaultCORSConfig.MaxAge
	}

	allowedOrigins := make(map[string]bool)
	for _, origin := range cfg.AllowedOrigins {
		allowedOrigins[strings.ToLower(origin)] = true
	}

	methods := make(map[string]bool)
	for _, m := range cfg.AllowedMethods {
		methods[strings.ToUpper(m)] = true
	}

	headers := make(map[string]bool)
	for _, h := range cfg.AllowedHeaders {
		headers[strings.ToLower(h)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if r.Method == "OPTIONS" {
				handlePreflight(w, r, origin, &cfg, allowedOrigins, methods, headers)
				return
			}

			handleActualRequest(w, r, origin, &cfg, allowedOrigins)
			next.ServeHTTP(w, r)
		})
	}
}

func handlePreflight(
	w http.ResponseWriter,
	r *http.Request,
	origin string,
	cfg *CORSConfig,
	allowedOrigins map[string]bool,
	methods map[string]bool,
	headers map[string]bool,
) {
	appendVary(w.Header(), "Origin")
	appendVary(w.Header(), "Access-Control-Request-Method")
	appendVary(w.Header(), "Access-Control-Request-Headers")

	if !isOriginAllowed(origin, allowedOrigins) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	requestMethod := strings.ToUpper(strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")))
	if requestMethod != "" && !methods[requestMethod] {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	requestHeaders := strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",")
	for _, requestHeader := range requestHeaders {
		headerName := strings.ToLower(strings.TrimSpace(requestHeader))
		if headerName == "" {
			continue
		}
		if !headers[headerName] {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	allowOrigin := origin
	if allowedOrigins["*"] && !cfg.AllowCredentials {
		allowOrigin = "*"
	}

	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))

	if cfg.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if cfg.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleActualRequest(
	w http.ResponseWriter,
	r *http.Request,
	origin string,
	cfg *CORSConfig,
	allowedOrigins map[string]bool,
) {
	appendVary(w.Header(), "Origin")

	if !isOriginAllowed(origin, allowedOrigins) {
		return
	}

	allowOrigin := origin
	if allowedOrigins["*"] && !cfg.AllowCredentials {
		allowOrigin = "*"
	}

	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)

	if len(cfg.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
	}

	if cfg.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

func isOriginAllowed(origin string, allowedOrigins map[string]bool) bool {
	if origin == "" {
		return false
	}

	originLower := strings.ToLower(origin)

	if allowedOrigins["*"] {
		return true
	}

	return allowedOrigins[originLower]
}

func appendVary(headers http.Header, value string) {
	for _, existing := range headers.Values("Vary") {
		for _, token := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(token), value) {
				return
			}
		}
	}
	headers.Add("Vary", value)
}
