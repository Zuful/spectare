// Package scanner discovers video files in a directory and registers them in the store.
package scanner

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Zuful/spectare/internal/store"
	"github.com/Zuful/spectare/internal/tmdb"
)

// VideoExts lists supported video file extensions.
var VideoExts = map[string]bool{
	".mp4": true, ".m4v": true, ".mkv": true, ".avi": true,
	".mov": true, ".webm": true, ".flv": true, ".wmv": true,
	".ts": true, ".mpg": true, ".mpeg": true,
}

// ImageExts lists supported image file extensions.
var ImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
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
var seasonRe = regexp.MustCompile(`(?i)^(?:season|saison|serie|series|staffel|temporada)\s*(\d+)$|^[Ss](\d+)$`)
var episodeRe = regexp.MustCompile(`(?i)[Ss](\d{1,2})[Ee](\d{1,3})|[Ee][Pp]?(\d{1,3})`)

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
func Scan(s *store.Store, dir string, tmdbClient *tmdb.Client) (int, error) {
	if _, err := os.Stat(dir); err != nil {
		return 0, err
	}

	// Build a set of already-known DirectPaths to avoid duplicates.
	// Also remove titles whose DirectPath no longer exists on disk (stale entries
	// left behind after a data directory wipe or drive disconnect).
	existing, _ := s.List()
	existing = deduplicateSeries(s, existing)
	known := make(map[string]bool, len(existing))
	knownTitles := make(map[string]*store.Title, len(existing)) // path → title, for companion re-check
	for _, t := range existing {
		if t.DirectPath != "" {
			if _, err := os.Stat(t.DirectPath); os.IsNotExist(err) {
				// Source file gone — remove the stale entry so it can be re-imported
				log.Printf("scanner: removing stale title %q (path no longer exists: %s)", t.Title, t.DirectPath)
				s.Delete(t.ID)
				continue
			}
			known[t.DirectPath] = true
			knownTitles[t.DirectPath] = t
		}
	}

	// Build a set of already-known episode DirectPaths
	// We scan all series titles and their episodes to avoid duplicates.
	// Also remove any episodes whose source file is a macOS resource-fork sidecar (._*).
	knownEpisodePaths := make(map[string]bool)
	for _, t := range existing {
		if t.Type == "series" {
			eps, _ := s.ListEpisodes(t.ID)
			for _, e := range eps {
				if e.DirectPath != "" {
					if strings.HasPrefix(filepath.Base(e.DirectPath), ".") {
						log.Printf("scanner: removing hidden/sidecar episode %q", e.DirectPath)
						s.DeleteEpisode(e.ID)
						continue
					}
					knownEpisodePaths[e.DirectPath] = true
				}
			}
		}
	}

	// newSeries tracks series titles created during this scan to avoid duplicates
	// when multiple episodes from the same series are discovered in the same run.
	// (existing is loaded once before the walk and would be stale otherwise.)
	newSeries := make(map[string]*store.Title)

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

		// Skip hidden files and macOS resource-fork sidecar files (._*)
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !VideoExts[ext] {
			return nil
		}

		// Check if this file is inside a season folder
		parentDir := filepath.Dir(path)
		parentName := filepath.Base(parentDir)
		if m := seasonRe.FindStringSubmatch(parentName); m != nil {
			// This is an episode file
			if knownEpisodePaths[path] {
				return nil
			}

			// Extract season number (group 1 or group 2 depending on which branch matched)
			seasonNum := 0
			if m[1] != "" {
				seasonNum, _ = strconv.Atoi(m[1])
			} else if m[2] != "" {
				seasonNum, _ = strconv.Atoi(m[2])
			}

			// Series name is the grandparent directory
			grandparentDir := filepath.Dir(parentDir)
			seriesName := filepath.Base(grandparentDir)

			// Find or create a series title
			seriesTitle, err := findOrCreateSeries(s, seriesName, existing, newSeries)
			if err != nil {
				log.Printf("scanner: failed to find/create series %q: %v", seriesName, err)
				return nil
			}

			// Parse episode number and title from filename
			base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
			episodeNum := 0
			episodeTitle := base
			if em := episodeRe.FindStringSubmatchIndex(base); em != nil {
				submatches := episodeRe.FindStringSubmatch(base)
				// submatches[1],[2] = SxxExx groups; submatches[3] = Exx group
				if submatches[2] != "" {
					episodeNum, _ = strconv.Atoi(submatches[2])
				} else if submatches[3] != "" {
					episodeNum, _ = strconv.Atoi(submatches[3])
				}
				// Episode title is everything after the pattern
				afterIdx := em[1]
				tail := strings.TrimSpace(base[afterIdx:])
				tail = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(tail)
				tail = strings.TrimSpace(tail)
				if tail != "" {
					episodeTitle = tail
				}
			} else {
				// No episode pattern in filename; use cleaned filename as title
				episodeTitle, _ = ParseFilename(d.Name())
				if episodeTitle == "" {
					episodeTitle = base
				}
			}

			e := &store.Episode{
				ID:              newID(),
				SeriesID:        seriesTitle.ID,
				Season:          seasonNum,
				Number:          episodeNum,
				Title:           episodeTitle,
				StreamReady:     false,
				TranscodeStatus: store.StatusPending,
				DirectPath:      path,
				DirectExt:       ext,
				CreatedAt:       time.Now(),
			}
			if err := s.SaveEpisode(e); err != nil {
				log.Printf("scanner: failed to save episode %q: %v", path, err)
				return nil
			}
			knownEpisodePaths[path] = true
			log.Printf("scanner: imported episode S%02dE%02d %q (series: %q)", seasonNum, episodeNum, episodeTitle, seriesName)
			added++
			return nil
		}

		if known[path] {
			// Already registered — re-check companion files in case they were added later
			if t, ok := knownTitles[path]; ok {
				thumbDir := filepath.Join(s.TitleDir(t.ID), "thumbnails")
				if !existsGlob(thumbDir, "card.*") || !existsGlob(thumbDir, "poster.*") || !existsGlob(thumbDir, "backdrop.*") {
					importCompanionFiles(s, t.ID, path, d.Name())
				}
			}
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

		importCompanionFiles(s, t.ID, path, d.Name())

		if tmdbClient != nil {
			go enrichFromTMDB(s, t, tmdbClient)
		}

		log.Printf("scanner: imported %q", title)
		added++
		return nil
	})
	return added, err
}

