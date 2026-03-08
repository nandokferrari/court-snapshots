package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/nandokferrari/court-snapshots/storage"
)

var uuidRegex = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

type SnapshotHandler struct {
	Storage          *storage.DiskStorage
	DeleteAfterServe bool
}

func (h *SnapshotHandler) ServeLatest(w http.ResponseWriter, r *http.Request) {
	courtID := r.PathValue("courtId")

	if !uuidRegex.MatchString(courtID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid court ID"})
		return
	}

	filePath, err := h.Storage.GetLatestSnapshot(courtID)
	if err != nil {
		switch err.Error() {
		case "court not found", "no snapshot available":
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		default:
			log.Printf("error getting snapshot for court %s: %v", courtID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("error reading snapshot file %s: %v", filePath, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Snapshot-File", filepath.Base(filePath))
	w.Write(data)

	if h.DeleteAfterServe {
		go func() {
			if err := h.Storage.DeleteFile(filePath); err != nil {
				log.Printf("error deleting snapshot %s: %v", filePath, err)
			}
		}()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
