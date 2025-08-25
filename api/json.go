package api

import (
	"encoding/json"
	"log"
	"net/http"
)

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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}
