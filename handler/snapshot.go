package handler

import (
	"encoding/json"
	"fmt"
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

func (h *SnapshotHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	courtID := r.PathValue("courtId")

	if !uuidRegex.MatchString(courtID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid court ID"})
		return
	}

	snapshots, err := h.Storage.ListSnapshots(courtID)
	if err != nil {
		if err.Error() == "court not found" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		} else {
			log.Printf("error listing snapshots for court %s: %v", courtID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}

	baseURL := fmt.Sprintf("/snapshots/%s/file", courtID)
	type fileEntry struct {
		Filename  string `json:"filename"`
		SizeBytes int64  `json:"size_bytes"`
		CreatedAt string `json:"created_at"`
		OpenURL   string `json:"open_url"`
	}

	files := make([]fileEntry, len(snapshots))
	for i, s := range snapshots {
		files[i] = fileEntry{
			Filename:  s.Filename,
			SizeBytes: s.Size,
			CreatedAt: s.CreatedAt.Format("2006-01-02 15:04:05"),
			OpenURL:   fmt.Sprintf("%s/%s", baseURL, s.Filename),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"court_id": courtID,
		"count":    len(files),
		"files":    files,
	})
}

func (h *SnapshotHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	courtID := r.PathValue("courtId")
	filename := r.PathValue("filename")

	if !uuidRegex.MatchString(courtID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid court ID"})
		return
	}

	filePath, err := h.Storage.GetSnapshotFile(courtID, filename)
	if err != nil {
		switch err.Error() {
		case "court not found", "file not found":
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		default:
			log.Printf("error getting file %s for court %s: %v", filename, courtID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("error reading file %s: %v", filePath, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

func (h *SnapshotHandler) ServeThumbnails(w http.ResponseWriter, r *http.Request) {
	courtID := r.PathValue("courtId")

	if !uuidRegex.MatchString(courtID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid court ID"})
		return
	}

	snapshots, err := h.Storage.ListSnapshots(courtID)
	if err != nil {
		if err.Error() == "court not found" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		} else {
			log.Printf("error listing snapshots for court %s: %v", courtID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}

	limit := 100
	if len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}

	key := r.URL.Query().Get("key")
	keyParam := ""
	if key != "" {
		keyParam = "?key=" + key
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Snapshots - %s</title>
<style>body{background:#111;color:#fff;font-family:monospace;margin:20px}
h1{font-size:16px}p{color:#888;font-size:12px}
.grid{display:flex;flex-wrap:wrap;gap:10px}
.grid a{display:block}.grid img{width:200px;height:auto;border:1px solid #333}</style>
</head><body>`, courtID)
	fmt.Fprintf(w, `<h1>Court %s</h1><p>%d snapshots</p><div class="grid">`, courtID, len(snapshots))

	for _, s := range snapshots {
		title := s.CreatedAt.Format("2006-01-02 15:04:05")
		imgURL := fmt.Sprintf("/snapshots/%s/file/%s%s", courtID, s.Filename, keyParam)
		fmt.Fprintf(w, `<a href="%s" target="_blank"><img src="%s" alt="%s" title="%s" loading="lazy"></a>`,
			imgURL, imgURL, title, title)
	}

	fmt.Fprint(w, `</div></body></html>`)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
