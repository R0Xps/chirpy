package api

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/R0Xps/chirpy/internal/auth"
	"github.com/R0Xps/chirpy/internal/database"
	"github.com/google/uuid"
)

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

// Handler function for the GET /api/chirps endpoint
// Responds with 200 and a list of chirps
// Optional query parameters:
//   - author_id={userID} -> responds with all chirps made by the user with UUID of {userID}, if omitted responds with all chirps in the database
//   - sort={order} -> {order} must be 'asc' or 'desc', defaults to 'asc' if omitted, orders returned chirps by creation time in (asc)ending or (desc)ending order
func (cfg *Config) GetChirpsHandler(w http.ResponseWriter, r *http.Request) {
	authorId := r.URL.Query().Get("author_id")
	var dbChirps []database.Chirp
	var err error
	if authorId != "" {
		uuid, err := uuid.Parse(authorId)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid UUID")
			return
		}
		dbChirps, err = cfg.DB.GetChirpsByUser(r.Context(), uuid)
	} else {
		dbChirps, err = cfg.DB.GetChirps(r.Context())
	}
	if err != nil {
		log.Printf("(GET /api/chirps) Error fetching chirps from database: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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
		respondWithError(w, http.StatusBadRequest, "Invalid sort parameter (must be 'asc' or 'desc')")
		return
	}

	respondWithJSON(w, http.StatusOK, chirps)
}

// Handler function for GET /api/chirps/{chirpID} endpoint
// Responds with 200 and the chirp that has UUID chirpID if it exists, and 404 otherwise
func (cfg *Config) GetChirpHandler(w http.ResponseWriter, r *http.Request) {
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid UUID")
		return
	}

	dbChirp, err := cfg.DB.GetChirpById(r.Context(), chirpID)

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	responseChirp := chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}

	respondWithJSON(w, http.StatusOK, responseChirp)
}

// Handler function for the POST /api/chirps endpoint
// Creates a new chirp and responds with 201 and that chirp on success
// If the chirp includes any of the words in the badWords map defined above (exact match, case-insensitive), it will be replaced with '****'
// Request requirements:
//   - Authorization Headers:
//     -- Requires headers of the format 'Authorization: Bearer {accessToken}' where {accessToken} is the user's access token, given by the POST /api/login endpoint
//   - Body:
//     -- Request body must be of the form: '{"body":"{text}"}' where {text} can be any string of length 140 or less
func (cfg *Config) PostChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
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
		log.Printf("(POST /api/chirps) Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	createChirpParams := database.CreateChirpParams{
		Body:   cleanChirp(params.Body),
		UserID: uuid,
	}

	dbChirp, err := cfg.DB.CreateChirp(r.Context(), createChirpParams)

	if err != nil {
		log.Printf("(POST /api/chirps) Error creating chirp: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseChirp := chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}

	respondWithJSON(w, http.StatusCreated, responseChirp)
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
func (cfg *Config) DeleteChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid UUID")
		return
	}

	dbChirp, err := cfg.DB.GetChirpById(r.Context(), chirpID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
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

	if dbChirp.UserID != uuid {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = cfg.DB.DeleteChirpById(r.Context(), dbChirp.ID)
	if err != nil {
		log.Printf("(DELETE /api/chirps/{chirpID}) Error deleting chirp: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
