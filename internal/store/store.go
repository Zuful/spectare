package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TranscodeStatus string

const (
	StatusPending     TranscodeStatus = "pending"
	StatusTranscoding TranscodeStatus = "transcoding"
	StatusReady       TranscodeStatus = "ready"
	StatusError       TranscodeStatus = "error"
)

type Title struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Year            int             `json:"year"`
	Genre           []string        `json:"genre"`
	Type            string          `json:"type"`   // "movie" | "series"
	Rating          string          `json:"rating"` // "PG-13", "TV-MA", etc.
	Synopsis        string          `json:"synopsis"`
	Director        string          `json:"director"`
	Cast            []string        `json:"cast"`
	StreamReady     bool            `json:"streamReady"`
	TranscodeStatus TranscodeStatus `json:"transcodeStatus"`
	// DirectPath is the absolute path to the source video file (set when imported from MEDIA_DIR).
	// The file is served directly without transcoding via GET /api/stream/{id}/direct.
	DirectPath string `json:"directPath,omitempty"`
	// DirectExt is the lowercase file extension (e.g. ".mp4") used for MIME type detection.
	DirectExt string `json:"directExt,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// HasDirect reports whether this title can be played directly without HLS transcoding.
func (t *Title) HasDirect() bool { return t.DirectPath != "" }

// Progress is the in-memory transcoding progress for a title.
type Progress struct {
	Status   TranscodeStatus `json:"status"`
	Progress float64         `json:"progress"` // 0–100
	Error    string          `json:"error,omitempty"`
}

// Store manages title metadata on disk and transcoding progress in memory.
type Store struct {
	mu       sync.RWMutex
	dataDir  string
	progress map[string]*Progress
}

func New(dataDir string) *Store {
	os.MkdirAll(filepath.Join(dataDir, "titles"), 0755)
	return &Store{
		dataDir:  dataDir,
		progress: make(map[string]*Progress),
	}
}

func (s *Store) DataDir() string                  { return s.dataDir }
func (s *Store) TitleDir(id string) string        { return filepath.Join(s.dataDir, "titles", id) }
func (s *Store) HLSDir(id string) string          { return filepath.Join(s.TitleDir(id), "hls") }
func (s *Store) OriginalDir(id string) string     { return filepath.Join(s.TitleDir(id), "original") }

// Save persists a title's metadata to disk.
func (s *Store) Save(t *Title) error {
	dir := s.TitleDir(t.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), data, 0644)
}

// Load reads a title from disk, overlaying any in-memory progress.
func (s *Store) Load(id string) (*Title, error) {
	data, err := os.ReadFile(filepath.Join(s.TitleDir(id), "meta.json"))
	if err != nil {
		return nil, err
	}
	var t Title
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	s.mu.RLock()
	if p, ok := s.progress[id]; ok {
		t.TranscodeStatus = p.Status
		t.StreamReady = p.Status == StatusReady
	}
	s.mu.RUnlock()
	return &t, nil
}

// List returns all titles sorted by creation time (newest first).
func (s *Store) List() ([]*Title, error) {
	dir := filepath.Join(s.dataDir, "titles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []*Title{}, nil
	}
	var titles []*Title
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := s.Load(e.Name())
		if err != nil {
			continue
		}
		titles = append(titles, t)
	}
	// Newest first
	for i, j := 0, len(titles)-1; i < j; i, j = i+1, j-1 {
		titles[i], titles[j] = titles[j], titles[i]
	}
	return titles, nil
}

// SetProgress updates in-memory progress and persists terminal states to disk.
func (s *Store) SetProgress(id string, p *Progress) {
	s.mu.Lock()
	s.progress[id] = p
	s.mu.Unlock()

	if p.Status != StatusReady && p.Status != StatusError {
		return
	}
	// Persist terminal state to meta.json without calling Load() (avoids lock inversion)
	metaPath := filepath.Join(s.TitleDir(id), "meta.json")
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return
	}
	var t Title
	if err := json.Unmarshal(raw, &t); err != nil {
		return
	}
	t.TranscodeStatus = p.Status
	t.StreamReady = p.Status == StatusReady
	if data, err := json.MarshalIndent(t, "", "  "); err == nil {
		os.WriteFile(metaPath, data, 0644)
	}
}

// GetProgress returns the current transcoding progress for a title.
func (s *Store) GetProgress(id string) *Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.progress[id]; ok {
		cp := *p
		return &cp
	}
	return nil
}
