package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DiskStorage struct {
	BaseDir string
}

func NewDiskStorage(baseDir string) *DiskStorage {
	return &DiskStorage{BaseDir: baseDir}
}

// isCompleteJPEG checks that the file ends with the JPEG EOI marker (FF D9).
func isCompleteJPEG(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.Size() < 2 {
		return false
	}

	buf := make([]byte, 2)
	if _, err := f.ReadAt(buf, fi.Size()-2); err != nil {
		return false
	}
	return buf[0] == 0xFF && buf[1] == 0xD9
}

func (s *DiskStorage) CourtDir(courtID string) string {
	return filepath.Join(s.BaseDir, "court-"+courtID)
}

func (s *DiskStorage) GetLatestSnapshot(courtID string) (string, error) {
	dir := s.CourtDir(courtID)

	info, err := os.Stat(dir)
	if os.IsNotExist(err) || !info.IsDir() {
		return "", fmt.Errorf("court not found")
	}
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var images []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
			continue
		}
		if !isCompleteJPEG(filepath.Join(dir, e.Name())) {
			continue
		}
		images = append(images, e.Name())
	}

	if len(images) == 0 {
		return "", fmt.Errorf("no snapshot available")
	}

	sort.Strings(images)
	latest := images[len(images)-1]
	return filepath.Join(dir, latest), nil
}

type SnapshotInfo struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *DiskStorage) ListSnapshots(courtID string) ([]SnapshotInfo, error) {
	dir := s.CourtDir(courtID)

	info, err := os.Stat(dir)
	if os.IsNotExist(err) || !info.IsDir() {
		return nil, fmt.Errorf("court not found")
	}
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var snapshots []SnapshotInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
			continue
		}
		filePath := filepath.Join(dir, e.Name())
		if !isCompleteJPEG(filePath) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		snapshots = append(snapshots, SnapshotInfo{
			Filename:  e.Name(),
			Size:      fi.Size(),
			CreatedAt: fi.ModTime(),
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Filename > snapshots[j].Filename
	})

	return snapshots, nil
}

func (s *DiskStorage) GetSnapshotFile(courtID, filename string) (string, error) {
	dir := s.CourtDir(courtID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("court not found")
	}

	filePath := filepath.Join(dir, filepath.Base(filename))
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found")
	}
	if !isCompleteJPEG(filePath) {
		return "", fmt.Errorf("file not ready")
	}

	return filePath, nil
}

func (s *DiskStorage) DeleteFile(filePath string) error {
	return os.Remove(filePath)
}
