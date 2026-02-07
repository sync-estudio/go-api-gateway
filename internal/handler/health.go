package handler

import "net/http"

// HealthHandler returns a handler that responds with the health status of the API gateway.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"api-gateway"}`))
	}
}
