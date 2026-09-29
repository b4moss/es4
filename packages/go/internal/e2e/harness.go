// Package e2e provides shared helpers for Object Recovery × RustFS E2E tests.
//
// Build with -tags=e2e. Helpers assemble an AWS SDK v2 S3 client against a
// local RustFS endpoint and implement recovery.ObjectAPI for OpenWith injection.
package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

// Default endpoint / credentials match docker/e2e/docker-compose.yml.
const (
	DefaultEndpoint  = "http://127.0.0.1:9000"
	DefaultRegion    = "us-east-1"
	DefaultAccessKey = "es4e2eaccess"
	DefaultSecretKey = "es4e2esecretkey"
)

// Config holds RustFS connection settings for E2E.
type Config struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
}

// ConfigFromEnv reads ES4_E2E_* / AWS_* with compose defaults.
func ConfigFromEnv() Config {
	return Config{
		Endpoint:  firstNonEmpty(os.Getenv("ES4_E2E_S3_ENDPOINT"), DefaultEndpoint),
		Region:    firstNonEmpty(os.Getenv("ES4_E2E_S3_REGION"), os.Getenv("AWS_REGION"), DefaultRegion),
		AccessKey: firstNonEmpty(os.Getenv("ES4_E2E_S3_ACCESS_KEY"), os.Getenv("AWS_ACCESS_KEY_ID"), DefaultAccessKey),
		SecretKey: firstNonEmpty(os.Getenv("ES4_E2E_S3_SECRET_KEY"), os.Getenv("AWS_SECRET_ACCESS_KEY"), DefaultSecretKey),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Client wraps *s3.Client and implements recovery.ObjectAPI.
type Client struct {
	api *s3.Client
	cfg Config
}

// NewClient builds a path-style S3 client for RustFS (or other S3-compatible).
func NewClient(cfg Config) *Client {
	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}
	if cfg.Region == "" {
		cfg.Region = DefaultRegion
	}
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}
	api := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	return &Client{api: api, cfg: cfg}
}

// Config returns the connection settings.
func (c *Client) Config() Config { return c.cfg }

// WaitHealthy probes GET {endpoint}/health until success or timeout.
func WaitHealthy(ctx context.Context, endpoint string) error {
	endpoint = strings.TrimRight(endpoint, "/")
	url := endpoint + "/health"
	deadline := time.Now().Add(60 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	var last error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				return nil
			}
			last = fmt.Errorf("health status %d", res.StatusCode)
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("e2e: rustfs not healthy at %s: %w", url, last)
}

// UniqueBucket returns a collision-resistant bucket name (S3 naming rules).
func UniqueBucket(prefix string) string {
	if prefix == "" {
		prefix = "es4-e2e"
	}
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	name := fmt.Sprintf("%s-%d-%s", prefix, time.Now().Unix(), id[:8])
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.ToLower(name)
}

// UniquePrefix returns a per-case object key prefix (trailing slash).
func UniquePrefix(caseName string) string {
	caseName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, caseName)
	return fmt.Sprintf("run-%s/case-%s/", uuid.NewString()[:8], caseName)
}

// EnsureBucket creates the bucket if missing (idempotent).
func (c *Client) EnsureBucket(ctx context.Context, bucket string) error {
	_, err := c.api.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	_, err = c.api.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		var owned *types.BucketAlreadyOwnedByYou
		var exists *types.BucketAlreadyExists
		if errors.As(err, &owned) || errors.As(err, &exists) {
			return nil
		}
		// Some S3-compatible servers return generic errors after a race.
		if _, headErr := c.api.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)}); headErr == nil {
			return nil
		}
		return fmt.Errorf("e2e: create bucket %q: %w", bucket, err)
	}
	return nil
}

// ClearPrefix deletes all objects under prefix (idempotent; empty is OK).
func (c *Client) ClearPrefix(ctx context.Context, bucket, prefix string) error {
	prefix = normalizePrefix(prefix)
	for {
		out, err := c.api.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		})
		if err != nil {
			return fmt.Errorf("e2e: list %s/%s: %w", bucket, prefix, err)
		}
		if len(out.Contents) == 0 {
			return nil
		}
		var objs []types.ObjectIdentifier
		for _, obj := range out.Contents {
			if obj.Key != nil {
				objs = append(objs, types.ObjectIdentifier{Key: obj.Key})
			}
		}
		_, err = c.api.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{Objects: objs, Quiet: aws.Bool(true)},
		})
		if err != nil {
			// Fallback: delete one-by-one for strict S3 compat.
			for _, o := range objs {
				if _, dErr := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
					Bucket: aws.String(bucket),
					Key:    o.Key,
				}); dErr != nil {
					return fmt.Errorf("e2e: delete %s: %w", aws.ToString(o.Key), dErr)
				}
			}
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			return nil
		}
	}
}

// DeleteBucketEmpty deletes an empty bucket (best-effort after ClearPrefix).
func (c *Client) DeleteBucketEmpty(ctx context.Context, bucket string) error {
	_, err := c.api.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	return err
}

// ListKeys returns object keys under prefix.
func (c *Client) ListKeys(ctx context.Context, bucket, prefix string) ([]string, error) {
	return c.ListObjectKeys(ctx, bucket, prefix)
}

// --- recovery.ObjectAPI ---

func (c *Client) PutObject(ctx context.Context, bucket, key string, body []byte) error {
	_, err := c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}

func (c *Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := c.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (c *Client) ListObjectKeys(ctx context.Context, bucket, prefix string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(c.api, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}
	return keys, nil
}

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func normalizePrefix(p string) string {
	p = strings.TrimLeft(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}
