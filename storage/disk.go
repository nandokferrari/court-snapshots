package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type DiskStorage struct {
	BaseDir string
}

func NewDiskStorage(baseDir string) *DiskStorage {
	return &DiskStorage{BaseDir: baseDir}
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
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
			images = append(images, e.Name())
		}
	}

	if len(images) == 0 {
		return "", fmt.Errorf("no snapshot available")
	}

	sort.Strings(images)
	latest := images[len(images)-1]
	return filepath.Join(dir, latest), nil
}

func (s *DiskStorage) CleanupSnapshots(courtID string) error {
	dir := s.CourtDir(courtID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
