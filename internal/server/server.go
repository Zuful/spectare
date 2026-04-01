package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func New(embedded embed.FS) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/titles", handleListTitles)
		r.Get("/titles/{id}", handleGetTitle)
		r.Get("/stream/{id}/master.m3u8", handleStream)
	})

	// Static frontend
	sub, err := fs.Sub(embedded, "frontend/out")
	if err != nil {
		// Frontend not built yet — serve placeholder
		r.Get("/*", placeholderHandler)
		return r
	}
	fileServer := http.FileServer(http.FS(sub))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Next.js static export: serve index.html for unknown paths (client-side routing)
		if _, err := fs.Stat(sub, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	return r
}

// handleListTitles returns the catalogue of available titles.
// GET /api/titles
func handleListTitles(w http.ResponseWriter, r *http.Request) {
	// TODO: replace with real DB query
	titles := []map[string]any{
		{"id": "1", "title": "The Last Kingdom", "year": 2015, "genre": []string{"Drama", "History"}, "type": "series", "rating": "TV-MA", "poster": ""},
		{"id": "2", "title": "Inception", "year": 2010, "genre": []string{"Sci-Fi", "Thriller"}, "type": "movie", "rating": "PG-13", "poster": ""},
		{"id": "3", "title": "The Matrix", "year": 1999, "genre": []string{"Sci-Fi", "Action"}, "type": "movie", "rating": "R", "poster": ""},
	}
	jsonResponse(w, titles)
}

// handleGetTitle returns metadata for a single title.
// GET /api/titles/{id}
func handleGetTitle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// TODO: replace with real DB lookup
	title := map[string]any{
		"id":          id,
		"title":       "The Last Kingdom",
		"year":        2015,
		"genre":       []string{"Drama", "History"},
		"type":        "series",
		"rating":      "TV-MA",
		"synopsis":    "A displaced nobleman fights to reclaim his home in 9th century England.",
		"director":    "Various",
		"cast":        []string{"Alexander Dreymon", "Emily Cox", "David Dawson"},
		"seasons":     5,
		"streamReady": false,
	}
	jsonResponse(w, title)
}

// handleStream serves the HLS master playlist for a title.
// GET /api/stream/{id}/master.m3u8
func handleStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// TODO: transcode with ffmpeg and serve real HLS segments
	http.Error(w, fmt.Sprintf("stream for %s not yet available", id), http.StatusNotImplemented)
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
