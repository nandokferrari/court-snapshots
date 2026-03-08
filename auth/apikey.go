package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

func RequireAPIKey(apiKey string) func(http.Handler) http.Handler {
	keyBytes := []byte(apiKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				t, ok := strings.CutPrefix(authHeader, "Bearer ")
				if ok {
					token = t
				}
			}
			if token == "" {
				token = r.URL.Query().Get("key")
			}

			if token == "" || subtle.ConstantTimeCompare([]byte(token), keyBytes) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
