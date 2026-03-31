package response

import (
	"encoding/json"
	"net/http"
)

type envelope map[string]any

func JSON(w http.ResponseWriter, status int, key string, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(envelope{key: payload})
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, "error", envelope{"message": message})
}
