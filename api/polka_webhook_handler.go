package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/R0Xps/chirpy/internal/auth"
	"github.com/google/uuid"
)

// Handler function for the POST /api/polka/webhooks endpoint
// Used by Polka to inform us of a user's payment to upgrade to Chirpy Red
// Responds with 204 unless the given UUID does not belong to a user, then it responds with 404
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: ApiKey {apiKey}' where {apiKey} must match out Polka api key that is stored in the POLKA_KEY environment variable
//   - Body:
//     -- Request body must be of the form: '{"event":"user.upgraded", "data":{"user_id":"{userID}"}}' where {userID} is the UUID of the user who paid to upgrade
func (cfg *Config) PostPolkaWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	key, err := auth.GetApiKey(r.Header)
	if err != nil || key != cfg.PolkaKey {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("(POST /api/polka/webhooks) Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if params.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	userID, err := uuid.Parse(params.Data.UserID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid UUID")
		return
	}

	_, err = cfg.DB.GetUserById(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = cfg.DB.UpgradeUserToChirpyRed(r.Context(), userID)
	if err != nil {
		log.Printf("(POST /api/polka/webhooks) Error upgrading user to chirpy red: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
