package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func main() {
	port := "8082"
	mux := http.NewServeMux()
	authService, err := RegisterService("http://localhost:8080")

	if err != nil {
		log.Fatal(err)
	}

	// MUX, Route, Registered service (/%route%/)
	err = RegisterHandler(mux, "/warehouse", authService)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("[Proxy] Started on " + port)
	err = http.ListenAndServe(":"+port, LoggingMiddleware(mux))

	if err != nil {
		log.Fatal("Failed to start server: " + err.Error())
	}

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
