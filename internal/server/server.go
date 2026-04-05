package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
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
	tmdbclient "github.com/Zuful/spectare/internal/tmdb"
	"github.com/Zuful/spectare/internal/transcode"
)

type Server struct {
	store      *store.Store
	mediaDir   string // optional: MEDIA_DIR env var
	tmdbClient *tmdbclient.Client
}

func New(embedded embed.FS, s *store.Store, mediaDir string, tmdb *tmdbclient.Client) http.Handler {
	srv := &Server{store: s, mediaDir: mediaDir, tmdbClient: tmdb}
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
		r.Get("/titles/{id}/subtitles", srv.handleListSubtitles)
		r.Post("/titles/{id}/subtitles", srv.handleUploadSubtitle)
		r.Get("/titles/{id}/subtitles/{file}", srv.handleServeSubtitle)
		r.Delete("/titles/{id}/subtitles/{file}", srv.handleDeleteSubtitle)
		r.Get("/titles/{id}/episodes", srv.handleListEpisodes)
		r.Post("/titles/{id}/episodes", srv.handleUploadEpisode)
		r.Get("/episodes/{id}", srv.handleGetEpisode)
		r.Put("/episodes/{id}", srv.handleUpdateEpisode)
		r.Delete("/episodes/{id}", srv.handleDeleteEpisode)
		r.Get("/episodes/{id}/thumbnail", srv.handleEpisodeThumbnail)
		r.Post("/episodes/{id}/thumbnail", srv.handleUploadEpisodeThumbnail)
		r.Get("/episodes/{id}/status", srv.handleEpisodeTranscodeStatus)
		r.Post("/episodes/{id}/transcode", srv.handleStartEpisodeTranscode)
		r.Get("/stream/episodes/{id}/*", srv.handleEpisodeHLSFile)
		r.Get("/stream/episodes/{id}/direct", srv.handleEpisodeDirectStream)
		r.Post("/titles/{id}/fetch-metadata", srv.handleFetchMetadata)
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
	// serveFile serves a single file from the embedded FS using http.ServeContent,
	// which does not perform any redirects (unlike http.FileServer).
	serveFile := func(w http.ResponseWriter, r *http.Request, path string) {
		f, err := sub.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		rs, ok := f.(io.ReadSeeker)
		if !ok {
			data, _ := io.ReadAll(f)
			rs = strings.NewReader(string(data))
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
	}

	// staticHandler serves pre-built Next.js pages.
	// We bypass http.FileServer entirely to avoid its auto-redirect behaviour
	// (e.g. /index.html → /, /browse/index.html → /browse/ → infinite loop).
	// Next.js App Router also sends POST for RSC prefetch — normalised to GET.
	staticHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			r.Method = http.MethodGet
		}

		path := strings.TrimPrefix(r.URL.Path, "/")

		// Directory-like paths → look for index.html inside
		if path == "" || strings.HasSuffix(path, "/") {
			base := strings.TrimSuffix(path, "/")
			if base == "" {
				path = "index.html"
			} else {
				path = base + "/index.html"
			}
		}

		// Exact file hit (static asset or pre-generated page)
		if info, err := fs.Stat(sub, path); err == nil {
			if info.IsDir() {
				// Directory entry: serve its index.html
				path = path + "/index.html"
				if _, err := fs.Stat(sub, path); err == nil {
					serveFile(w, r, path)
					return
				}
			} else {
				serveFile(w, r, path)
				return
			}
		}

		// Dynamic route fallback: serve the pre-generated template (id "1") for
		// any dynamic route whose real ID is a runtime hex string.
		// Handles: /watch/{id}, /title/{id}, /admin/titles/{id}/edit, etc.
		base := strings.TrimSuffix(path, "/index.html")
		parts := strings.Split(base, "/")
		for i := len(parts) - 1; i >= 1; i-- {
			candidate := strings.Join(append(parts[:i], append([]string{"1"}, parts[i+1:]...)...), "/") + "/index.html"
			if _, err := fs.Stat(sub, candidate); err == nil {
				serveFile(w, r, candidate)
				return
			}
		}

		// Final fallback: root index.html
		serveFile(w, r, "index.html")
	})

	r.Get("/*", staticHandler)
	r.Post("/*", staticHandler) // Next.js RSC prefetch
	r.Get("/", staticHandler)
	r.Post("/", staticHandler)

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
	added, err := scanner.Scan(srv.store, dir, srv.tmdbClient)
	if err != nil {
		http.Error(w, "scan error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{"added": added, "dir": dir})
}

// POST /api/titles/{id}/fetch-metadata — fetch missing metadata from TMDB
func (srv *Server) handleFetchMetadata(w http.ResponseWriter, r *http.Request) {
	if srv.tmdbClient == nil {
		http.Error(w, "TMDB_API_KEY not configured", http.StatusNotImplemented)
		return
	}
	id := chi.URLParam(r, "id")
	t, err := srv.store.Load(id)
	if err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}

	result, genres, err := srv.tmdbClient.FetchMetadata(t.Title, t.Type, t.Year)
	if err != nil {
		http.Error(w, "TMDB lookup failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	updated := false

	// Fill in missing synopsis
	if t.Synopsis == "" && result.Overview != "" {
		t.Synopsis = result.Overview
		updated = true
	}

	// Fill in missing year
	if t.Year == 0 && result.Year() > 0 {
		t.Year = result.Year()
		updated = true
	}

	// Fill in missing genres
	if len(t.Genre) == 0 || (len(t.Genre) == 1 && t.Genre[0] == "Uncategorised") {
		if len(genres) > 0 {
			t.Genre = genres
			updated = true
		}
	}

	// Fill in missing director
	if t.Director == "" {
		if dir := srv.tmdbClient.Director(result.ID, result.MediaType); dir != "" {
			t.Director = dir
			updated = true
		}
	}

	// Download missing thumbnails
	thumbDir := filepath.Join(srv.store.TitleDir(id), "thumbnails")
	os.MkdirAll(thumbDir, 0755)

	posterGlob, _ := filepath.Glob(filepath.Join(thumbDir, "poster.*"))
	if len(posterGlob) == 0 && result.PosterPath != "" {
		dest := filepath.Join(thumbDir, "poster.jpg")
		if err := srv.tmdbClient.DownloadImage(result.PosterPath, "w342", dest); err != nil {
			log.Printf("TMDB: poster download failed: %v", err)
		}
	}

	backdropGlob, _ := filepath.Glob(filepath.Join(thumbDir, "backdrop.*"))
	if len(backdropGlob) == 0 && result.BackdropPath != "" {
		dest := filepath.Join(thumbDir, "backdrop.jpg")
		if err := srv.tmdbClient.DownloadImage(result.BackdropPath, "w1280", dest); err != nil {
			log.Printf("TMDB: backdrop download failed: %v", err)
		}
	}

	// Also try card thumbnail from backdrop if card is missing
	cardGlob, _ := filepath.Glob(filepath.Join(thumbDir, "card.*"))
	if len(cardGlob) == 0 && result.BackdropPath != "" {
		dest := filepath.Join(thumbDir, "card.jpg")
		if err := srv.tmdbClient.DownloadImage(result.BackdropPath, "w780", dest); err != nil {
			log.Printf("TMDB: card download failed: %v", err)
		}
	}

	if updated {
		if err := srv.store.Save(t); err != nil {
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}
	}

	jsonResponse(w, map[string]any{
		"updated": updated,
		"title":   t.Title,
		"tmdbId":  result.ID,
		"genres":  genres,
	})
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
		srv.store.Delete(id)
		http.Error(w, "failed to create upload dir", http.StatusInternalServerError)
		return
	}
	origPath := filepath.Join(origDir, "video"+ext)
	dst, err := os.Create(origPath)
	if err != nil {
		srv.store.Delete(id)
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, f); err != nil {
		dst.Close()
		srv.store.Delete(id)
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

// ── subtitles ────────────────────────────────────────────────────────────────

type subtitleTrack struct {
	Lang  string `json:"lang"`
	Label string `json:"label"`
	File  string `json:"file"` // basename, e.g. "en.srt"
}

var langLabels = map[string]string{
	"en": "English", "fr": "Français", "es": "Español", "de": "Deutsch",
	"it": "Italiano", "pt": "Português", "nl": "Nederlands", "ru": "Русский",
	"ja": "日本語", "ko": "한국어", "zh": "中文", "ar": "العربية",
}

func subtitleDir(titleDir string) string { return filepath.Join(titleDir, "subtitles") }

// GET /api/titles/{id}/subtitles
func (srv *Server) handleListSubtitles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dir := subtitleDir(srv.store.TitleDir(id))
	entries, err := os.ReadDir(dir)
	tracks := []subtitleTrack{}
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".srt" && ext != ".vtt" {
				continue
			}
			lang := strings.TrimSuffix(name, ext)
			label, ok := langLabels[lang]
			if !ok {
				label = strings.ToUpper(lang)
			}
			tracks = append(tracks, subtitleTrack{Lang: lang, Label: label, File: name})
		}
	}
	jsonResponse(w, tracks)
}

