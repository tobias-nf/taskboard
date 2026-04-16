package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client           *s3.Client
	presign          *s3.PresignClient
	bucket           string
	autoCreateBucket bool
}

type Config struct {
	Endpoint         string
	Region           string
	AccessKey        string
	SecretKey        string
	Bucket           string
	UseSSL           bool
	ForcePathStyle   bool
	AutoCreateBucket bool
}

func NewS3Client(cfg Config) (*S3Client, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket must be set")
	}
	if (cfg.AccessKey == "") != (cfg.SecretKey == "") {
		return nil, fmt.Errorf("access key and secret key must be provided together")
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	clientOptions := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = cfg.ForcePathStyle
		},
	}
	if cfg.Endpoint != "" {
		baseEndpoint := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
		clientOptions = append(clientOptions, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(baseEndpoint)
		})
	}

	client := s3.NewFromConfig(awsCfg, clientOptions...)
	return &S3Client{
		client:           client,
		presign:          s3.NewPresignClient(client),
		bucket:           cfg.Bucket,
		autoCreateBucket: cfg.AutoCreateBucket,
	}, nil
}

// EnsureBucket verifies the bucket exists and optionally creates it for local S3-compatible endpoints.
func (s *S3Client) EnsureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil || !s.autoCreateBucket {
		return err
	}

	if !isBucketMissing(err) {
		return err
	}

	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}

// Upload stores a file in S3 and returns the storage key.
func (s *S3Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := s.client.PutObject(ctx, input)
	return err
}

// Download returns a reader for the given storage key.
func (s *S3Client) Download(ctx context.Context, key string) (io.ReadCloser, string, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}

	contentType := "application/octet-stream"
	if resp.ContentType != nil && *resp.ContentType != "" {
		contentType = *resp.ContentType
	}

	return resp.Body, contentType, nil
}

// PresignedURL generates a time-limited download URL.
func (s *S3Client) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Delete removes a file from S3.
func (s *S3Client) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func normalizeEndpoint(endpoint string, useSSL bool) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}

	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, endpoint)
}

func isBucketMissing(err error) bool {
	var respErr *awshttp.ResponseError
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr.HTTPStatusCode() == http.StatusNotFound
}
