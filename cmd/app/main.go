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
	service, err := RegisterService("http://localhost:4000")

	if err != nil {
		log.Fatal(err)
	}

	// MUX, Route, Registered service (/%route%/)
	err = RegisterHandler(mux, "/api/", service)
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

	if !strings.HasSuffix(serviceRoute, "/") {
		// serviceRoute = serviceRoute + "/" // Enforce sub - pathing
	}

	mux.Handle(serviceRoute, proxy)
	return nil
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		log.Printf(
			"[REQ] %s %s | %s | %s",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			time.Since(start),
		)
	})
}
