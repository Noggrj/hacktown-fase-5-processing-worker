package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/noggrj/fiapx-events/payloads"
	"github.com/noggrj/fiapx-processing-worker/internal/processing/usecase"
)

// ---------- fakes ----------

type fakeDownloader struct {
	content []byte
	err     error
}

func (f fakeDownloader) Download(context.Context, string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(bytes.NewReader(f.content)), nil
}

type fakeUploader struct {
	mu       sync.Mutex
	uploaded map[string][]byte
	err      error
}

func newFakeUploader() *fakeUploader { return &fakeUploader{uploaded: map[string][]byte{}} }

func (f *fakeUploader) Upload(_ context.Context, key string, body io.Reader, _ int64) error {
	if f.err != nil {
		return f.err
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploaded[key] = data
	return nil
}

// fakeExtractor writes a small file to workDir and returns its path, so
// ProcessVideoUseCase's upload step has something real to read — it does
// not shell out to ffmpeg.
type fakeExtractor struct {
	frameCount int
	err        error
}

func (f fakeExtractor) ExtractFrames(_ context.Context, _ string, workDir string) (string, int, error) {
	if f.err != nil {
		return "", 0, f.err
	}
	zipPath := filepath.Join(workDir, "frames.zip")
	if err := os.WriteFile(zipPath, []byte("fake-zip-bytes"), 0o644); err != nil {
		return "", 0, err
	}
	return zipPath, f.frameCount, nil
}

type fakePublisher struct {
	mu             sync.Mutex
	processedCalls int
	failedCalls    int
	failedReason   string
	processErr     error
	failErr        error
}

func (p *fakePublisher) PublishVideoProcessed(context.Context, string, string, string, int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processedCalls++
	return p.processErr
}

func (p *fakePublisher) PublishVideoFailed(_ context.Context, _, _, _, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failedCalls++
	p.failedReason = reason
	return p.failErr
}

func silentLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

var errBoom = errors.New("boom")

// ---------- tests ----------

func TestProcessVideo_HappyPath(t *testing.T) {
	uploader := newFakeUploader()
	pub := &fakePublisher{}
	uc := usecase.NewProcessVideo(fakeDownloader{content: []byte("video bytes")}, uploader, fakeExtractor{frameCount: 7}, pub, silentLogger())

	err := uc.Execute(context.Background(), payloads.VideoUploaded{VideoID: "v1", UserID: "u1", Filename: "clip.mp4", S3RawKey: "raw/v1/clip.mp4"}, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if pub.processedCalls != 1 || pub.failedCalls != 0 {
		t.Fatalf("expected 1 processed call and 0 failed calls, got processed=%d failed=%d", pub.processedCalls, pub.failedCalls)
	}
	if _, ok := uploader.uploaded["processed/v1.zip"]; !ok {
		t.Fatalf("expected zip uploaded under processed/v1.zip, got keys: %v", uploader.uploaded)
	}
}

func TestProcessVideo_DownloadFailure_ReturnsErrorForDLQ(t *testing.T) {
	pub := &fakePublisher{}
	uc := usecase.NewProcessVideo(fakeDownloader{err: errBoom}, newFakeUploader(), fakeExtractor{}, pub, silentLogger())

	err := uc.Execute(context.Background(), payloads.VideoUploaded{VideoID: "v1", S3RawKey: "raw/v1/clip.mp4"}, "")
	if err == nil {
		t.Fatal("expected a non-nil error so the transport layer routes this to the DLQ")
	}
	if pub.failedCalls != 0 {
		t.Fatal("an infra failure must not be reported as a business video.failed event")
	}
}

func TestProcessVideo_ExtractionFailure_PublishesVideoFailed_ReturnsNil(t *testing.T) {
	pub := &fakePublisher{}
	uc := usecase.NewProcessVideo(fakeDownloader{content: []byte("x")}, newFakeUploader(), fakeExtractor{err: errBoom}, pub, silentLogger())

	err := uc.Execute(context.Background(), payloads.VideoUploaded{VideoID: "v1", UserID: "u1", Filename: "clip.mp4", S3RawKey: "raw/v1/clip.mp4"}, "")
	if err != nil {
		t.Fatalf("a business-level ffmpeg failure must not error out (would loop the message forever): %v", err)
	}
	if pub.failedCalls != 1 || pub.processedCalls != 0 {
		t.Fatalf("expected 1 failed call and 0 processed calls, got failed=%d processed=%d", pub.failedCalls, pub.processedCalls)
	}
	if pub.failedReason == "" {
		t.Fatal("expected a non-empty failure reason")
	}
}

func TestProcessVideo_PublishFailedItselfFails_ReturnsErrorForDLQ(t *testing.T) {
	pub := &fakePublisher{failErr: errBoom}
	uc := usecase.NewProcessVideo(fakeDownloader{content: []byte("x")}, newFakeUploader(), fakeExtractor{err: errBoom}, pub, silentLogger())

	err := uc.Execute(context.Background(), payloads.VideoUploaded{VideoID: "v1", Filename: "clip.mp4", S3RawKey: "raw/v1/clip.mp4"}, "")
	if err == nil {
		t.Fatal("expected an error when we couldn't even report the failure")
	}
}

func TestProcessVideo_UploadFailure_ReturnsErrorForDLQ(t *testing.T) {
	uploader := newFakeUploader()
	uploader.err = errBoom
	pub := &fakePublisher{}
	uc := usecase.NewProcessVideo(fakeDownloader{content: []byte("x")}, uploader, fakeExtractor{frameCount: 1}, pub, silentLogger())

	err := uc.Execute(context.Background(), payloads.VideoUploaded{VideoID: "v1", Filename: "clip.mp4", S3RawKey: "raw/v1/clip.mp4"}, "")
	if err == nil {
		t.Fatal("expected upload failure to propagate as an error")
	}
	if pub.processedCalls != 0 && pub.failedCalls != 0 {
		t.Fatal("no publish should succeed when the upload itself failed")
	}
}

func TestProcessVideo_PublishProcessedFails_ReturnsErrorForDLQ(t *testing.T) {
	pub := &fakePublisher{processErr: errBoom}
	uc := usecase.NewProcessVideo(fakeDownloader{content: []byte("x")}, newFakeUploader(), fakeExtractor{frameCount: 1}, pub, silentLogger())

	err := uc.Execute(context.Background(), payloads.VideoUploaded{VideoID: "v1", Filename: "clip.mp4", S3RawKey: "raw/v1/clip.mp4"}, "")
	if err == nil {
		t.Fatal("expected publish failure to propagate as an error")
	}
}
