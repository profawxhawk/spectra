package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store is an ObjectStore backed by Amazon S3 (or any S3-compatible
// service such as MinIO).
type S3Store struct {
	client *s3.Client
	bucket string
}

// S3Option configures optional S3Store behaviour.
type S3Option func(*s3Options)

type s3Options struct {
	endpoint       string
	forcePathStyle bool
}

// WithEndpoint overrides the default AWS endpoint, which is useful for
// local development against MinIO or similar services.
func WithEndpoint(endpoint string) S3Option {
	return func(o *s3Options) {
		o.endpoint = endpoint
	}
}

// WithPathStyle forces path-style addressing (required by MinIO and
// most S3-compatible stores).
func WithPathStyle(on bool) S3Option {
	return func(o *s3Options) {
		o.forcePathStyle = on
	}
}

// NewS3 returns an S3Store for the given bucket.
// It loads the default AWS SDK configuration from the environment and
// accepts functional options to customise the endpoint (e.g. for MinIO).
func NewS3(ctx context.Context, bucket string, opts ...S3Option) (*S3Store, error) {
	var sopts s3Options
	for _, o := range opts {
		o(&sopts)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	var s3Opts []func(*s3.Options)

	if sopts.endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(sopts.endpoint)
		})
	}
	if sopts.forcePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(cfg, s3Opts...)

	return &S3Store{client: client, bucket: bucket}, nil
}

// Put writes data to the given key.
func (s *S3Store) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

// PutReader streams the contents of r to key.
func (s *S3Store) PutReader(ctx context.Context, key string, r io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	return err
}

// Get returns the contents stored under key or ErrNotFound.
func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// Delete removes key from the bucket. Missing keys are silently ignored.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// List returns all keys sharing the given prefix.
func (s *S3Store) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}
	return keys, nil
}

// Exists reports whether key is present in the bucket.
func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isNotFound returns true if the error indicates the object does not exist.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var nsk *types.NoSuchKey
	if ok := errors.As(err, &nsk); ok {
		return true
	}
	var nf *types.NotFound
	if ok := errors.As(err, &nf); ok {
		return true
	}
	// HeadObject returns a generic error with "NotFound" in the message
	// because the service responds with HTTP 404 and no XML body.
	if strings.Contains(err.Error(), "NotFound") ||
		strings.Contains(err.Error(), "404") {
		return true
	}
	return false
}
