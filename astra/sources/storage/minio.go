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
	// Create bucket if not exists
	exists, err := client.BucketExists(context.Background(), bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return &MinIOClient{client: client, bucket: bucket}, nil
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
	_, err = m.client.PutObject(ctx, m.bucket, key, io.NopCloser(strings.NewReader(string(data))), int64(len(data)), minio.PutObjectOptions{ContentType: "application/json"})
	if err != nil {
		return "", err
	}

	return key, nil
}

func (m *MinIOClient) GetScrape(ctx context.Context, key string) (string, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