// GET /api/titles/{id}/subtitles/{file} — serve SRT converted to WebVTT, or VTT as-is
func (srv *Server) handleServeSubtitle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	file := chi.URLParam(r, "file")
	if strings.Contains(file, "..") || strings.Contains(file, "/") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	dir := subtitleDir(srv.store.TitleDir(id))

	// Try the exact filename first, then swap .vtt → .srt (serve SRT as VTT)
	path := filepath.Join(dir, file)
	raw, err := os.ReadFile(path)
	isSRT := strings.HasSuffix(strings.ToLower(path), ".srt")
	if err != nil {
		alt := strings.TrimSuffix(path, filepath.Ext(path)) + ".srt"
		raw, err = os.ReadFile(alt)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		isSRT = true // fell back to reading the .srt companion file
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	if isSRT {
		w.Write([]byte(srtToVTT(string(raw))))
		return
	}
	w.Write(raw)
}

// POST /api/titles/{id}/subtitles — upload a subtitle file
// Form fields: file (SRT or VTT), lang (e.g. "en")
func (srv *Server) handleUploadSubtitle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := srv.store.Load(id); err != nil {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer f.Close()

	lang := strings.TrimSpace(r.FormValue("lang"))
	if lang == "" {
		lang = strings.ToLower(strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename)))
	}
	if lang == "" {
		lang = "und"
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".srt" && ext != ".vtt" {
		http.Error(w, "only .srt and .vtt files are accepted", http.StatusBadRequest)
		return
	}

	dir := subtitleDir(srv.store.TitleDir(id))
	os.MkdirAll(dir, 0755)
	dst, err := os.Create(filepath.Join(dir, lang+ext))
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, f); err != nil {
		dst.Close()
		http.Error(w, "failed to write subtitle", http.StatusInternalServerError)
		return
	}
	dst.Close()

	label, ok := langLabels[lang]
	if !ok {
		label = strings.ToUpper(lang)
	}
	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, subtitleTrack{Lang: lang, Label: label, File: lang + ext})
}

