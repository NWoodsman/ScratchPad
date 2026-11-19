package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const appName = "ScratchPad"

// defaultDataDir follows each OS's convention for per-user app data:
//
//	Linux:   $XDG_DATA_HOME/ScratchPad or ~/.local/share/ScratchPad
//	macOS:   ~/Library/Application Support/ScratchPad
//	Windows: %LOCALAPPDATA%\ScratchPad
func defaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, appName), nil
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", appName), nil
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, appName), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", appName), nil
}

// Store persists the document to a single file.
type Store struct {
	path string
}

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(dir, "content.txt")}, nil
}

// Load returns the saved document, or "" if none exists yet.
func (s *Store) Load() (string, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// Save writes atomically: a crash mid-write leaves the previous file intact.
func (s *Store) Save(text string) error {
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".content-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

// RunSaver writes unsaved changes every interval, and once more when ctx ends.
func RunSaver(ctx context.Context, h *Hub, s *Store, interval time.Duration) {
	flush := func() {
		text, version, dirty := h.Snapshot()
		if !dirty {
			return
		}
		if err := s.Save(text); err != nil {
			log.Printf("save failed: %v", err)
			return
		}
		h.MarkSaved(version)
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}
