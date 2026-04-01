package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Zuful/spectare/internal/server"
	"github.com/Zuful/spectare/internal/store"
)

//go:embed frontend/out
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

	s := store.New(dataDir)
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Spectare listening on http://localhost%s  (data: %s)", addr, dataDir)

	srv := server.New(frontendDist, s)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
