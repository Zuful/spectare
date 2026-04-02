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

	"github.com/Zuful/spectare/internal/scanner"
	"github.com/Zuful/spectare/internal/store"
	"github.com/Zuful/spectare/internal/transcode"
)

type Server struct {
	store    *store.Store
	mediaDir string // optional: MEDIA_DIR env var
}

func New(embedded embed.FS, s *store.Store, mediaDir string) http.Handler {
	srv := &Server{store: s, mediaDir: mediaDir}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Route("/api", func(r chi.Router) {
		r.Get("/titles", srv.handleListTitles)
		r.Post("/titles", srv.handleUpload)
		r.Get("/titles/{id}", srv.handleGetTitle)
		r.Get("/titles/{id}/status", srv.handleTranscodeStatus)
		r.Post("/titles/{id}/transcode", srv.handleStartTranscode)
		r.Post("/scan", srv.handleScan)
		r.Get("/stream/{id}/*", srv.handleHLSFile)
		r.Get("/stream/{id}/direct", srv.handleDirectStream)
	})

	// Static frontend
	sub, err := fs.Sub(embedded, "frontend/out")
	if err != nil {
		r.Get("/*", placeholderHandler)
		r.Get("/", placeholderHandler)
		return r
	}
	// Verify the frontend was actually built (index.html must exist)
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		r.Get("/*", placeholderHandler)
		r.Get("/", placeholderHandler)
		return r
	}
	fileServer := http.FileServer(http.FS(sub))
	serve := func(w http.ResponseWriter, r *http.Request, path string) {
		r.URL.Path = "/" + path
		fileServer.ServeHTTP(w, r)
	}
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Normalise: strip leading slash, add trailing index.html for dirs
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || strings.HasSuffix(path, "/") {
			path = strings.TrimSuffix(path, "/")
			if path == "" {
				path = "index.html"
			} else {
				path = path + "/index.html"
			}
		}

		// Exact file exists → serve it directly
		if _, err := fs.Stat(sub, path); err == nil {
			r.URL.Path = "/" + path
			fileServer.ServeHTTP(w, r)
			return
		}

		// Dynamic route fallback: for /watch/{id} and /title/{id}, serve the
		// pre-generated template page (id "1") so the client-side router can
		// render the correct component using the real URL.
		parts := strings.SplitN(strings.TrimSuffix(path, "/index.html"), "/", 3)
		if len(parts) == 2 {
			template := parts[0] + "/1/index.html"
			if _, err := fs.Stat(sub, template); err == nil {
				serve(w, r, template)
				return
			}
		}

		// Final fallback: root index.html (SPA home)
		serve(w, r, "index.html")
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
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
		t, err := srv.store.Load(id)
		if err != nil {
			http.Error(w, "title not found", http.StatusNotFound)
			return
		}
		pct := 0.0
		if t.TranscodeStatus == store.StatusReady {
			pct = 100
		}
		jsonResponse(w, &store.Progress{Status: t.TranscodeStatus, Progress: pct})
		return
	}
	jsonResponse(w, p)
}

// POST /api/titles/{id}/transcode — start HLS transcoding on demand
func (srv *Server) handleStartTranscode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := srv.store.Load(id)
	if err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}
	if t.StreamReady {
		jsonResponse(w, map[string]string{"status": "already_ready"})
		return
	}
	if p := srv.store.GetProgress(id); p != nil && p.Status == store.StatusTranscoding {
		jsonResponse(w, map[string]string{"status": "already_transcoding"})
		return
	}

	// Determine input file
	inputPath := t.DirectPath
	if inputPath == "" {
		// Fall back to uploaded original
		origDir := srv.store.OriginalDir(id)
		entries, err := os.ReadDir(origDir)
		if err != nil || len(entries) == 0 {
			http.Error(w, "no source file found", http.StatusBadRequest)
			return
		}
		inputPath = filepath.Join(origDir, entries[0].Name())
	}

	transcode.Start(srv.store, id, inputPath)
	w.WriteHeader(http.StatusAccepted)
	jsonResponse(w, map[string]string{"status": "transcoding"})
}

// POST /api/scan — re-scan MEDIA_DIR
func (srv *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	dir := srv.mediaDir
	// Allow overriding the scan dir per-request
	if body := r.FormValue("dir"); body != "" {
		dir = body
	}
	if dir == "" {
		http.Error(w, "no MEDIA_DIR configured (set MEDIA_DIR env var or pass dir= in body)", http.StatusBadRequest)
		return
	}
	added, err := scanner.Scan(srv.store, dir)
	if err != nil {
		http.Error(w, "scan error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{"added": added, "dir": dir})
}

// POST /api/titles — multipart upload: file + metadata + optional transcode flag
func (srv *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
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

	yearStr := r.FormValue("year")
	year, _ := strconv.Atoi(yearStr)
	if year == 0 {
		year = time.Now().Year()
	}

	titleStr := r.FormValue("title")
	if titleStr == "" {
		titleStr = strings.TrimSuffix(fh.Filename, ext)
	}

	genres := splitTrim(r.FormValue("genre"), ",")
	if len(genres) == 0 {
		genres = []string{"Uncategorised"}
	}

	titleType := r.FormValue("type")
	if titleType != "movie" && titleType != "series" {
		titleType = "movie"
	}

	doTranscode := r.FormValue("transcode") == "true" || r.FormValue("transcode") == "1"

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
		DirectExt:       strings.ToLower(ext),
		CreatedAt:       time.Now(),
	}
	if err := srv.store.Save(t); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

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

	// Set direct path so the file can be played immediately
	t.DirectPath = origPath
	if err := srv.store.Save(t); err != nil {
		http.Error(w, "failed to update metadata", http.StatusInternalServerError)
		return
	}

	if doTranscode {
		transcode.Start(srv.store, id, origPath)
		w.WriteHeader(http.StatusAccepted)
		jsonResponse(w, map[string]string{"id": id, "status": "transcoding"})
	} else {
		w.WriteHeader(http.StatusCreated)
		jsonResponse(w, map[string]string{"id": id, "status": "ready"})
	}
}

// GET /api/stream/{id}/direct — serve source file with Range support
func (srv *Server) handleDirectStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := srv.store.Load(id)
	if err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}
	if !t.HasDirect() {
		http.Error(w, "no direct stream available", http.StatusNotFound)
		return
	}

	f, err := os.Open(t.DirectPath)
	if err != nil {
		http.Error(w, "source file not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "stat error", http.StatusInternalServerError)
		return
	}

	mime := scanner.MIMEType(t.DirectExt)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Accept-Ranges", "bytes")
	// Allow Cross-Origin for the video element (dev server)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	http.ServeContent(w, r, filepath.Base(t.DirectPath), fi.ModTime(), f)
}

// GET /api/stream/{id}/* — serve HLS files from disk
func (srv *Server) handleHLSFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rest := chi.URLParam(r, "*")

	if strings.Contains(rest, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// "direct" is handled by its own route, but just in case
	if rest == "direct" {
		srv.handleDirectStream(w, r)
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
