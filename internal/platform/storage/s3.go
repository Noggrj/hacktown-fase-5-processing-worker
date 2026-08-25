// Package storage wraps the AWS S3 client used to download raw uploaded
// videos and upload the resulting frame-zip files.
package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3 struct {
	client *s3.Client
	bucket string
}

// NewS3 talks to real AWS S3 when endpoint/accessKey/secretKey are all
// empty (the normal production path). When endpoint is set, it points at
// an S3-compatible service instead (MinIO for local docker-compose)
// using static credentials and path-style addressing.
func NewS3(ctx context.Context, bucket, region, endpoint, accessKey, secretKey string) (*S3, error) {
	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is empty")
	}

	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if endpoint != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})
	return &S3{client: client, bucket: bucket}, nil
}

func (s *S3) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return out.Body, nil
}

func (s *S3) Upload(ctx context.Context, key string, body io.Reader, contentLength int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(contentLength),
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (s *S3) Healthy(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3 client not initialized")
	}
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	return err
}
