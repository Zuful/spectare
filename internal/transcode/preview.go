package transcode

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// GeneratePreview extracts the first 30 seconds of inputPath as a web-optimised
// 640px-wide MP4 at {titleDir}/preview.mp4. Runs asynchronously and does NOT
// overwrite an existing file (allows manual override).
func GeneratePreview(titleDir, inputPath string) {
	outPath := filepath.Join(titleDir, "preview.mp4")
	if _, err := os.Stat(outPath); err == nil {
		return // manual preview already present — don't overwrite
	}
	go func() {
		args := []string{
			"-i", inputPath,
			"-t", "30",
			"-c:v", "libx264", "-preset", "fast", "-crf", "28",
			"-c:a", "aac", "-b:a", "96k",
			"-vf", "scale=640:-2",
			"-movflags", "+faststart",
			"-y", outPath,
		}
		if err := exec.Command("ffmpeg", args...).Run(); err != nil {
			log.Printf("preview: ffmpeg failed for %s: %v", filepath.Base(titleDir), err)
			os.Remove(outPath)
		} else {
			log.Printf("preview: generated for %s", filepath.Base(titleDir))
		}
	}()
}
