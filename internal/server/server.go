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
		r.Put("/titles/{id}", srv.handleUpdateTitle)
		r.Get("/titles/{id}/status", srv.handleTranscodeStatus)
		r.Post("/titles/{id}/transcode", srv.handleStartTranscode)
		r.Get("/titles/{id}/thumbnail", srv.handleThumbnail)          // backward compat → card
		r.Get("/titles/{id}/thumbnail/{variant}", srv.handleThumbnail) // card | poster | backdrop
		r.Get("/titles/{id}/preview", srv.handlePreview)
		r.Post("/titles/{id}/preview", srv.handleUploadPreview)
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

		// Dynamic route fallback: serve the pre-generated template (id "1") for
		// any dynamic route whose ID is a runtime hex string rather than a
		// static integer. Walk up the path replacing the dynamic segment with "1".
		// Handles: /watch/{id}, /title/{id}, /admin/titles/{id}/edit, etc.
		base := strings.TrimSuffix(path, "/index.html")
		parts := strings.Split(base, "/")
		for i := len(parts) - 1; i >= 1; i-- {
			candidate := strings.Join(append(parts[:i], append([]string{"1"}, parts[i+1:]...)...), "/") + "/index.html"
			if _, err := fs.Stat(sub, candidate); err == nil {
				serve(w, r, candidate)
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

// PUT /api/titles/{id} — update metadata and optionally replace thumbnails
func (srv *Server) handleUpdateTitle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := srv.store.Load(id)
	if err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	if v := r.FormValue("title"); v != "" {
		t.Title = v
	}
	if v := r.FormValue("year"); v != "" {
		if y, err := strconv.Atoi(v); err == nil && y > 0 {
			t.Year = y
		}
	}
	if v := r.FormValue("type"); v == "movie" || v == "series" {
		t.Type = v
	}
	if v := r.FormValue("genre"); v != "" {
		t.Genre = splitTrim(v, ",")
	}
	if v := r.FormValue("rating"); r.Form.Has("rating") {
		t.Rating = v
	}
	if v := r.FormValue("synopsis"); r.Form.Has("synopsis") {
		t.Synopsis = v
	}
	if v := r.FormValue("director"); r.Form.Has("director") {
		t.Director = v
	}
	if v := r.FormValue("cast"); r.Form.Has("cast") {
		t.Cast = splitTrim(v, ",")
	}

	// Optional thumbnail replacements
	thumbDir := filepath.Join(srv.store.TitleDir(id), "thumbnails")
	os.MkdirAll(thumbDir, 0755)
	for _, variant := range []string{"card", "poster", "backdrop"} {
		th, thHeader, err := r.FormFile(variant)
		if err != nil {
			continue
		}
		thExt := strings.ToLower(filepath.Ext(thHeader.Filename))
		if thExt == "" {
			thExt = ".jpg"
		}
		// Remove old variants of this slot before writing new one
		for _, oldExt := range []string{".jpg", ".jpeg", ".png", ".webp", ".avif"} {
			os.Remove(filepath.Join(thumbDir, variant+oldExt))
		}
		if dst, err := os.Create(filepath.Join(thumbDir, variant+thExt)); err == nil {
			io.Copy(dst, th)
			dst.Close()
		}
		th.Close()
	}

	if err := srv.store.Save(t); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
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

	// Save thumbnail variants: card (16:9), poster (2:3), backdrop (wide)
	thumbDir := filepath.Join(srv.store.TitleDir(id), "thumbnails")
	os.MkdirAll(thumbDir, 0755)
	for _, variant := range []string{"card", "poster", "backdrop"} {
		th, thHeader, err := r.FormFile(variant)
		if err != nil {
			// Also accept legacy "thumbnail" field as card
			if variant == "card" {
				th, thHeader, err = r.FormFile("thumbnail")
			}
			if err != nil {
				continue
			}
		}
		thExt := strings.ToLower(filepath.Ext(thHeader.Filename))
		if thExt == "" {
			thExt = ".jpg"
		}
		if dst, err := os.Create(filepath.Join(thumbDir, variant+thExt)); err == nil {
			io.Copy(dst, th)
			dst.Close()
		}
		th.Close()
	}

	// Set direct path so the file can be played immediately
	t.DirectPath = origPath
	if err := srv.store.Save(t); err != nil {
		http.Error(w, "failed to update metadata", http.StatusInternalServerError)
		return
	}

	// Auto-generate a 30s preview clip (async, does not block the response)
	transcode.GeneratePreview(srv.store.TitleDir(id), origPath)

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

// GET /api/titles/{id}/thumbnail[/{variant}]
// variant: card (16:9), poster (2:3), backdrop (wide). Defaults to "card".
// Falls back: poster→card, backdrop→card, card→legacy thumbnail.*
func (srv *Server) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	variant := chi.URLParam(r, "variant")
	if variant == "" {
		variant = "card"
	}
	switch variant {
	case "card", "poster", "backdrop":
	default:
		http.NotFound(w, r)
		return
	}

	titleDir := srv.store.TitleDir(id)
	// Try thumbnails/{variant}.* first
	thumbDir := filepath.Join(titleDir, "thumbnails")
	if serveImageFile(w, r, thumbDir, variant) {
		return
	}
	// Fallback: non-card variants fall back to card
	if variant != "card" && serveImageFile(w, r, thumbDir, "card") {
		return
	}
	// Legacy: single thumbnail.* at title root (before multi-variant support)
	if serveImageFile(w, r, titleDir, "thumbnail") {
		return
	}
	http.NotFound(w, r)
}

func serveImageFile(w http.ResponseWriter, r *http.Request, dir, base string) bool {
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".png": "image/png", ".webp": "image/webp", ".avif": "image/avif",
	}
	for ext, mime := range mimeTypes {
		f, err := os.Open(filepath.Join(dir, base+ext))
		if err != nil {
			continue
		}
		defer f.Close()
		fi, _ := f.Stat()
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
		return true
	}
	return false
}

// GET /api/titles/{id}/preview — serve the short preview clip
func (srv *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := filepath.Join(srv.store.TitleDir(id), "preview.mp4")
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(w, r, "preview.mp4", fi.ModTime(), f)
}

// POST /api/titles/{id}/preview — replace the auto-generated preview with a manual upload
func (srv *Server) handleUploadPreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := srv.store.Load(id); err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)
	f, _, err := r.FormFile("preview")
	if err != nil {
		http.Error(w, "missing preview field", http.StatusBadRequest)
		return
	}
	defer f.Close()
	dst, err := os.Create(filepath.Join(srv.store.TitleDir(id), "preview.mp4"))
	if err != nil {
		http.Error(w, "failed to write preview", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	io.Copy(dst, f)
	w.WriteHeader(http.StatusNoContent)
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
