package response

import (
	"encoding/json"
	"net/http"
)

type SuccessEnvelope struct {
	Data any `json:"data"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

func Success(w http.ResponseWriter, status int, payload any) {
	writeJSON(w, status, SuccessEnvelope{Data: payload})
}

func Failure(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorEnvelope{Error: ErrorBody{Message: message}})
}
