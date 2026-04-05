package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
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
	Type            string          `json:"type"`
	Rating          string          `json:"rating"`
	Synopsis        string          `json:"synopsis"`
	Director        string          `json:"director"`
	Cast            []string        `json:"cast"`
	StreamReady     bool            `json:"streamReady"`
	TranscodeStatus TranscodeStatus `json:"transcodeStatus"`
	DirectPath      string          `json:"directPath,omitempty"`
	DirectExt       string          `json:"directExt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

func (t *Title) HasDirect() bool { return t.DirectPath != "" }

type Progress struct {
	Status   TranscodeStatus `json:"status"`
	Progress float64         `json:"progress"`
	Error    string          `json:"error,omitempty"`
}

var bucketTitles = []byte("titles")

type Store struct {
	mu       sync.RWMutex
	dataDir  string
	db       *bolt.DB
	progress map[string]*Progress
}

func New(dataDir string) *Store {
	os.MkdirAll(filepath.Join(dataDir, "titles"), 0755)

	db, err := bolt.Open(filepath.Join(dataDir, "spectare.db"), 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Fatalf("store: open db: %v", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketTitles)
		return err
	}); err != nil {
		log.Fatalf("store: create bucket: %v", err)
	}

	s := &Store{
		dataDir:  dataDir,
		db:       db,
		progress: make(map[string]*Progress),
	}
	s.importLegacyJSON()
	return s
}

// importLegacyJSON reads any existing meta.json files and inserts them into
// the DB (skipping IDs that are already present). Runs once on startup.
func (s *Store) importLegacyJSON() {
	dir := filepath.Join(s.dataDir, "titles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	imported := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip if already in DB
		var exists bool
		s.db.View(func(tx *bolt.Tx) error {
			exists = tx.Bucket(bucketTitles).Get([]byte(e.Name())) != nil
			return nil
		})
		if exists {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var t Title
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		if err := s.Save(&t); err == nil {
			imported++
		}
	}
	if imported > 0 {
		log.Printf("store: migrated %d title(s) from JSON to bbolt", imported)
	}
}

func (s *Store) DataDir() string              { return s.dataDir }
func (s *Store) TitleDir(id string) string    { return filepath.Join(s.dataDir, "titles", id) }
func (s *Store) HLSDir(id string) string      { return filepath.Join(s.TitleDir(id), "hls") }
func (s *Store) OriginalDir(id string) string { return filepath.Join(s.TitleDir(id), "original") }

func (s *Store) Delete(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("titles")).Delete([]byte(id))
	})
}

func (s *Store) Save(t *Title) error {
	if err := os.MkdirAll(s.TitleDir(t.ID), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketTitles).Put([]byte(t.ID), data)
	})
}

func (s *Store) Load(id string) (*Title, error) {
	var t Title
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketTitles).Get([]byte(id))
		if v == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(v, &t)
	})
	if err != nil {
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

func (s *Store) List() ([]*Title, error) {
	var titles []*Title
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketTitles).ForEach(func(_, v []byte) error {
			var t Title
			if err := json.Unmarshal(v, &t); err != nil {
				return nil // skip malformed entries
			}
			titles = append(titles, &t)
			return nil
		})
	})
	if err != nil {
		return []*Title{}, err
	}

	// Overlay in-memory progress
	s.mu.RLock()
	for _, t := range titles {
		if p, ok := s.progress[t.ID]; ok {
			t.TranscodeStatus = p.Status
			t.StreamReady = p.Status == StatusReady
		}
	}
	s.mu.RUnlock()

	// Newest first
	for i, j := 0, len(titles)-1; i < j; i, j = i+1, j-1 {
		titles[i], titles[j] = titles[j], titles[i]
	}
	return titles, nil
}

func (s *Store) SetProgress(id string, p *Progress) {
	s.mu.Lock()
	s.progress[id] = p
	s.mu.Unlock()

	if p.Status != StatusReady && p.Status != StatusError {
		return
	}
	// Persist terminal state to DB
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTitles)
		v := b.Get([]byte(id))
		if v == nil {
			return nil
		}
		var t Title
		if err := json.Unmarshal(v, &t); err != nil {
			return err
		}
		t.TranscodeStatus = p.Status
		t.StreamReady = p.Status == StatusReady
		data, err := json.Marshal(&t)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	}); err != nil {
		log.Printf("store: SetProgress persist: %v", err)
	}
}

func (s *Store) GetProgress(id string) *Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.progress[id]; ok {
		cp := *p
		return &cp
	}
	return nil
}