// enrichFromTMDB fetches metadata from TMDB and fills in missing fields on a title.
func enrichFromTMDB(s *store.Store, t *store.Title, client *tmdb.Client) {
	result, genres, err := client.FetchMetadata(t.Title, t.Type, t.Year)
	if err != nil {
		log.Printf("TMDB: no match for %q: %v", t.Title, err)
		return
	}
	changed := false
	if t.Synopsis == "" && result.Overview != "" {
		t.Synopsis = result.Overview
		changed = true
	}
	if t.Year == 0 && result.Year() > 0 {
		t.Year = result.Year()
		changed = true
	}
	if (len(t.Genre) == 0 || (len(t.Genre) == 1 && t.Genre[0] == "Uncategorised")) && len(genres) > 0 {
		t.Genre = genres
		changed = true
	}
	if t.Director == "" {
		if dir := client.Director(result.ID, result.MediaType); dir != "" {
			t.Director = dir
			changed = true
		}
	}
	thumbDir := filepath.Join(s.TitleDir(t.ID), "thumbnails")
	os.MkdirAll(thumbDir, 0755)
	posterGlob, _ := filepath.Glob(filepath.Join(thumbDir, "poster.*"))
	if len(posterGlob) == 0 {
		client.DownloadImage(result.PosterPath, "w342", filepath.Join(thumbDir, "poster.jpg"))
	}
	backdropGlob, _ := filepath.Glob(filepath.Join(thumbDir, "backdrop.*"))
	if len(backdropGlob) == 0 {
		client.DownloadImage(result.BackdropPath, "w1280", filepath.Join(thumbDir, "backdrop.jpg"))
	}
	cardGlob, _ := filepath.Glob(filepath.Join(thumbDir, "card.*"))
	if len(cardGlob) == 0 {
		client.DownloadImage(result.BackdropPath, "w780", filepath.Join(thumbDir, "card.jpg"))
	}
	if changed {
		s.Save(t)
	}
	log.Printf("TMDB: enriched %q (id=%d)", t.Title, result.ID)
}

// deduplicateSeries merges series entries that share the same title (case-insensitive).
// It keeps the oldest entry as canonical and re-assigns all episodes from duplicates to it,
// then deletes the duplicate entries. Returns the updated title list.
func deduplicateSeries(s *store.Store, existing []*store.Title) []*store.Title {
	groups := make(map[string][]*store.Title)
	for _, t := range existing {
		if t.Type != "series" {
			continue
		}
		key := strings.ToLower(t.Title)
		groups[key] = append(groups[key], t)
	}

	merged := false
	for _, group := range groups {
		if len(group) <= 1 {
			continue
		}
		// Keep the oldest entry as canonical
		sort.Slice(group, func(i, j int) bool {
			return group[i].CreatedAt.Before(group[j].CreatedAt)
		})
		canonical := group[0]
		seenEpisodes := make(map[string]bool) // directPath → already kept
		// Seed with episodes already on the canonical series
		if eps, _ := s.ListEpisodes(canonical.ID); len(eps) > 0 {
			for _, e := range eps {
				if e.DirectPath != "" {
					seenEpisodes[e.DirectPath] = true
				}
			}
		}
		for _, dup := range group[1:] {
			eps, _ := s.ListEpisodes(dup.ID)
			for _, e := range eps {
				if e.DirectPath != "" && seenEpisodes[e.DirectPath] {
					// True duplicate episode — remove it
					s.DeleteEpisode(e.ID)
					continue
				}
				e.SeriesID = canonical.ID
				if err := s.SaveEpisode(e); err == nil && e.DirectPath != "" {
					seenEpisodes[e.DirectPath] = true
				}
			}
			s.Delete(dup.ID)
			log.Printf("scanner: merged duplicate series %q (%s) into %s", dup.Title, dup.ID, canonical.ID)
			merged = true
		}
	}

	if !merged {
		return existing
	}
	updated, _ := s.List()
	return updated
}

