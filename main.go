package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/R0Xps/chirpy/internal/auth"
	"github.com/R0Xps/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Program entry point
func main() {
	// Load environment variables from .env
	godotenv.Load()
	// Get the values of environment variables for api config
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")
	polkaKey := os.Getenv("POLKA_KEY")
	// Connect to the database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	// This struct is used for handling file server hits, secrets, and anything else needed by multiple endpoints
	apiConfig := apiConfig{
		dbQueries: dbQueries,
		platform:  platform,
		secret:    secret,
		polkaKey:  polkaKey,
	}
	// Register handler functions for file server and api endpoints
	mux := http.NewServeMux()
	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", apiConfig.middlewareMetricsInc(fileServerHandler))
	mux.HandleFunc("GET /api/healthz", healthzHandler)
	mux.HandleFunc("GET /admin/metrics", apiConfig.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetHandler)
	mux.HandleFunc("GET /api/chirps", apiConfig.getChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiConfig.getChirpHandler)
	mux.HandleFunc("POST /api/chirps", apiConfig.postChirpsHandler)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiConfig.deleteChirpsHandler)
	mux.HandleFunc("POST /api/users", apiConfig.postUsersHandler)
	mux.HandleFunc("PUT /api/users", apiConfig.putUsersHandler)
	mux.HandleFunc("POST /api/login", apiConfig.loginHandler)
	mux.HandleFunc("POST /api/refresh", apiConfig.refreshHandler)
	mux.HandleFunc("POST /api/revoke", apiConfig.revokeHandler)
	mux.HandleFunc("POST /api/polka/webhooks", apiConfig.postPolkaWebhooksHandler)
	// Set the server listening port and start it
	server := http.Server{Handler: mux, Addr: ":8080"}
	server.ListenAndServe()
}

// This struct is used for handling file server hits, secrets, and anything else needed by multiple endpoints
type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
	secret         string
	polkaKey       string
}

// This map acts as a set of words to be replaced with '****' when cleaning the chirp
var badWords = map[string]struct{}{
	"kerfuffle": {},
	"sharbert":  {},
	"fornax":    {},
}

// This struct is used for JSON responses that include a chirp
type chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

// This struct is used for JSON responses that include a user (without access or refresh tokens)
type user struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

// This middleware function is used to increment the file server hits variable whenever the file server endpoint is accessed
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Handler function for the GET /api/healthz endpoint
// Always responds with a 200 status code
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprint(w, "OK")
}

// Handler function for the GET /admin/metrics endpoint
// Responds with 200 and a simple HTML page that shows how many times the file server has been accessed since the last restart/reset
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

// Handler function for the POST /admin/reset endpoint
// This endpoint only works if the PLATFORM environment variable is set to 'dev', responds with 403 otherwise
// Resets the file server hits counter, and clears the users table in the database, responds with 200 on success
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(403)
		return
	}
	err := cfg.dbQueries.ResetUsers(r.Context())
	if err != nil {
		log.Printf("(POST /admin/reset) Error resetting users table: %s", err)
		w.WriteHeader(500)
		return
	}
	cfg.fileServerHits.Store(0)
	w.WriteHeader(200)
	fmt.Fprint(w, "OK")
}

// Handler function for the GET /api/chirps endpoint
// Responds with 200 and a list of chirps
// Optional query parameters:
//   - author_id={userID} -> responds with all chirps made by the user with UUID of {userID}, if omitted responds with all chirps in the database
//   - sort={order} -> {order} must be 'asc' or 'desc', defaults to 'asc' if omitted, orders returned chirps by creation time in (asc)ending or (desc)ending order
func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	authorId := r.URL.Query().Get("author_id")
	var dbChirps []database.Chirp
	var err error
	if authorId != "" {
		uuid, err := uuid.Parse(authorId)
		if err != nil {
			respondWithError(w, 400, "Invalid UUID")
			return
		}
		dbChirps, err = cfg.dbQueries.GetChirpsByUser(r.Context(), uuid)
	} else {
		dbChirps, err = cfg.dbQueries.GetChirps(r.Context())
	}
	if err != nil {
		log.Printf("(GET /api/chirps) Error fetching chirps from database: %s", err)
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

	order := r.URL.Query().Get("sort")
	if order == "" || order == "asc" {
		slices.SortFunc(chirps, func(a, b chirp) int {
			return a.CreatedAt.Compare(b.CreatedAt)
		})
	} else if order == "desc" {
		slices.SortFunc(chirps, func(a, b chirp) int {
			return b.CreatedAt.Compare(a.CreatedAt)
		})
	} else {
		respondWithError(w, 400, "Invalid sort parameter (must be 'asc' or 'desc')")
		return
	}

	respondWithJSON(w, 200, chirps)
}

// Handler function for GET /api/chirps/{chirpID} endpoint
// Responds with 200 and the chirp that has UUID chirpID if it exists, and 404 otherwise
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

// Handler function for the POST /api/chirps endpoint
// Creates a new chirp and responds with 201 and that chirp on success
// If the chirp includes any of the words in the badWords map defined above (exact match, case-insensitive), it will be replaced with '****'
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {accessToken}' where {accessToken} is the user's access token, given by the POST /api/login endpoint
//   - Body:
//     -- Request body must be of the form: '{"body":"{text}"}' where {text} can be any string of length 140 or less
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
		log.Printf("(POST /api/chirps) Error decoding parameters: %s", err)
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
		log.Printf("(POST /api/chirps) Error creating chirp: %s", err)
		w.WriteHeader(500)
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

