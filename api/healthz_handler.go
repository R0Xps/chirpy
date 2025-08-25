package api

import (
	"fmt"
	"net/http"
)

// Handler function for the GET /api/healthz endpoint
// Always responds with a 200 status code
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
