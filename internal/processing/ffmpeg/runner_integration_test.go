package ffmpeg_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/noggrj/hacktown-fase-5-processing-worker/internal/processing/ffmpeg"
)

// TestExtractFrames_RealFFmpeg is an integration test: it shells out to
// the real `ffmpeg` binary twice — once to synthesize a tiny test-pattern
// video, once (via Runner) to extract frames from it. Skipped when
// ffmpeg isn't on PATH (e.g. a bare `go test` outside the Docker image),
// so it never blocks local development, only exercises the real pipeline
// in CI/Docker where ffmpeg is guaranteed to be installed.
func TestExtractFrames_RealFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH — skipping integration test")
	}

	workDir := t.TempDir()
	videoPath := filepath.Join(workDir, "test.mp4")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	gen := exec.CommandContext(ctx, "ffmpeg",
		"-f", "lavfi", "-i", "testsrc=duration=3:size=64x64:rate=1",
		"-y", videoPath)
	if output, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("failed to synthesize test video: %v\n%s", err, output)
	}
	if _, err := os.Stat(videoPath); err != nil {
		t.Fatalf("synthesized video missing: %v", err)
	}

	runner := ffmpeg.NewRunner()
	zipPath, frameCount, err := runner.ExtractFrames(ctx, videoPath, workDir)
	if err != nil {
		t.Fatalf("ExtractFrames: %v", err)
	}
	if frameCount < 2 {
		t.Fatalf("expected at least 2 frames from a 3s/1fps video, got %d", frameCount)
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("expected zip to exist at %s: %v", zipPath, err)
	}
}
