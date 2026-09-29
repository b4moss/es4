package recovery

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectAPI is the minimal S3-compatible surface used by the Object adapter.
// Tests inject a fake; production uses the AWS SDK v2 S3 client (GCS via endpoint).
type ObjectAPI interface {
	PutObject(ctx context.Context, bucket, key string, body []byte) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	ListObjectKeys(ctx context.Context, bucket, prefix string) ([]string, error)
	DeleteObject(ctx context.Context, bucket, key string) error
}

// ObjectConfig configures an S3-compatible Object Recovery adapter.
type ObjectConfig struct {
	Bucket   string
	Prefix   string
	Region   string
	Endpoint string
	TTL      time.Duration
	Client   ObjectAPI // optional injection (tests / custom)
	Clock    func() time.Time
}

// Object stores recovery generations as objects under Bucket/Prefix.
// GCS is supported via the same adapter with an S3-compatible endpoint.
type Object struct {
	bucket string
	prefix string
	ttl    time.Duration
	client ObjectAPI
	clock  func() time.Time
}

// NewObject returns an Object Recovery adapter. Client must be non-nil.
func NewObject(cfg ObjectConfig) (*Object, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("recovery: empty s3 bucket")
	}
	if cfg.Client == nil {
		return nil, fmt.Errorf("recovery: nil object client")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Object{
		bucket: cfg.Bucket,
		prefix: normalizePrefix(cfg.Prefix),
		ttl:    cfg.TTL,
		client: cfg.Client,
		clock:  clock,
	}, nil
}

// OpenObject builds an Object adapter using the AWS SDK default credential chain
// (standard AWS env vars). Endpoint, when set, targets S3-compatible stores (GCS).
func OpenObject(ctx context.Context, cfg ObjectConfig) (*Object, error) {
	if cfg.Client != nil {
		return NewObject(cfg)
	}
	client, err := newAWSS3Client(ctx, cfg.Region, cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	cfg.Client = client
	return NewObject(cfg)
}

func normalizePrefix(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	return p + "/"
}

func (o *Object) now() time.Time {
	if o.clock != nil {
		return o.clock()
	}
	return time.Now()
}

func (o *Object) objectKey(id string) string {
	return o.prefix + genFilePrefix + id + genFileSuffix
}

func (o *Object) parseKey(key string) (id string, ok bool) {
	if o.prefix != "" {
		if !strings.HasPrefix(key, o.prefix) {
			return "", false
		}
		key = strings.TrimPrefix(key, o.prefix)
	}
	if !strings.HasPrefix(key, genFilePrefix) || !strings.HasSuffix(key, genFileSuffix) {
		return "", false
	}
	id = strings.TrimSuffix(strings.TrimPrefix(key, genFilePrefix), genFileSuffix)
	if _, err := parseGenID(id); err != nil {
		return "", false
	}
	return id, true
}

// Save writes a new generation object and prunes per TTL.
func (o *Object) Save(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id := genID(o.now())
	key := o.objectKey(id)
	if err := o.client.PutObject(ctx, o.bucket, key, data); err != nil {
		return fmt.Errorf("recovery: object put: %w", err)
	}
	return o.prune(ctx)
}

func (o *Object) prune(ctx context.Context) error {
	keys, err := o.listGenKeys(ctx)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	now := o.now()
	newest := keys[len(keys)-1]
	for _, key := range keys {
		if key == newest {
			continue
		}
		id, ok := o.parseKey(key)
		if !ok {
			continue
		}
		created, err := parseGenID(id)
		if err != nil {
			_ = o.client.DeleteObject(ctx, o.bucket, key)
			continue
		}
		if keepGeneration(created, now, o.ttl) {
			continue
		}
		if err := o.client.DeleteObject(ctx, o.bucket, key); err != nil {
			return fmt.Errorf("recovery: object prune: %w", err)
		}
	}
	return nil
}

func (o *Object) listGenKeys(ctx context.Context) ([]string, error) {
	all, err := o.client.ListObjectKeys(ctx, o.bucket, o.prefix+genFilePrefix)
	if err != nil {
		return nil, fmt.Errorf("recovery: object list: %w", err)
	}
	var keys []string
	for _, k := range all {
		if _, ok := o.parseKey(k); ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// Load returns the latest generation payload.
func (o *Object) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keys, err := o.listGenKeys(ctx)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, ErrNotFound
	}
	latest := keys[len(keys)-1]
	data, err := o.client.GetObject(ctx, o.bucket, latest)
	if err != nil {
		return nil, fmt.Errorf("recovery: object get: %w", err)
	}
	return data, nil
}

// --- AWS SDK v2 wrapper ---

type awsS3Client struct {
	api *s3.Client
}

func newAWSS3Client(ctx context.Context, region, endpoint string) (*awsS3Client, error) {
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("recovery: aws config: %w", err)
	}
	var s3opts []func(*s3.Options)
	if endpoint != "" {
		ep := endpoint
		s3opts = append(s3opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(ep)
			o.UsePathStyle = true
		})
	}
	return &awsS3Client{api: s3.NewFromConfig(cfg, s3opts...)}, nil
}

func (c *awsS3Client) PutObject(ctx context.Context, bucket, key string, body []byte) error {
	_, err := c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}

func (c *awsS3Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
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

func (c *awsS3Client) ListObjectKeys(ctx context.Context, bucket, prefix string) ([]string, error) {
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

func (c *awsS3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// MemoryObject is an in-process fake ObjectAPI for tests (no cloud).
type MemoryObject struct {
	objects map[string]map[string][]byte // bucket -> key -> body
}

// NewMemoryObject returns an empty fake ObjectAPI.
func NewMemoryObject() *MemoryObject {
	return &MemoryObject{objects: make(map[string]map[string][]byte)}
}

func (m *MemoryObject) bucketMap(bucket string) map[string][]byte {
	b, ok := m.objects[bucket]
	if !ok {
		b = make(map[string][]byte)
		m.objects[bucket] = b
	}
	return b
}

func (m *MemoryObject) PutObject(_ context.Context, bucket, key string, body []byte) error {
	cp := append([]byte(nil), body...)
	m.bucketMap(bucket)[key] = cp
	return nil
}

func (m *MemoryObject) GetObject(_ context.Context, bucket, key string) ([]byte, error) {
	b, ok := m.objects[bucket]
	if !ok {
		return nil, fmt.Errorf("recovery: mock get: bucket %q missing", bucket)
	}
	data, ok := b[key]
	if !ok {
		return nil, fmt.Errorf("recovery: mock get: key %q missing", key)
	}
	return append([]byte(nil), data...), nil
}

func (m *MemoryObject) ListObjectKeys(_ context.Context, bucket, prefix string) ([]string, error) {
	b, ok := m.objects[bucket]
	if !ok {
		return nil, nil
	}
	var keys []string
	for k := range b {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *MemoryObject) DeleteObject(_ context.Context, bucket, key string) error {
	if b, ok := m.objects[bucket]; ok {
		delete(b, key)
	}
	return nil
}
