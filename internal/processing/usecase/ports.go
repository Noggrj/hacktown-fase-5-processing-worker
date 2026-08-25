// ports.go declares the infrastructure interfaces ProcessVideoUseCase
// depends on. Concrete implementations live under internal/platform/storage
// (S3) and internal/saga (Kafka publisher) — kept out of this package so
// it has zero knowledge of S3 or Kafka.
package usecase

import (
	"context"
	"io"
)

type Downloader interface {
	Download(ctx context.Context, key string) (io.ReadCloser, error)
}

type Uploader interface {
	Upload(ctx context.Context, key string, body io.Reader, contentLength int64) error
}

// FrameExtractor runs the actual video→frames→zip pipeline against a
// local file and returns the path to the resulting zip plus how many
// frames were extracted.
type FrameExtractor interface {
	ExtractFrames(ctx context.Context, videoPath, workDir string) (zipPath string, frameCount int, err error)
}

type Publisher interface {
	PublishVideoProcessed(ctx context.Context, traceparent, videoID, s3ZipKey string, frameCount int) error
	PublishVideoFailed(ctx context.Context, traceparent, videoID, userID, reason string) error
}
