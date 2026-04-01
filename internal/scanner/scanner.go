// Package scanner discovers video files in a directory and registers them in the store.
package scanner

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Zuful/spectare/internal/store"
)

// VideoExts lists supported video file extensions.
var VideoExts = map[string]bool{
	".mp4": true, ".m4v": true, ".mkv": true, ".avi": true,
	".mov": true, ".webm": true, ".flv": true, ".wmv": true,
	".ts": true, ".mpg": true, ".mpeg": true,
}

// MIMEType returns the Content-Type for a video extension.
func MIMEType(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".flv":
		return "video/x-flv"
	case ".wmv":
		return "video/x-ms-wmv"
	default:
		return "video/mpeg"
	}
}

var yearRe = regexp.MustCompile(`[\(\[\s\.](\d{4})[\)\]\s\.]`)

// ParseFilename extracts a human-readable title and year from a video filename.
//
//	"Inception (2010).mp4"          → "Inception", 2010
//	"The.Matrix.1999.1080p.mkv"     → "The Matrix", 1999
//	"my_movie.mp4"                  → "my movie", 0
func ParseFilename(name string) (title string, year int) {
	base := strings.TrimSuffix(name, filepath.Ext(name))

	// Pad with spaces so yearRe boundary anchors work at the edges
	padded := " " + base + " "
	if m := yearRe.FindStringSubmatch(padded); m != nil {
		year, _ = strconv.Atoi(m[1])
		padded = yearRe.ReplaceAllString(padded, " ")
	}
	base = strings.TrimSpace(padded)

	// Replace common separators with spaces
	base = strings.NewReplacer(".", " ", "_", " ").Replace(base)

	// Drop well-known quality/codec tags (stop at first one found)
	qualityTags := map[string]bool{
		"1080P": true, "720P": true, "480P": true, "2160P": true, "4K": true,
		"BLURAY": true, "BDRIP": true, "WEBRIP": true, "HDTV": true, "WEB-DL": true,
		"X264": true, "X265": true, "HEVC": true, "AVC": true,
		"AAC": true, "AC3": true, "DTS": true,
	}
	parts := strings.Fields(base)
	var kept []string
	for _, p := range parts {
		if qualityTags[strings.ToUpper(p)] {
			break
		}
		kept = append(kept, p)
	}
	if len(kept) > 0 {
		base = strings.Join(kept, " ")
	}

	title = strings.TrimSpace(base)
	return title, year
}

// Scan walks dir recursively, registers any video files not yet in s, and returns
// the number of new titles added.
func Scan(s *store.Store, dir string) (int, error) {
	if _, err := os.Stat(dir); err != nil {
		return 0, err
	}

	// Build a set of already-known DirectPaths to avoid duplicates
	existing, _ := s.List()
	known := make(map[string]bool, len(existing))
	for _, t := range existing {
		if t.DirectPath != "" {
			known[t.DirectPath] = true
		}
	}

	added := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir // skip hidden dirs
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !VideoExts[ext] {
			return nil
		}
		if known[path] {
			return nil
		}

		title, year := ParseFilename(d.Name())
		if title == "" {
			title = d.Name()
		}

		t := &store.Title{
			ID:              newID(),
			Title:           title,
			Year:            year,
			Genre:           []string{},
			Type:            "movie",
			StreamReady:     false,
			TranscodeStatus: store.StatusPending,
			DirectPath:      path,
			DirectExt:       ext,
			CreatedAt:       time.Now(),
		}
		if err := s.Save(t); err != nil {
			log.Printf("scanner: failed to save %q: %v", path, err)
			return nil
		}
		log.Printf("scanner: imported %q", title)
		added++
		return nil
	})
	return added, err
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
