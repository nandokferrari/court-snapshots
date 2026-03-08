package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/nandokferrari/court-snapshots/auth"
	"github.com/nandokferrari/court-snapshots/config"
	"github.com/nandokferrari/court-snapshots/handler"
	"github.com/nandokferrari/court-snapshots/storage"
)

func New(cfg *config.Config) *http.Server {
	store := storage.NewDiskStorage(cfg.SnapshotsDir)
	snapshotHandler := &handler.SnapshotHandler{
		Storage:          store,
		DeleteAfterServe: cfg.DeleteAfterServe,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	authMiddleware := auth.RequireAPIKey(cfg.APIKey)
	mux.Handle("GET /snapshots/{courtId}/latest", authMiddleware(http.HandlerFunc(snapshotHandler.ServeLatest)))

	loggedMux := loggingMiddleware(mux)

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggedMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}
