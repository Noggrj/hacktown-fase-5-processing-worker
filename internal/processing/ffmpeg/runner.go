// Package ffmpeg shells out to the ffmpeg binary to extract one frame
// per second from a video file, then zips the resulting PNGs — the same
// pipeline as the original FIAP X proof of concept, just moved out of
// the HTTP request path into this worker.
package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Runner implements usecase.FrameExtractor.
type Runner struct{}

func NewRunner() *Runner { return &Runner{} }

// ExtractFrames runs `ffmpeg -vf fps=1` against videoPath, writing PNG
// frames and the resulting zip inside workDir, and returns the zip's
// path plus how many frames were found.
func (r *Runner) ExtractFrames(ctx context.Context, videoPath, workDir string) (zipPath string, frameCount int, err error) {
	framesDir := filepath.Join(workDir, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create frames dir: %w", err)
	}
	framePattern := filepath.Join(framesDir, "frame_%04d.png")

	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", videoPath, "-vf", "fps=1", "-y", framePattern)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("ffmpeg failed: %w (output: %s)", err, string(output))
	}

	frames, err := filepath.Glob(filepath.Join(framesDir, "*.png"))
	if err != nil {
		return "", 0, fmt.Errorf("glob frames: %w", err)
	}
	if len(frames) == 0 {
		return "", 0, fmt.Errorf("no frames extracted from video")
	}

	zipPath = filepath.Join(workDir, "frames.zip")
	if err := zipFiles(frames, zipPath); err != nil {
		return "", 0, err
	}
	return zipPath, len(frames), nil
}
