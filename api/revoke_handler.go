package api

import (
	"log"
	"net/http"
	"time"

	"github.com/R0Xps/chirpy/internal/auth"
)

// Handler function for the POST /api/revoke endpoint
// Revokes the refresh token used to authorize this request and responds with 204 on success
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {refreshToken}' where {refreshToken} is the user's refresh token, given by the POST /api/login endpoint
func (cfg *Config) RevokeHandler(w http.ResponseWriter, r *http.Request) {
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

	err = cfg.DB.RevokeRefreshToken(r.Context(), dbRefreshToken.Token)
	if err != nil {
		log.Printf("Error revoking token: %s", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
