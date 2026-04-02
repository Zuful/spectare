package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

type Store struct {
	mu       sync.RWMutex
	dataDir  string
	db       *sql.DB
	progress map[string]*Progress
}

func New(dataDir string) *Store {
	os.MkdirAll(filepath.Join(dataDir, "titles"), 0755)

	dbPath := filepath.Join(dataDir, "spectare.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		log.Fatalf("store: open db: %v", err)
	}

	s := &Store{
		dataDir:  dataDir,
		db:       db,
		progress: make(map[string]*Progress),
	}
	s.migrate()
	s.importLegacyJSON()
	return s
}

func (s *Store) migrate() {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS titles (
		id               TEXT PRIMARY KEY,
		title            TEXT NOT NULL,
		year             INTEGER NOT NULL DEFAULT 0,
		genre            TEXT NOT NULL DEFAULT '[]',
		type             TEXT NOT NULL DEFAULT 'movie',
		rating           TEXT NOT NULL DEFAULT '',
		synopsis         TEXT NOT NULL DEFAULT '',
		director         TEXT NOT NULL DEFAULT '',
		cast             TEXT NOT NULL DEFAULT '[]',
		stream_ready     INTEGER NOT NULL DEFAULT 0,
		transcode_status TEXT NOT NULL DEFAULT 'pending',
		direct_path      TEXT NOT NULL DEFAULT '',
		direct_ext       TEXT NOT NULL DEFAULT '',
		created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatalf("store: migrate: %v", err)
	}
}

// importLegacyJSON reads any existing meta.json files and inserts them into
// SQLite (skipping IDs that are already present). This runs once on startup.
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
		metaPath := filepath.Join(dir, e.Name(), "meta.json")
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var t Title
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		// Skip if already in DB
		var exists int
		s.db.QueryRow(`SELECT COUNT(*) FROM titles WHERE id = ?`, t.ID).Scan(&exists)
		if exists > 0 {
			continue
		}
		if err := s.Save(&t); err == nil {
			imported++
		}
	}
	if imported > 0 {
		log.Printf("store: migrated %d title(s) from JSON to SQLite", imported)
	}
}

func (s *Store) DataDir() string              { return s.dataDir }
func (s *Store) TitleDir(id string) string    { return filepath.Join(s.dataDir, "titles", id) }
func (s *Store) HLSDir(id string) string      { return filepath.Join(s.TitleDir(id), "hls") }
func (s *Store) OriginalDir(id string) string { return filepath.Join(s.TitleDir(id), "original") }

func (s *Store) Save(t *Title) error {
	// Still create the title directory (needed for HLS / original files)
	if err := os.MkdirAll(s.TitleDir(t.ID), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	genre, _ := json.Marshal(t.Genre)
	cast, _ := json.Marshal(t.Cast)

	_, err := s.db.Exec(`INSERT INTO titles
		(id, title, year, genre, type, rating, synopsis, director, cast,
		 stream_ready, transcode_status, direct_path, direct_ext, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  title=excluded.title, year=excluded.year, genre=excluded.genre,
		  type=excluded.type, rating=excluded.rating, synopsis=excluded.synopsis,
		  director=excluded.director, cast=excluded.cast,
		  stream_ready=excluded.stream_ready, transcode_status=excluded.transcode_status,
		  direct_path=excluded.direct_path, direct_ext=excluded.direct_ext,
		  created_at=excluded.created_at`,
		t.ID, t.Title, t.Year, string(genre), t.Type, t.Rating, t.Synopsis,
		t.Director, string(cast), boolToInt(t.StreamReady), string(t.TranscodeStatus),
		t.DirectPath, t.DirectExt, t.CreatedAt.UTC(),
	)
	return err
}

func (s *Store) Load(id string) (*Title, error) {
	row := s.db.QueryRow(`SELECT
		id, title, year, genre, type, rating, synopsis, director, cast,
		stream_ready, transcode_status, direct_path, direct_ext, created_at
		FROM titles WHERE id = ?`, id)

	t, err := scanTitle(row)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	if p, ok := s.progress[id]; ok {
		t.TranscodeStatus = p.Status
		t.StreamReady = p.Status == StatusReady
	}
	s.mu.RUnlock()

	return t, nil
}

func (s *Store) List() ([]*Title, error) {
	rows, err := s.db.Query(`SELECT
		id, title, year, genre, type, rating, synopsis, director, cast,
		stream_ready, transcode_status, direct_path, direct_ext, created_at
		FROM titles ORDER BY created_at DESC`)
	if err != nil {
		return []*Title{}, nil
	}
	defer rows.Close()

	var titles []*Title
	for rows.Next() {
		t, err := scanTitle(rows)
		if err != nil {
			continue
		}
		s.mu.RLock()
		if p, ok := s.progress[t.ID]; ok {
			t.TranscodeStatus = p.Status
			t.StreamReady = p.Status == StatusReady
		}
		s.mu.RUnlock()
		titles = append(titles, t)
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
	_, err := s.db.Exec(
		`UPDATE titles SET stream_ready=?, transcode_status=? WHERE id=?`,
		boolToInt(p.Status == StatusReady), string(p.Status), id,
	)
	if err != nil {
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

// ── helpers ───────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanTitle(row scanner) (*Title, error) {
	var (
		t         Title
		genreJSON string
		castJSON  string
		streamInt int
		createdAt string
	)
	err := row.Scan(
		&t.ID, &t.Title, &t.Year, &genreJSON, &t.Type, &t.Rating,
		&t.Synopsis, &t.Director, &castJSON,
		&streamInt, &t.TranscodeStatus, &t.DirectPath, &t.DirectExt, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	t.StreamReady = streamInt != 0
	json.Unmarshal([]byte(genreJSON), &t.Genre)
	json.Unmarshal([]byte(castJSON), &t.Cast)
	if t.Genre == nil {
		t.Genre = []string{}
	}
	if t.Cast == nil {
		t.Cast = []string{}
	}
	// Parse SQLite datetime (stored as UTC)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
		if ts, err := time.Parse(layout, strings.TrimSuffix(createdAt, " +0000 UTC")); err == nil {
			t.CreatedAt = ts.UTC()
			break
		}
	}
	return &t, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
