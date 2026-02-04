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

func main() {
	err := godotenv.Load()

	if err != nil {
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
	}

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
		services = append(services, ServiceConfig{
			URL:   urls[i],
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

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		log.Printf(
			"[REQUEST] %s %s | %s | %s | %s",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			http.StatusText(rec.status),
			time.Since(start),
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