// findOrCreateSeries finds an existing series Title by name or creates a new one.
// newSeries is an in-scan cache that must be checked (and updated) so that multiple
// episodes discovered in the same scan run share the same series entry.
func findOrCreateSeries(s *store.Store, name string, existing []*store.Title, newSeries map[string]*store.Title) (*store.Title, error) {
	// 1. Look in titles that existed before this scan started.
	for _, t := range existing {
		if t.Type == "series" && strings.EqualFold(t.Title, name) {
			return t, nil
		}
	}
	// 2. Look in series created earlier during this same scan run.
	key := strings.ToLower(name)
	if t, ok := newSeries[key]; ok {
		return t, nil
	}
	// 3. Create a new series entry and register it in the in-scan cache.
	t := &store.Title{
		ID:              newID(),
		Title:           name,
		Type:            "series",
		Genre:           []string{},
		StreamReady:     false,
		TranscodeStatus: store.StatusPending,
		CreatedAt:       time.Now(),
	}
	if err := s.Save(t); err != nil {
		return nil, err
	}
	newSeries[key] = t
	return t, nil
}

// importCompanionFiles looks for thumbnail images and subtitle files alongside
// the video file and copies them into the title's data directory.
//
// Thumbnail lookup order (per variant):
//
//	poster.{ext}  / backdrop.{ext}  → explicit variant files
//	{videobase}.{ext}                → named after the video file
//	cover.{ext} / thumb.{ext}        → common generic names (→ card)
//
// Subtitle lookup:
//
//	{lang}.srt / {lang}.vtt          → e.g. en.srt, fr.vtt
//	{videobase}.{lang}.srt/vtt       → e.g. Inception.2010.en.srt
func importCompanionFiles(s *store.Store, id, videoPath, videoName string) {
	videoDir := filepath.Dir(videoPath)
	videoBase := strings.TrimSuffix(videoName, filepath.Ext(videoName))
	titleDir := s.TitleDir(id)

	// ── Thumbnails ─────────────────────────────────────────────────────────────
	thumbDir := filepath.Join(titleDir, "thumbnails")
	os.MkdirAll(thumbDir, 0755)

	// variant → candidate filenames (first match wins)
	thumbCandidates := map[string][]string{
		"card":     {videoBase + ".card", "card", "cover", "thumb", videoBase},
		"poster":   {videoBase + ".poster", "poster", videoBase},
		"backdrop": {videoBase + ".backdrop", "backdrop", "fanart", "background", videoBase},
	}

	for variant, names := range thumbCandidates {
		// Skip if already present
		if existsGlob(thumbDir, variant+".*") {
			continue
		}
		for _, stem := range names {
			if found := findImageFile(videoDir, stem); found != "" {
				dst := filepath.Join(thumbDir, variant+filepath.Ext(found))
				if err := copyFile(found, dst); err == nil {
					log.Printf("scanner: copied %s thumbnail for %q from %s", variant, videoBase, found)
				}
				break
			}
		}
	}

	// ── Subtitles ──────────────────────────────────────────────────────────────
	subtitleDir := filepath.Join(titleDir, "subtitles")
	os.MkdirAll(subtitleDir, 0755)

	// Known ISO 639-1 language codes
	langCodes := []string{
		"en", "fr", "es", "de", "it", "pt", "nl", "ru",
		"ja", "ko", "zh", "ar", "pl", "sv", "da", "no",
		"fi", "tr", "he", "cs", "hu", "ro", "th", "vi",
	}

	for _, lang := range langCodes {
		// Patterns: {lang}.srt, {videobase}.{lang}.srt, {videobase}.{lang}.forced.srt, etc.
		candidates := []string{
			lang + ".srt",
			lang + ".vtt",
			videoBase + "." + lang + ".srt",
			videoBase + "." + lang + ".vtt",
			videoBase + "." + lang + ".forced.srt",
			videoBase + "." + lang + ".forced.vtt",
		}
		for _, candidate := range candidates {
			src := filepath.Join(videoDir, candidate)
			if _, err := os.Stat(src); err == nil {
				ext := filepath.Ext(candidate)
				dstName := lang + ext
				dst := filepath.Join(subtitleDir, dstName)
				if _, err := os.Stat(dst); err != nil { // don't overwrite
					if err := copyFile(src, dst); err == nil {
						log.Printf("scanner: copied %s subtitle for %q", lang, videoBase)
					}
				}
				break
			}
		}
	}
}

// findImageFile returns the path of the first image file matching stem.{ext} in dir.
func findImageFile(dir, stem string) string {
	for ext := range ImageExts {
		p := filepath.Join(dir, stem+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// existsGlob reports whether any file matches pattern in dir.
func existsGlob(dir, pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	return len(matches) > 0
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
