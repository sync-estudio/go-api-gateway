package proxy

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewProxy(targetURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetURL)

	if err != nil {
		return nil, errors.New("invalid url (parsing error)")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	}

	return proxy, nil
}

// It handles both exact routes and subtree routing with proper path stripping.
func RegisterHandler(mux *http.ServeMux, serviceRoute string, proxy *httputil.ReverseProxy) error {
	if !strings.HasPrefix(serviceRoute, "/") {
		return errors.New("route must start with /")
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