// This function is used to trim and extra spaces in the beginning or end of a string, and replace any words in the badWords map with '****'
func cleanChirp(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if _, ok := badWords[strings.ToLower(w)]; ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

// Handler function for the DELETE /api/chirps/{chirpID} endpoint
// Deletes the chirp with UUID {chirpID} from the database, responds with 204 on success
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {accessToken}' where {accessToken} is the chirp author's access token, given by the POST /api/login endpoint
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
		log.Printf("(DELETE /api/chirps/{chirpID}) Error deleting chirp: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(204)
}

// Handler function for the POST /api/users endpoint
// Creates a new user and responds with 201 and that user on success, and 422 when the given email is used by another user
// Request requirements:
//   - Body:
//     -- Request body must be of the form: '{"email":"{email}", "password":"{password}"}' where {email} is the new user's email, and {password} is that user's password
func (cfg *apiConfig) postUsersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("(POST /api/users) Error decoding parameters: %s", err)
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
		log.Printf("(POST /api/users) Error hashing password: %s", err)
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
		respondWithError(w, 422, "Email is already used by another user")
		return
	}

	responseUser := user{
		ID:          dbUser.ID,
		CreatedAt:   dbUser.CreatedAt,
		UpdatedAt:   dbUser.UpdatedAt,
		Email:       dbUser.Email,
		IsChirpyRed: dbUser.IsChirpyRed,
	}

	respondWithJSON(w, 201, responseUser)
}

// Handler function for the PUT /api/users endpoint
// Updates the logged-in user's email and password, responds with 200 and the updated user on success
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {accessToken}' where {accessToken} is the user's access token, given by the POST /api/login endpoint
//   - Body:
//     -- Request body must be of the form: '{"email":"{email}", "password":"{password}"}' where {email} is the user's new email, and {password} is the new password
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
		log.Printf("(PUT /api/users) Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("(PUT /api/users) Error hashing password: %s", err)
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
		log.Printf("(PUT /api/users) Error updating user email and password: %s", err)
		w.WriteHeader(500)
		return
	}

	responseUser := user{
		ID:          dbUser.ID,
		CreatedAt:   dbUser.CreatedAt,
		UpdatedAt:   dbUser.UpdatedAt,
		Email:       dbUser.Email,
		IsChirpyRed: dbUser.IsChirpyRed,
	}

	respondWithJSON(w, 200, responseUser)
}

// Handler function for the POST /api/login endpoint
// Validates the given email and password, responds with 200 and the user's info (including access and refresh tokens) on success, and 401 otherwise
// Handler function for the POST /api/users endpoint
// Creates a new user and responds with 201 and that user on success, and 422 when the given email is used by another user
// Request requirements:
//   - Body:
//     -- Request body must be of the form: '{"email":"{email}", "password":"{password}"}' where {email} is the user's email, and {password} is that user's password
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
		IsChirpyRed  bool      `json:"is_chirpy_red"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("(POST /api/login) Error decoding parameters: %s", err)
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
		log.Printf("(POST /api/login) Error creating JWT: %s", err)
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
		log.Printf("(POST /api/login) Error adding refresh token to database: %s", err)
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
		IsChirpyRed:  dbUser.IsChirpyRed,
	}

	respondWithJSON(w, 200, responseUser)
}

// Handler function for the POST /api/refresh endpoint
// Generates a new access token for the user whose refresh token was used for authorization
// Responds with 200 and the new access token on success, and 401 otherwise
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {refreshToken}' where {refreshToken} is the user's refresh token, given by the POST /api/login endpoint
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
		log.Printf("(POST /api/refresh) Error making JWT: %s", err)
		w.WriteHeader(500)
		return
	}

	accessTokenResponse := accessToken{
		Token: newToken,
	}

	respondWithJSON(w, 200, accessTokenResponse)
}

// Handler function for the POST /api/revoke endpoint
// Revokes the refresh token used to authorize this request and responds with 204 on success
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {refreshToken}' where {refreshToken} is the user's refresh token, given by the POST /api/login endpoint
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

// Handler function for the POST /api/polka/webhooks endpoint
// Used by Polka to inform us of a user's payment to upgrade to Chirpy Red
// Responds with 204 unless the given UUID does not belong to a user, then it responds with 404
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: ApiKey {apiKey}' where {apiKey} must match out Polka api key that is stored in the POLKA_KEY environment variable
//   - Body:
//     -- Request body must be of the form: '{"event":"user.upgraded", "data":{"user_id":"{userID}"}}' where {userID} is the UUID of the user who paid to upgrade
func (cfg *apiConfig) postPolkaWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	key, err := auth.GetApiKey(r.Header)
	if err != nil || key != cfg.polkaKey {
		w.WriteHeader(401)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("(POST /api/polka/webhooks) Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if params.Event != "user.upgraded" {
		w.WriteHeader(204)
		return
	}
	userID, err := uuid.Parse(params.Data.UserID)
	if err != nil {
		respondWithError(w, 400, "Invalid UUID")
		return
	}

	_, err = cfg.dbQueries.GetUserById(r.Context(), userID)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	err = cfg.dbQueries.UpgradeUserToChirpyRed(r.Context(), userID)
	if err != nil {
		log.Printf("(POST /api/polka/webhooks) Error upgrading user to chirpy red: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(204)
}

// Writes a response to the given http.ResponseWriter with the given code, and a body of '{"error": "{errorMsg}"}' where {errorMsg} is the msg argument
func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	response := errorResponse{
		Error: msg,
	}
	respondWithJSON(w, code, response)
}

// Writes a JSON response to the given http.ResponseWriter with the given code, and a body matching the payload argument
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	data, err := json.Marshal(payload)

	if err != nil {
		log.Printf("(respondWithJSON()) Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}
