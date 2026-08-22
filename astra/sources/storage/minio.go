package storage

import (
	"astra/astra/config"
	"context"
	"crypto/md5" // For simple URL hashing
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOClient struct {
	client *minio.Client
	bucket string
}

type ScrapeObject struct {
	URL       string    `json:"url"`
	Text      string    `json:"extracted_text"`
	Metadata  string    `json:"metadata"`
	Timestamp time.Time `json:"timestamp"`
}

func NewMinIOClient(cfg config.Config) (*MinIOClient, error) {
	// Use insecure for local (no HTTPS)
	bucket := cfg.MinIOBucket
	client, err := minio.New(
		cfg.MinIOEndpoint,
		&minio.Options{
			Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
			Secure: false,
		},
	)

	fmt.Println("NewMinIOClient ", err)
	if err != nil {
		return nil, err
	}
	return &MinIOClient{client: client, bucket: bucket}, nil
}

func (m *MinIOClient) ensureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (m *MinIOClient) PutObject(ctx context.Context, key string, data []byte, contentType string) error {
	if err := m.ensureBucket(ctx); err != nil {
		return err
	}
	_, err := m.client.PutObject(ctx, m.bucket, key, strings.NewReader(string(data)), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (m *MinIOClient) GetObjectBytes(ctx context.Context, key string) ([]byte, error) {
	if err := m.ensureBucket(ctx); err != nil {
		return nil, err
	}
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (m *MinIOClient) UploadScrape(ctx context.Context, url, text, metadata string) (string, error) {
	// Hash URL for key (avoid special chars)
	hash := fmt.Sprintf("%x", md5.Sum([]byte(url)))
	key := filepath.Join("scrapes", fmt.Sprintf("%s.json", hash))

	obj := ScrapeObject{
		URL:       url,
		Text:      text,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	// Upload
	if err = m.PutObject(ctx, key, data, "application/json"); err != nil {
		return "", err
	}

	return key, nil
}

func (m *MinIOClient) GetScrape(ctx context.Context, key string) (string, error) {
	data, err := m.GetObjectBytes(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
