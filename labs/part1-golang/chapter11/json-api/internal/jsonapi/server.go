package jsonapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// NewServer wires routes to the private service implementation.
func NewServer(service EchoService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/echo", EchoHandler(service))
	return mux
}

// EchoHandler decodes JSON requests and delegates learning logic to EchoService.
func EchoHandler(service EchoService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		response, err := service.Echo(req.Message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Errorf("write echo response: %w", err).Error(), http.StatusInternalServerError)
		}
	}
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
