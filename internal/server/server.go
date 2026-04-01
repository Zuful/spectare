package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Zuful/spectare/internal/store"
	"github.com/Zuful/spectare/internal/transcode"
)

type Server struct {
	store *store.Store
}

func New(embedded embed.FS, s *store.Store) http.Handler {
	srv := &Server{store: s}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Route("/api", func(r chi.Router) {
		r.Get("/titles", srv.handleListTitles)
		r.Post("/titles", srv.handleUpload)
		r.Get("/titles/{id}", srv.handleGetTitle)
		r.Get("/titles/{id}/status", srv.handleTranscodeStatus)
		r.Get("/stream/{id}/*", srv.handleHLSFile)
	})

	// Static frontend
	sub, err := fs.Sub(embedded, "frontend/out")
	if err != nil {
		r.Get("/*", placeholderHandler)
		return r
	}
	fileServer := http.FileServer(http.FS(sub))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	return r
}

// GET /api/titles
func (srv *Server) handleListTitles(w http.ResponseWriter, r *http.Request) {
	titles, err := srv.store.List()
	if err != nil {
		http.Error(w, "failed to list titles", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, titles)
}

// GET /api/titles/{id}
func (srv *Server) handleGetTitle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := srv.store.Load(id)
	if err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, t)
}

// GET /api/titles/{id}/status
func (srv *Server) handleTranscodeStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := srv.store.GetProgress(id)
	if p == nil {
		// Not in memory — check disk
		t, err := srv.store.Load(id)
		if err != nil {
			http.Error(w, "title not found", http.StatusNotFound)
			return
		}
		jsonResponse(w, &store.Progress{
			Status:   t.TranscodeStatus,
			Progress: map[store.TranscodeStatus]float64{store.StatusReady: 100}[t.TranscodeStatus],
		})
		return
	}
	jsonResponse(w, p)
}

// POST /api/titles — multipart upload: file + metadata fields
func (srv *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// 8 GB max
	r.Body = http.MaxBytesReader(w, r.Body, 8<<30)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer f.Close()

	id := newID()
	ext := filepath.Ext(fh.Filename)
	if ext == "" {
		ext = ".mp4"
	}

	// Parse metadata
	yearStr := r.FormValue("year")
	year, _ := strconv.Atoi(yearStr)
	if year == 0 {
		year = time.Now().Year()
	}

	titleStr := r.FormValue("title")
	if titleStr == "" {
		titleStr = strings.TrimSuffix(fh.Filename, ext)
	}

	genreStr := r.FormValue("genre")
	var genres []string
	for _, g := range strings.Split(genreStr, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			genres = append(genres, g)
		}
	}
	if len(genres) == 0 {
		genres = []string{"Uncategorised"}
	}

	titleType := r.FormValue("type")
	if titleType != "movie" && titleType != "series" {
		titleType = "movie"
	}

	t := &store.Title{
		ID:              id,
		Title:           titleStr,
		Year:            year,
		Genre:           genres,
		Type:            titleType,
		Rating:          r.FormValue("rating"),
		Synopsis:        r.FormValue("synopsis"),
		Director:        r.FormValue("director"),
		Cast:            splitTrim(r.FormValue("cast"), ","),
		StreamReady:     false,
		TranscodeStatus: store.StatusPending,
		CreatedAt:       time.Now(),
	}

	// Save metadata first so the title appears in the catalogue immediately
	if err := srv.store.Save(t); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	// Write original file to disk
	origDir := srv.store.OriginalDir(id)
	if err := os.MkdirAll(origDir, 0755); err != nil {
		http.Error(w, "failed to create upload dir", http.StatusInternalServerError)
		return
	}
	origPath := filepath.Join(origDir, "video"+ext)
	dst, err := os.Create(origPath)
	if err != nil {
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, f); err != nil {
		dst.Close()
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	dst.Close()

	// Start async transcoding
	transcode.Start(srv.store, id, origPath)

	w.WriteHeader(http.StatusAccepted)
	jsonResponse(w, map[string]string{"id": id, "status": "transcoding"})
}

// GET /api/stream/{id}/* — serve HLS master playlist and segments from disk
func (srv *Server) handleHLSFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rest := chi.URLParam(r, "*")

	// Prevent path traversal
	if strings.Contains(rest, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(srv.store.HLSDir(id), rest)

	switch {
	case strings.HasSuffix(rest, ".m3u8"):
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasSuffix(rest, ".ts"):
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}

	http.ServeFile(w, r, filePath)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func splitTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func placeholderHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Spectare</title>
<style>body{background:#0d0d0d;color:#e5e2e1;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.card{text-align:center;padding:2rem}h1{color:#87a96b;font-size:2.5rem;margin-bottom:.5rem}code{background:#1a1a1a;padding:.2em .5em;border-radius:4px}</style>
</head>
<body><div class="card">
<h1>Spectare</h1>
<p>Frontend not built yet.</p>
<p>Run <code>make build</code> to get started.</p>
<p>API is live at <code>/api/</code></p>
</div></body></html>`)
}
