package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Zuful/spectare/internal/server"
)

//go:embed frontend/out
var frontendDist embed.FS

func main() {
	port := 8766
	if p := os.Getenv("PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Spectare listening on http://localhost%s", addr)

	srv := server.New(frontendDist)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
