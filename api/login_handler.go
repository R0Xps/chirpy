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

// Handler function for the POST /api/login endpoint
// Validates the given email and password, responds with 200 and the user's info (including access and refresh tokens) on success, and 401 otherwise
// Handler function for the POST /api/users endpoint
// Creates a new user and responds with 201 and that user on success, and 422 when the given email is used by another user
// Request requirements:
//   - Body:
//     -- Request body must be of the form: '{"email":"{email}", "password":"{password}"}' where {email} is the user's email, and {password} is that user's password
func (cfg *Config) LoginHandler(w http.ResponseWriter, r *http.Request) {
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dbUser, err := cfg.DB.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "incorrect email or password")
		return
	}

	err = auth.CheckPasswordHash(params.Password, dbUser.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "incorrect email or password")
		return
	}

	token, err := auth.MakeJWT(dbUser.ID, cfg.Secret, time.Hour)
	if err != nil {
		log.Printf("(POST /api/login) Error creating JWT: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	refreshToken, _ := auth.MakeRefreshToken()

	createRefreshTokenParams := database.CreateRefreshTokenParams{
		Token:  refreshToken,
		UserID: dbUser.ID,
	}

	_, err = cfg.DB.CreateRefreshToken(r.Context(), createRefreshTokenParams)
	if err != nil {
		log.Printf("(POST /api/login) Error adding refresh token to database: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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

	respondWithJSON(w, http.StatusOK, responseUser)
}
