package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/R0Xps/chirpy/api"
	"github.com/R0Xps/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Program entry point
func main() {
	// Load environment variables from .env
	godotenv.Load()
	// Get the values of environment variables for api config
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL environment variable not set")
	}
	platform := os.Getenv("PLATFORM")
	if platform == "" {
		log.Fatal("PLATFORM environment variable not set")
	}
	secret := os.Getenv("SECRET")
	if secret == "" {
		log.Fatal("SECRET environment variable not set")
	}
	polkaKey := os.Getenv("POLKA_KEY")
	if polkaKey == "" {
		log.Fatal("POLKA_KEY environment variable not set")
	}
	// Connect to the database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	// This struct is used for handling file server hits, secrets, and anything else needed by multiple endpoints
	apiConfig := api.Config{
		DB:       dbQueries,
		Platform: platform,
		Secret:   secret,
		PolkaKey: polkaKey,
	}
	// Register handler functions for file server and api endpoints
	mux := http.NewServeMux()
	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", apiConfig.MiddlewareMetricsInc(fileServerHandler))
	mux.HandleFunc("GET /api/healthz", api.HealthzHandler)
	mux.HandleFunc("GET /admin/metrics", apiConfig.MetricsHandler)
	mux.HandleFunc("POST /admin/reset", apiConfig.ResetHandler)
	mux.HandleFunc("GET /api/chirps", apiConfig.GetChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiConfig.GetChirpHandler)
	mux.HandleFunc("POST /api/chirps", apiConfig.PostChirpsHandler)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiConfig.DeleteChirpsHandler)
	mux.HandleFunc("POST /api/users", apiConfig.PostUsersHandler)
	mux.HandleFunc("PUT /api/users", apiConfig.PutUsersHandler)
	mux.HandleFunc("POST /api/login", apiConfig.LoginHandler)
	mux.HandleFunc("POST /api/refresh", apiConfig.RefreshHandler)
	mux.HandleFunc("POST /api/revoke", apiConfig.RevokeHandler)
	mux.HandleFunc("POST /api/polka/webhooks", apiConfig.PostPolkaWebhooksHandler)
	// Set the server listening port and start it
	server := http.Server{Handler: mux, Addr: ":8080"}
	server.ListenAndServe()
}
