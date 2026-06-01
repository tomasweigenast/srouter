package handler

import (
	"encoding/json"
	"net/http"
)

type jsonResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// writeJSON writes a JSON response. Uses 422 on failure, 200 on success.
func writeJSON(w http.ResponseWriter, ok bool, message string) {
	status := http.StatusOK
	if !ok {
		status = http.StatusUnprocessableEntity
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(jsonResult{OK: ok, Message: message})
}
