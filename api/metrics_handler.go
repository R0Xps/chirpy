package api

import (
	"fmt"
	"net/http"
)

// Handler function for the GET /admin/metrics endpoint
// Responds with 200 and a simple HTML page that shows how many times the file server has been accessed since the last restart/reset
func (cfg *Config) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.FileServerHits.Load())
}
