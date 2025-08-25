package api

import (
	"fmt"
	"log"
	"net/http"
)

// Handler function for the POST /admin/reset endpoint
// This endpoint only works if the PLATFORM environment variable is set to 'dev', responds with 403 otherwise
// Resets the file server hits counter, and clears the users table in the database, responds with 200 on success
func (cfg *Config) ResetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.Platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	err := cfg.DB.ResetUsers(r.Context())
	if err != nil {
		log.Printf("(POST /admin/reset) Error resetting users table: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cfg.FileServerHits.Store(0)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
