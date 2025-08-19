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

	"github.com/R0Xps/chirpy/internal/auth"
	"github.com/R0Xps/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	apiConfig := apiConfig{
		dbQueries: dbQueries,
		platform:  platform,
		secret:    secret,
	}

	mux := http.NewServeMux()
	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", apiConfig.middlewareMetricsInc(fileServerHandler))
	mux.HandleFunc("GET /api/healthz", healthzHandler)
	mux.HandleFunc("GET /admin/metrics", apiConfig.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetHandler)
	mux.HandleFunc("POST /api/chirps", apiConfig.postChirpsHandler)
	mux.HandleFunc("GET /api/chirps", apiConfig.getChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiConfig.getChirpHandler)
	mux.HandleFunc("POST /api/users", apiConfig.postUsersHandler)
	mux.HandleFunc("POST /api/login", apiConfig.loginHandler)
	mux.HandleFunc("POST /api/refresh", apiConfig.refreshHandler)
	mux.HandleFunc("POST /api/revoke", apiConfig.revokeHandler)
	mux.HandleFunc("PUT /api/users", apiConfig.putUsersHandler)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiConfig.deleteChirpsHandler)
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
	platform       string
	secret         string
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
	if cfg.platform != "dev" {
		w.WriteHeader(403)
		return
	}
	err := cfg.dbQueries.ResetUsers(r.Context())
	if err != nil {
		log.Printf("Error resetting users table: %s", err)
		w.WriteHeader(500)
		return
	}
	cfg.fileServerHits.Store(0)
	w.WriteHeader(200)
	fmt.Fprint(w, "OK")
}

type chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) postChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	uuid, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	createChirpParams := database.CreateChirpParams{
		Body:   cleanChirp(params.Body),
		UserID: uuid,
	}

	dbChirp, err := cfg.dbQueries.CreateChirp(r.Context(), createChirpParams)

	if err != nil {
		respondWithError(w, 422, err.Error())
		return
	}

	responseChirp := chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}

	respondWithJSON(w, 201, responseChirp)
}

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.dbQueries.GetChirpsInOrder(r.Context())

	if err != nil {
		log.Printf("Error fetching chirps from database: %s", err)
		w.WriteHeader(500)
		return
	}

	chirps := make([]chirp, len(dbChirps))

	for i, dbChirp := range dbChirps {
		chirps[i] = chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		}
	}

	respondWithJSON(w, 200, chirps)
}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, r *http.Request) {
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 400, "Invalid UUID")
		return
	}

	dbChirp, err := cfg.dbQueries.GetChirpById(r.Context(), chirpID)

	if err != nil {
		w.WriteHeader(404)
		return
	}

	responseChirp := chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}

	respondWithJSON(w, 200, responseChirp)
}

var bad_words map[string]struct{} = map[string]struct{}{
	"kerfuffle": {},
	"sharbert":  {},
	"fornax":    {},
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

type user struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *apiConfig) postUsersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if params.Email == "" {
		respondWithError(w, 400, "Email is required")
		return
	}

	if params.Password == "" {
		respondWithError(w, 400, "Password is required")
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("Error hashing password: %s", err)
		w.WriteHeader(500)
		return
	}

	createUserParams := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	}

	dbUser, err := cfg.dbQueries.CreateUser(r.Context(), createUserParams)
	if err != nil {
		log.Printf("Error creating user: %s", err)
		w.WriteHeader(500)
		return
	}

	responseUser := user{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}

	respondWithJSON(w, 201, responseUser)
}

func (cfg *apiConfig) putUsersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	uuid, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("Error hashing password: %s", err)
		w.WriteHeader(500)
		return
	}

	updateUserEmailAndPasswordParams := database.UpdateUserEmailAndPasswordParams{
		ID:             uuid,
		Email:          params.Email,
		HashedPassword: hashedPassword,
	}

	dbUser, err := cfg.dbQueries.UpdateUserEmailAndPassword(r.Context(), updateUserEmailAndPasswordParams)
	if err != nil {
		log.Printf("Error updating user email and password: %s", err)
		w.WriteHeader(500)
		return
	}

	responseUser := user{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}

	respondWithJSON(w, 200, responseUser)
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	type loginUser struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	dbUser, err := cfg.dbQueries.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 401, "incorrect email or password")
		return
	}

	err = auth.CheckPasswordHash(params.Password, dbUser.HashedPassword)
	if err != nil {
		respondWithError(w, 401, "incorrect email or password")
		return
	}

	token, err := auth.MakeJWT(dbUser.ID, cfg.secret, time.Hour)
	if err != nil {
		log.Printf("Error creating JWT: %s", err)
		w.WriteHeader(500)
		return
	}

	refreshToken, _ := auth.MakeRefreshToken()

	createRefreshTokenParams := database.CreateRefreshTokenParams{
		Token:  refreshToken,
		UserID: dbUser.ID,
	}

	_, err = cfg.dbQueries.CreateRefreshToken(r.Context(), createRefreshTokenParams)
	if err != nil {
		log.Printf("Error adding refresh token to database: %s", err)
		w.WriteHeader(500)
		return
	}

	responseUser := loginUser{
		ID:           dbUser.ID,
		CreatedAt:    dbUser.CreatedAt,
		UpdatedAt:    dbUser.UpdatedAt,
		Email:        dbUser.Email,
		Token:        token,
		RefreshToken: refreshToken,
	}

	respondWithJSON(w, 200, responseUser)
}

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	dbRefreshToken, err := cfg.dbQueries.GetRefreshToken(r.Context(), token)
	if err != nil || dbRefreshToken.ExpiresAt.Before(time.Now()) || dbRefreshToken.RevokedAt.Valid {
		w.WriteHeader(401)
		return
	}

	type accessToken struct {
		Token string `json:"token"`
	}

	newToken, err := auth.MakeJWT(dbRefreshToken.UserID, cfg.secret, time.Hour)
	if err != nil {
		log.Printf("Error making JWT: %s", err)
		w.WriteHeader(500)
		return
	}

	accessTokenResponse := accessToken{
		Token: newToken,
	}

	respondWithJSON(w, 200, accessTokenResponse)
}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	dbRefreshToken, err := cfg.dbQueries.GetRefreshToken(r.Context(), token)
	if err != nil || dbRefreshToken.ExpiresAt.Before(time.Now()) || dbRefreshToken.RevokedAt.Valid {
		w.WriteHeader(401)
		return
	}

	err = cfg.dbQueries.RevokeRefreshToken(r.Context(), dbRefreshToken.Token)
	if err != nil {
		log.Printf("Error revoking token: %s", err)
	}
	w.WriteHeader(204)
}

func (cfg *apiConfig) deleteChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 400, "Invalid UUID")
		return
	}

	dbChirp, err := cfg.dbQueries.GetChirpById(r.Context(), chirpID)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	uuid, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		w.WriteHeader(401)
		return
	}

	if dbChirp.UserID != uuid {
		w.WriteHeader(403)
		return
	}

	err = cfg.dbQueries.DeleteChirpById(r.Context(), dbChirp.ID)
	if err != nil {
		log.Printf("Error deleting chirp: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(204)
}
