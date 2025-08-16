package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/R0Xps/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	apiConfig := apiConfig{
		dbQueries: dbQueries,
	}

	mux := http.NewServeMux()
	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", apiConfig.middlewareMetricsInc(fileServerHandler))
	mux.HandleFunc("GET /api/healthz", healthzHandler)
	mux.HandleFunc("GET /admin/metrics", apiConfig.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
	mux.HandleFunc("POST /api/users", apiConfig.usersHandler)
	server := http.Server{Handler: mux, Addr: ":8080"}
	server.ListenAndServe()
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprint(w, "OK")
}

type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries      *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(200)
	fmt.Fprintf(w, `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileServerHits.Load())
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileServerHits.Store(0)
	w.WriteHeader(200)
	fmt.Fprint(w, "OK")
}

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	type cleanedBodyResponse struct {
		CleanedBody string `json:"cleaned_body"`
	}

	response := cleanedBodyResponse{
		CleanedBody: cleanChirp(params.Body),
	}

	respondWithJSON(w, 200, response)
}

var bad_words map[string]struct{} = map[string]struct{}{
	"kerfuffle": struct{}{},
	"sharbert":  struct{}{},
	"fornax":    struct{}{},
}

func cleanChirp(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if _, ok := bad_words[strings.ToLower(w)]; ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	response := errorResponse{
		Error: msg,
	}
	respondWithJSON(w, code, response)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	data, err := json.Marshal(payload)

	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}

	type User struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	dbUser, err := cfg.dbQueries.CreateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Error creating user: %s", err)
		w.WriteHeader(500)
		return
	}

	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}

	respondWithJSON(w, 201, user)
}