// DELETE /api/titles/{id}/subtitles/{file}
func (srv *Server) handleDeleteSubtitle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	file := chi.URLParam(r, "file")
	if strings.Contains(file, "..") || strings.Contains(file, "/") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	path := filepath.Join(subtitleDir(srv.store.TitleDir(id)), file)
	if err := os.Remove(path); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func srtToVTT(srt string) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, line := range strings.Split(srt, "\n") {
		// Convert SRT timestamps: 00:00:01,000 --> 00:00:02,000
		// to VTT timestamps:      00:00:01.000 --> 00:00:02.000
		if strings.Contains(line, " --> ") {
			line = strings.ReplaceAll(line, ",", ".")
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// ── Episode handlers ─────────────────────────────────────────────────────────

// GET /api/titles/{id}/episodes
func (srv *Server) handleListEpisodes(w http.ResponseWriter, r *http.Request) {
	seriesID := chi.URLParam(r, "id")
	episodes, err := srv.store.ListEpisodes(seriesID)
	if err != nil {
		http.Error(w, "failed to list episodes", http.StatusInternalServerError)
		return
	}
	if episodes == nil {
		episodes = []*store.Episode{}
	}
	jsonResponse(w, episodes)
}

// POST /api/titles/{id}/episodes
func (srv *Server) handleUploadEpisode(w http.ResponseWriter, r *http.Request) {
	seriesID := chi.URLParam(r, "id")
	if _, err := srv.store.Load(seriesID); err != nil {
		http.Error(w, "series not found", http.StatusNotFound)
		return
	}

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

	seasonStr := r.FormValue("season")
	numberStr := r.FormValue("number")
	if seasonStr == "" || numberStr == "" {
		http.Error(w, "season and number are required", http.StatusBadRequest)
		return
	}
	season, err := strconv.Atoi(seasonStr)
	if err != nil || season < 1 {
		http.Error(w, "invalid season", http.StatusBadRequest)
		return
	}
	number, err := strconv.Atoi(numberStr)
	if err != nil || number < 1 {
		http.Error(w, "invalid number", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext == "" {
		ext = ".mp4"
	}

	id := newID()
	titleStr := r.FormValue("title")
	if titleStr == "" {
		titleStr = strings.TrimSuffix(fh.Filename, ext)
	}

	doTranscode := r.FormValue("transcode") == "true" || r.FormValue("transcode") == "1"

	e := &store.Episode{
		ID:              id,
		SeriesID:        seriesID,
		Season:          season,
		Number:          number,
		Title:           titleStr,
		Synopsis:        r.FormValue("synopsis"),
		StreamReady:     false,
		TranscodeStatus: store.StatusPending,
		DirectExt:       ext,
		CreatedAt:       time.Now(),
	}
	if err := srv.store.SaveEpisode(e); err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	origDir := srv.store.EpisodeOriginalDir(id)
	if err := os.MkdirAll(origDir, 0755); err != nil {
		srv.store.DeleteEpisode(id)
		http.Error(w, "failed to create upload dir", http.StatusInternalServerError)
		return
	}
	origPath := filepath.Join(origDir, "video"+ext)
	dst, err := os.Create(origPath)
	if err != nil {
		srv.store.DeleteEpisode(id)
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, f); err != nil {
		dst.Close()
		srv.store.DeleteEpisode(id)
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	dst.Close()

	// Optional thumbnail
	if th, thHeader, err := r.FormFile("thumbnail"); err == nil {
		thExt := strings.ToLower(filepath.Ext(thHeader.Filename))
		if thExt == "" {
			thExt = ".jpg"
		}
		thumbDir := filepath.Join(srv.store.EpisodeDir(id), "thumbnails")
		if os.MkdirAll(thumbDir, 0755) == nil {
			if tdst, err := os.Create(filepath.Join(thumbDir, "card"+thExt)); err == nil {
				io.Copy(tdst, th)
				tdst.Close()
			}
		}
		th.Close()
	}

	e.DirectPath = origPath
	if err := srv.store.SaveEpisode(e); err != nil {
		http.Error(w, "failed to update metadata", http.StatusInternalServerError)
		return
	}

	if doTranscode {
		transcode.StartEpisode(srv.store, id, origPath)
		w.WriteHeader(http.StatusAccepted)
		jsonResponse(w, map[string]string{"id": id, "status": "transcoding"})
	} else {
		w.WriteHeader(http.StatusCreated)
		jsonResponse(w, map[string]string{"id": id, "status": "ready"})
	}
}

// GET /api/episodes/{id}
func (srv *Server) handleGetEpisode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := srv.store.LoadEpisode(id)
	if err != nil {
		http.Error(w, "episode not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, e)
}

// PUT /api/episodes/{id}
func (srv *Server) handleUpdateEpisode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := srv.store.LoadEpisode(id)
	if err != nil {
		http.Error(w, "episode not found", http.StatusNotFound)
		return
	}

	var body struct {
		Season   *int   `json:"season"`
		Number   *int   `json:"number"`
		Title    string `json:"title"`
		Synopsis string `json:"synopsis"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if body.Season != nil {
		e.Season = *body.Season
	}
	if body.Number != nil {
		e.Number = *body.Number
	}
	if body.Title != "" {
		e.Title = body.Title
	}
	if body.Synopsis != "" {
		e.Synopsis = body.Synopsis
	}

	if err := srv.store.SaveEpisode(e); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, e)
}

// DELETE /api/episodes/{id}
func (srv *Server) handleDeleteEpisode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := srv.store.DeleteEpisode(id); err != nil {
		http.Error(w, "episode not found", http.StatusNotFound)
		return
	}
	os.RemoveAll(srv.store.EpisodeDir(id))
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/episodes/{id}/thumbnail
func (srv *Server) handleEpisodeThumbnail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	thumbDir := filepath.Join(srv.store.EpisodeDir(id), "thumbnails")
	if serveImageFile(w, r, thumbDir, "card") {
		return
	}
	http.NotFound(w, r)
}

// POST /api/episodes/{id}/thumbnail
func (srv *Server) handleUploadEpisodeThumbnail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := srv.store.LoadEpisode(id); err != nil {
		http.Error(w, "episode not found", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	thumbDir := filepath.Join(srv.store.EpisodeDir(id), "thumbnails")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		http.Error(w, "failed to create thumbnail dir", http.StatusInternalServerError)
		return
	}
	dst, err := os.Create(filepath.Join(thumbDir, "card"+ext))
	if err != nil {
		http.Error(w, "failed to write thumbnail", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	io.Copy(dst, f)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/episodes/{id}/status
func (srv *Server) handleEpisodeTranscodeStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := srv.store.GetEpisodeProgress(id)
	if p == nil {
		e, err := srv.store.LoadEpisode(id)
		if err != nil {
			http.Error(w, "episode not found", http.StatusNotFound)
			return
		}
		pct := 0.0
		if e.TranscodeStatus == store.StatusReady {
			pct = 100
		}
		jsonResponse(w, &store.Progress{Status: e.TranscodeStatus, Progress: pct})
		return
	}
	jsonResponse(w, p)
}

// POST /api/episodes/{id}/transcode
func (srv *Server) handleStartEpisodeTranscode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := srv.store.LoadEpisode(id)
	if err != nil {
		http.Error(w, "episode not found", http.StatusNotFound)
		return
	}
	if e.StreamReady {
		jsonResponse(w, map[string]string{"status": "already_ready"})
		return
	}
	if p := srv.store.GetEpisodeProgress(id); p != nil && p.Status == store.StatusTranscoding {
		jsonResponse(w, map[string]string{"status": "already_transcoding"})
		return
	}

	inputPath := e.DirectPath
	if inputPath == "" {
		origDir := srv.store.EpisodeOriginalDir(id)
		entries, err := os.ReadDir(origDir)
		if err != nil || len(entries) == 0 {
			http.Error(w, "no source file found", http.StatusBadRequest)
			return
		}
		inputPath = filepath.Join(origDir, entries[0].Name())
	}

	transcode.StartEpisode(srv.store, id, inputPath)
	w.WriteHeader(http.StatusAccepted)
	jsonResponse(w, map[string]string{"status": "transcoding"})
}

// GET /api/stream/episodes/{id}/* — serve episode HLS files
func (srv *Server) handleEpisodeHLSFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rest := chi.URLParam(r, "*")

	if strings.Contains(rest, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if rest == "direct" {
		srv.handleEpisodeDirectStream(w, r)
		return
	}

	filePath := filepath.Join(srv.store.EpisodeHLSDir(id), rest)
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

// GET /api/stream/episodes/{id}/direct — serve episode source file
func (srv *Server) handleEpisodeDirectStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := srv.store.LoadEpisode(id)
	if err != nil {
		http.Error(w, "episode not found", http.StatusNotFound)
		return
	}
	if !e.HasDirect() {
		http.Error(w, "no direct stream available", http.StatusNotFound)
		return
	}

	f, err := os.Open(e.DirectPath)
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

	mime := scanner.MIMEType(e.DirectExt)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeContent(w, r, filepath.Base(e.DirectPath), fi.ModTime(), f)
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
