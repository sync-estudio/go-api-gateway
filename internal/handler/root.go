package handler

import (
	"net/http"
	"strings"

	"sync.gateway/internal/service"
)

// NewRootHandler creates a handler for the root endpoint that shows available routes.
func NewRootHandler(registry *service.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only handle exact root path
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Build routes list
		routes := []string{"/health"}
		for _, svc := range registry.All() {
			routes = append(routes, svc.Alias)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Manual JSON construction to avoid encoding/json import
		routesJSON := `["` + strings.Join(routes, `","`) + `"]`
		response := `{"message":"API Gateway","routes":` + routesJSON + `}`
		w.Write([]byte(response))
	}
}
