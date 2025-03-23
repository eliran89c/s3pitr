package s3scanner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type mockS3Client struct {
	s3.Client
}

func (m *mockS3Client) ListObjectVersions(ctx context.Context, params *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	if *params.Prefix == "error" {
		return nil, errors.New("mock error")
	} else if *params.Prefix == "" {
		return &s3.ListObjectVersionsOutput{
			IsTruncated: aws.Bool(false),
			Versions: []types.ObjectVersion{
				{
					Key:          aws.String("prefix-a/object"),
					VersionId:    aws.String("v1"),
					LastModified: aws.Time(time.Now()),
					IsLatest:     aws.Bool(true),
				},
				{
					Key:          aws.String("prefix-b/object"),
					VersionId:    aws.String("v1"),
					LastModified: aws.Time(time.Now()),
					IsLatest:     aws.Bool(true),
				},
			},
		}, nil
	} else if *params.Prefix == "prefix-a/" {
		return &s3.ListObjectVersionsOutput{
			IsTruncated: aws.Bool(false),
			Versions: []types.ObjectVersion{
				{
					Key:          aws.String("prefix-a/object"),
					VersionId:    aws.String("v1"),
					LastModified: aws.Time(time.Now()),
					IsLatest:     aws.Bool(true),
				},
			},
		}, nil
	}
	return nil, nil
}

func (m *mockS3Client) GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if *params.Bucket == "test-bucket" {
		return &s3.GetBucketVersioningOutput{
			Status: types.BucketVersioningStatusEnabled,
		}, nil
	}
	return &s3.GetBucketVersioningOutput{}, nil
}

func TestNewScanner(t *testing.T) {
	ctx := context.Background()
	mockClient := new(mockS3Client)
	scanner, err := NewScanner(mockClient, ctx, 2)
	if scanner == nil || err != nil {
		t.Errorf("NewScanner failed: expected a Scanner instance, got %v and error %v", scanner, err)
	}
}

func TestScan(t *testing.T) {
	ctx := context.Background()

	mockClient := new(mockS3Client)
	scanner, _ := NewScanner(mockClient, ctx, 2)

	fn := func(o *S3Object) error {
		return nil
	}

	// Test successful scan
	stats, err := scanner.Scan("test-bucket", "", fn)
	if err != nil || stats == nil || stats.Objects != 2 {
		t.Errorf("Scan failed: expected no error and 2 objects, got error %v and stats %v", err, stats)
	}

	// Test non-versioned bucket
	stats, err = scanner.Scan("non-versioned-bucket", "", fn)
	if err == nil || stats != nil {
		t.Errorf("Scan failed: expected an error and nil stats, got error %v and stats %v", err, stats)
	}

	// Test prefixed bucket
	stats, err = scanner.Scan("test-bucket", "prefix-a/", fn)
	if err != nil || stats == nil || stats.Objects != 1 {
		t.Errorf("Scan failed: expected no error and 1 objects, got error %v and stats %v", err, stats)
	}

	// Test with error while scanning
	stats, err = scanner.Scan("test-bucket", "", func(o *S3Object) error {
		if *o.Key == "error" {
			return errors.New("mock error")
		}
		return nil
	})
	if err != nil || stats == nil {
		t.Errorf("Scan failed: expected no error and stats, got error %v and stats %v", err, stats)
	}
}
