package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/R0Xps/chirpy/internal/auth"
	"github.com/R0Xps/chirpy/internal/database"
	"github.com/google/uuid"
)

// This struct is used for JSON responses that include a user (without access or refresh tokens)
type user struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

// Handler function for the POST /api/users endpoint
// Creates a new user and responds with 201 and that user on success, and 422 when the given email is used by another user
// Request requirements:
//   - Body:
//     -- Request body must be of the form: '{"email":"{email}", "password":"{password}"}' where {email} is the new user's email, and {password} is that user's password
func (cfg *Config) PostUsersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("(POST /api/users) Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if params.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email is required")
		return
	}

	if params.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Password is required")
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("(POST /api/users) Error hashing password: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	createUserParams := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	}

	dbUser, err := cfg.DB.CreateUser(r.Context(), createUserParams)
	if err != nil {
		log.Printf("Error creating user: %s", err)
		respondWithError(w, http.StatusUnprocessableEntity, "Email is already used by another user")
		return
	}

	responseUser := user{
		ID:          dbUser.ID,
		CreatedAt:   dbUser.CreatedAt,
		UpdatedAt:   dbUser.UpdatedAt,
		Email:       dbUser.Email,
		IsChirpyRed: dbUser.IsChirpyRed,
	}

	respondWithJSON(w, http.StatusCreated, responseUser)
}

// Handler function for the PUT /api/users endpoint
// Updates the logged-in user's email and password, responds with 200 and the updated user on success
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {accessToken}' where {accessToken} is the user's access token, given by the POST /api/login endpoint
//   - Body:
//     -- Request body must be of the form: '{"email":"{email}", "password":"{password}"}' where {email} is the user's new email, and {password} is the new password
func (cfg *Config) PutUsersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	uuid, err := auth.ValidateJWT(token, cfg.Secret)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("(PUT /api/users) Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("(PUT /api/users) Error hashing password: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	updateUserEmailAndPasswordParams := database.UpdateUserEmailAndPasswordParams{
		ID:             uuid,
		Email:          params.Email,
		HashedPassword: hashedPassword,
	}

	dbUser, err := cfg.DB.UpdateUserEmailAndPassword(r.Context(), updateUserEmailAndPasswordParams)
	if err != nil {
		log.Printf("(PUT /api/users) Error updating user email and password: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseUser := user{
		ID:          dbUser.ID,
		CreatedAt:   dbUser.CreatedAt,
		UpdatedAt:   dbUser.UpdatedAt,
		Email:       dbUser.Email,
		IsChirpyRed: dbUser.IsChirpyRed,
	}

	respondWithJSON(w, http.StatusOK, responseUser)
}
