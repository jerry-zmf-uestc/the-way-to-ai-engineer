package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"unicode/utf8"
)

// ErrEmptyMessage reports an echo request that contains no useful content.
var ErrEmptyMessage = errors.New("message is required")

// EchoRequest is the JSON payload accepted by POST /api/echo.
type EchoRequest struct {
	Message string `json:"message"`
}

// EchoResponse is the stable JSON payload returned by POST /api/echo.
type EchoResponse struct {
	Echo   string `json:"echo"`
	Length int    `json:"length"`
}

// NewServer builds the minimal HTTP handler for this topic.
func NewServer() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/echo", EchoHandler)
	return mux
}

// EchoHandler decodes a JSON request and returns a deterministic JSON response.
func EchoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EchoRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	message := strings.TrimSpace(req.Message)
	if err := validateEchoMessage(message); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(EchoResponse{
		Echo:   message,
		Length: utf8.RuneCountInString(message),
	}); err != nil {
		http.Error(w, fmt.Errorf("write echo response: %w", err).Error(), http.StatusInternalServerError)
	}
}

func validateEchoMessage(message string) error {
	if strings.TrimSpace(message) == "" {
		return ErrEmptyMessage
	}

	return nil
}

// DemoRequest calls /api/echo through httptest and renders deterministic output.
func DemoRequest(handler http.Handler) string {
	body := bytes.NewBufferString(`{"message":"hello, json api"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/echo", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	response := strings.TrimSpace(rec.Body.String())
	return fmt.Sprintf("%s %s -> %d %s", req.Method, req.URL.Path, rec.Code, response)
}

func main() {
	fmt.Println(DemoRequest(NewServer()))
}
