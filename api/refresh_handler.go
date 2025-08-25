package api

import (
	"log"
	"net/http"
	"time"

	"github.com/R0Xps/chirpy/internal/auth"
)

// Handler function for the POST /api/refresh endpoint
// Generates a new access token for the user whose refresh token was used for authorization
// Responds with 200 and the new access token on success, and 401 otherwise
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {refreshToken}' where {refreshToken} is the user's refresh token, given by the POST /api/login endpoint
func (cfg *Config) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	dbRefreshToken, err := cfg.DB.GetRefreshToken(r.Context(), token)
	if err != nil || dbRefreshToken.ExpiresAt.Before(time.Now()) || dbRefreshToken.RevokedAt.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	type accessToken struct {
		Token string `json:"token"`
	}

	newToken, err := auth.MakeJWT(dbRefreshToken.UserID, cfg.Secret, time.Hour)
	if err != nil {
		log.Printf("(POST /api/refresh) Error making JWT: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	accessTokenResponse := accessToken{
		Token: newToken,
	}

	respondWithJSON(w, http.StatusOK, accessTokenResponse)
}
