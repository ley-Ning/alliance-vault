package storage

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClient struct {
	client *minio.Client
	bucket string
}

func NewMinioClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinioClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	return &MinioClient{client: client, bucket: bucket}, nil
}

func (m *MinioClient) EnsureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket: %w", err)
	}
	if exists {
		return nil
	}

	if err := m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create minio bucket: %w", err)
	}
	return nil
}

func (m *MinioClient) EnsureBucketCORS(ctx context.Context, allowedOrigins []string) error {
	origins := make([]string, 0, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		origins = append(origins, trimmed)
	}
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	config := cors.NewConfig([]cors.Rule{
		{
			AllowedMethod: []string{"GET", "PUT", "HEAD"},
			AllowedOrigin: origins,
			AllowedHeader: []string{"*"},
			ExposeHeader:  []string{"ETag", "Content-Length", "Content-Type"},
			MaxAgeSeconds: 86400,
		},
	})

	if err := m.client.SetBucketCors(ctx, m.bucket, config); err != nil {
		return fmt.Errorf("set minio bucket cors: %w", err)
	}
	return nil
}

func (m *MinioClient) PresignUploadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	presignedURL, err := m.client.PresignedPutObject(ctx, m.bucket, objectKey, expires)
	if err != nil {
		return "", fmt.Errorf("presign upload url: %w", err)
	}
	return presignedURL.String(), nil
}

func (m *MinioClient) PresignDownloadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	presignedURL, err := m.client.PresignedGetObject(ctx, m.bucket, objectKey, expires, nil)
	if err != nil {
		return "", fmt.Errorf("presign download url: %w", err)
	}
	return presignedURL.String(), nil
}

func (m *MinioClient) ObjectExists(ctx context.Context, objectKey string) (bool, int64, error) {
	info, err := m.client.StatObject(ctx, m.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.Code == "NoSuchObject" {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("stat object: %w", err)
	}
	return true, info.Size, nil
}

func (m *MinioClient) RemoveObject(ctx context.Context, objectKey string) error {
	if err := m.client.RemoveObject(ctx, m.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}

func BuildObjectKey(documentID, fileName, randomID string) string {
	documentID = sanitizeSegment(documentID)
	if documentID == "" {
		documentID = "unknown-doc"
	}

	safeName := sanitizeFileName(fileName)
	if safeName == "" {
		safeName = "file"
	}
	stamp := time.Now().UTC().Format("20060102")
	return path.Join("documents", documentID, stamp, randomID+"-"+safeName)
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}

	builder := strings.Builder{}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteRune('-')
		default:
			builder.WriteRune('-')
		}
	}

	return strings.Trim(builder.String(), "-")
}

func sanitizeSegment(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}

	builder := strings.Builder{}
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	return strings.Trim(builder.String(), "-")
}
