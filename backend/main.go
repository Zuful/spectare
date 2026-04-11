package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Zuful/spectare/internal/scanner"
	"github.com/Zuful/spectare/internal/server"
	"github.com/Zuful/spectare/internal/store"
	"github.com/Zuful/spectare/internal/tmdb"
)

//go:embed all:frontend/out
var frontendDist embed.FS

func main() {
	port := 8766
	if p := os.Getenv("PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	mediaDir := os.Getenv("MEDIA_DIR")

	s := store.New(dataDir)

	tmdbClient := tmdb.NewFromEnv()
	if tmdbClient != nil {
		log.Printf("TMDB metadata enrichment enabled")
	}

	// Auto-scan MEDIA_DIR on startup
	if mediaDir != "" {
		log.Printf("Scanning MEDIA_DIR: %s", mediaDir)
		n, err := scanner.Scan(s, mediaDir, tmdbClient)
		if err != nil {
			log.Printf("Warning: scan error: %v", err)
		} else {
			log.Printf("Scan complete: %d new title(s) added", n)
		}
	}

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Spectare listening on http://localhost%s  (data: %s)", addr, dataDir)
	if mediaDir != "" {
		log.Printf("Media dir: %s  (POST /api/scan to rescan)", mediaDir)
	}

	srv := server.New(frontendDist, s, mediaDir, tmdbClient)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
