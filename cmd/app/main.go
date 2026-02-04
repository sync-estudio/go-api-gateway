package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var registeredServices []ServiceConfig

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatal("Error loading .env file: ", err)
	}

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	services, err := LoadServicesFromEnv()

	if err != nil {
		log.Fatal(err)
	}

	for _, service := range services {
		proxy, err := RegisterService(service.URL)
		if err != nil {
			log.Fatal(err)
		}

		// MUX, Route, Registered service (/%route%/)
		err = RegisterHandler(mux, service.Alias, proxy)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("[PROXY] Registered route: %s -> %s", service.Alias, service.URL)
	}

	registeredServices = services

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"api-gateway"}`))
	})
	log.Println("[PROXY] Registered health check: /health")

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only handle exact root path
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Build routes list
		routes := []string{"/health"}
		for _, service := range registeredServices {
			routes = append(routes, service.Alias)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Manual JSON construction to avoid encoding/json import
		routesJSON := `["` + strings.Join(routes, `","`) + `"]`
		response := `{"message":"API Gateway","routes":` + routesJSON + `}`
		w.Write([]byte(response))
	})
	log.Println("[PROXY] Registered root endpoint: /")

	fmt.Println("[Proxy] Started on " + port)
	err = http.ListenAndServe(":"+port, LoggingMiddleware(mux))

	if err != nil {
		log.Fatal("Failed to start server: " + err.Error())
	}

}

type ServiceConfig struct {
	URL   string
	Alias string
}

func LoadServicesFromEnv() ([]ServiceConfig, error) {
	rawURLs := strings.TrimSpace(os.Getenv("SERVICES_URL"))
	rawAliases := strings.TrimSpace(os.Getenv("SERVICES_ALIASES"))

	if rawURLs == "" || rawAliases == "" {
		return nil, errors.New("Missing SERVICES_URL or SERVICES_ALIASES")
	}

	urls := splitAndTrim(rawURLs)
	aliases := splitAndTrim(rawAliases)

	if len(urls) != len(aliases) {
		return nil, errors.New("SERVICES_URL and SERVICES_ALIASES must have the same number of entries")
	}

	services := make([]ServiceConfig, 0, len(urls))
	for i := range urls {
		if urls[i] == "" || aliases[i] == "" {
			return nil, errors.New("SERVICES_URL and SERVICES_ALIASES entries cannot be empty")
		}

		// Normalize URL by removing trailing slash
		normalizedURL := strings.TrimSuffix(urls[i], "/")

		// Validate URL format
		parsedURL, err := url.Parse(normalizedURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, fmt.Errorf("invalid URL '%s': must have valid scheme and host", urls[i])
		}

		services = append(services, ServiceConfig{
			URL:   normalizedURL,
			Alias: aliases[i],
		})
	}

	return services, nil
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func RegisterService(urlPath string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(urlPath)

	if err != nil {
		return nil, errors.New("Invalid url (parsing error)")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the Director to preserve the correct Host header for Railway routing
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	}

	return proxy, nil
}

func RegisterHandler(mux *http.ServeMux, serviceRoute string, proxy *httputil.ReverseProxy) error {
	if !strings.HasPrefix(serviceRoute, "/") {
		return errors.New("Route must start with /")
	}

	// Normalize to have a trailing slash for subtree routing.
	if strings.HasSuffix(serviceRoute, "/") {
		// Strip prefix when proxying.
		mux.Handle(serviceRoute, http.StripPrefix(strings.TrimSuffix(serviceRoute, "/"), proxy))
		return nil
	}

	base := serviceRoute + "/"

	// Redirect bare path to trailing slash.
	mux.Handle(serviceRoute, http.RedirectHandler(base, http.StatusMovedPermanently))

	// Strip prefix when proxying.
	mux.Handle(base, http.StripPrefix(strings.TrimSuffix(base, "/"), proxy))
	return nil
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Determine upstream target
		upstreamTarget := "N/A"
		for _, service := range registeredServices {
			if strings.HasPrefix(r.URL.Path, service.Alias) {
				upstreamTarget = service.URL
				break
			}
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Build query string for logging
		queryString := ""
		if r.URL.RawQuery != "" {
			queryString = "?" + r.URL.RawQuery
		}

		// Log selected headers (excluding sensitive ones)
		headers := make([]string, 0)
		for key, values := range r.Header {
			// Skip potentially sensitive headers
			if key == "Authorization" || key == "Cookie" {
				headers = append(headers, fmt.Sprintf("%s: [REDACTED]", key))
			} else {
				headers = append(headers, fmt.Sprintf("%s: %s", key, strings.Join(values, ", ")))
			}
		}
		headersStr := strings.Join(headers, " | ")

		log.Printf(
			"[REQUEST] %s %s%s | From: %s | To: %s | Status: %s | Duration: %s | Headers: %s",
			r.Method,
			r.URL.Path,
			queryString,
			r.RemoteAddr,
			upstreamTarget,
			http.StatusText(rec.status),
			time.Since(start),
			headersStr,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
