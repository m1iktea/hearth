package api

import (
	"encoding/json"
	"net/http"
)

type response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Error   string `json:"error,omitempty"`
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, response{Success: true, Data: data})
}

// writeError 只输出给定消息，不透传内部错误细节（防泄露凭据等）。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, response{Success: false, Error: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
