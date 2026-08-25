package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/noggrj/hacktown-fase-5-events/payloads"
)

type ProcessVideoUseCase struct {
	downloader Downloader
	uploader   Uploader
	extractor  FrameExtractor
	pub        Publisher
	log        *slog.Logger
}

func NewProcessVideo(d Downloader, u Uploader, e FrameExtractor, pub Publisher, log *slog.Logger) *ProcessVideoUseCase {
	return &ProcessVideoUseCase{downloader: d, uploader: u, extractor: e, pub: pub, log: log}
}

// Execute downloads the raw video, extracts frames, uploads the zip and
// publishes the outcome.
//
// Two very different failure modes are handled deliberately differently:
//   - Infra failures (S3 unreachable, Kafka publish failing) return a
//     non-nil error — the saga consumer routes the message to the DLQ so
//     it isn't silently dropped, since these are expected to be transient.
//   - A processing failure (corrupt video, ffmpeg exits non-zero) is a
//     legitimate business outcome, not a transport error: it is captured
//     as a video.failed event and Execute returns nil so the Kafka
//     message commits normally instead of looping through the DLQ.
func (uc *ProcessVideoUseCase) Execute(ctx context.Context, p payloads.VideoUploaded, traceparent string) error {
	workDir, err := os.MkdirTemp("", "fiapx-worker-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	videoPath := filepath.Join(workDir, filepath.Base(p.Filename))
	if err := uc.downloadTo(ctx, p.S3RawKey, videoPath); err != nil {
		return fmt.Errorf("download raw video: %w", err)
	}

	zipPath, frameCount, err := uc.extractor.ExtractFrames(ctx, videoPath, workDir)
	if err != nil {
		uc.log.Warn("video processing failed", slog.String("videoId", p.VideoID), slog.Any("error", err))
		if pubErr := uc.pub.PublishVideoFailed(ctx, traceparent, p.VideoID, p.UserID, p.UserEmail, err.Error()); pubErr != nil {
			return fmt.Errorf("publish video.failed: %w", pubErr)
		}
		return nil
	}

	zipKey := fmt.Sprintf("processed/%s.zip", p.VideoID)
	if err := uc.uploadZip(ctx, zipPath, zipKey); err != nil {
		return fmt.Errorf("upload zip: %w", err)
	}

	if err := uc.pub.PublishVideoProcessed(ctx, traceparent, p.VideoID, zipKey, frameCount); err != nil {
		return fmt.Errorf("publish video.processed: %w", err)
	}
	uc.log.Info("video processed", slog.String("videoId", p.VideoID), slog.Int("frameCount", frameCount))
	return nil
}

func (uc *ProcessVideoUseCase) downloadTo(ctx context.Context, key, destPath string) error {
	rc, err := uc.downloader.Download(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, rc)
	return err
}

func (uc *ProcessVideoUseCase) uploadZip(ctx context.Context, zipPath, zipKey string) error {
	f, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	return uc.uploader.Upload(ctx, zipKey, f, info.Size())
}
