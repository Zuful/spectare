package transcode

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Zuful/spectare/internal/store"
)

var progressRe = regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2}\.\d+)`)

// Start launches ffmpeg transcoding asynchronously and reports progress via s.
func Start(s *store.Store, id, inputPath string) {
	go run(s, id, inputPath)
}

func run(s *store.Store, id, inputPath string) {
	s.SetProgress(id, &store.Progress{Status: store.StatusTranscoding, Progress: 0})

	if err := transcodeToDir(id, inputPath, s.HLSDir(id), s.SetProgress); err != nil {
		s.SetProgress(id, &store.Progress{Status: store.StatusError, Error: err.Error()})
		return
	}
	s.SetProgress(id, &store.Progress{Status: store.StatusReady, Progress: 100})
}

// StartEpisode is like Start but uses episode-specific store methods.
func StartEpisode(s *store.Store, id, inputPath string) {
	go runEpisode(s, id, inputPath)
}

func runEpisode(s *store.Store, id, inputPath string) {
	s.SetEpisodeProgress(id, &store.Progress{Status: store.StatusTranscoding, Progress: 0})
	if err := transcodeToDir(id, inputPath, s.EpisodeHLSDir(id), s.SetEpisodeProgress); err != nil {
		s.SetEpisodeProgress(id, &store.Progress{Status: store.StatusError, Error: err.Error()})
		return
	}
	s.SetEpisodeProgress(id, &store.Progress{Status: store.StatusReady, Progress: 100})
}

func transcodeToDir(id, inputPath, hlsDir string, progressFn func(string, *store.Progress)) error {
	for _, q := range []string{"360p", "720p"} {
		if err := os.MkdirAll(filepath.Join(hlsDir, q), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", q, err)
		}
	}

	duration, _ := probeDuration(inputPath)
	hasAudio := probeHasAudio(inputPath)

	args := buildArgs(inputPath, hlsDir, hasAudio)
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = io.Discard

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg not found — install ffmpeg to enable transcoding: %w", err)
	}

	var lastLines []string // keep last 20 lines of stderr for error reporting
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		line := sc.Text()
		lastLines = append(lastLines, line)
		if len(lastLines) > 20 {
			lastLines = lastLines[len(lastLines)-20:]
		}
		if duration <= 0 {
			continue
		}
		if m := progressRe.FindStringSubmatch(line); m != nil {
			h, _ := strconv.ParseFloat(m[1], 64)
			min, _ := strconv.ParseFloat(m[2], 64)
			sec, _ := strconv.ParseFloat(m[3], 64)
			elapsed := h*3600 + min*60 + sec
			pct := (elapsed / duration) * 100
			if pct > 99 {
				pct = 99
			}
			progressFn(id, &store.Progress{Status: store.StatusTranscoding, Progress: pct})
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("ffmpeg failed for %s:\n%s", id, strings.Join(lastLines, "\n"))
		return fmt.Errorf("ffmpeg exit %w — check server logs for details", err)
	}

	return writeMasterPlaylist(filepath.Join(hlsDir, "master.m3u8"), hasAudio)
}

func buildArgs(inputPath, hlsDir string, hasAudio bool) []string {
	segPattern := filepath.Join(hlsDir, "%v", "seg%03d.ts")
	playlistPattern := filepath.Join(hlsDir, "%v", "stream.m3u8")

	args := []string{
		"-y",
		"-i", inputPath,
	}

	if hasAudio {
		// Two video + two audio streams (one per rendition)
		args = append(args,
			"-map", "0:v:0", "-map", "0:a:0",
			"-map", "0:v:0", "-map", "0:a:0",
		)
	} else {
		args = append(args,
			"-map", "0:v:0",
			"-map", "0:v:0",
		)
	}

	args = append(args,
		"-c:v", "libx264", "-preset", "fast", "-crf", "23",
		"-s:v:0", "640x360", "-b:v:0", "800k",
		"-s:v:1", "1280x720", "-b:v:1", "2800k",
	)

	if hasAudio {
		args = append(args, "-c:a", "aac", "-b:a", "128k")
	}

	varStreamMap := "v:0,name:360p v:1,name:720p"
	if hasAudio {
		varStreamMap = "v:0,a:0,name:360p v:1,a:1,name:720p"
	}

	args = append(args,
		"-var_stream_map", varStreamMap,
		"-f", "hls",
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_list_size", "0",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", segPattern,
		playlistPattern,
	)
	return args
}

func writeMasterPlaylist(path string, hasAudio bool) error {
	codecs := `"avc1.42e01e"`
	if hasAudio {
		codecs = `"avc1.42e01e,mp4a.40.2"`
	}
	content := fmt.Sprintf(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360,CODECS=%s
360p/stream.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720,CODECS=%s
720p/stream.m3u8
`, codecs, codecs)
	return os.WriteFile(path, []byte(content), 0644)
}

func probeDuration(path string) (float64, error) {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func probeHasAudio(path string) bool {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		path,
	).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}
