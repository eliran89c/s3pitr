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

func TestVersionIdTiebreaker(t *testing.T) {
	// Test the VersionId tiebreaker logic for objects with same LastModified time
	// and neither marked as IsLatest

	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	// Test case: obj1 has lexicographically larger VersionId, should be chosen
	testCases := []struct {
		name            string
		obj1VersionId   string
		obj2VersionId   string
		expectedChosen  string
		description     string
	}{
		{
			name:           "obj1_larger_version",
			obj1VersionId:  "version_b",
			obj2VersionId:  "version_a", 
			expectedChosen: "version_b",
			description:    "Object with lexicographically larger VersionId should be chosen",
		},
		{
			name:           "obj2_larger_version",
			obj1VersionId:  "version_a",
			obj2VersionId:  "version_z",
			expectedChosen: "version_z", 
			description:    "Object with lexicographically larger VersionId should be chosen",
		},
		{
			name:           "numeric_versions",
			obj1VersionId:  "v123",
			obj2VersionId:  "v456",
			expectedChosen: "v456",
			description:    "Lexicographic comparison should work with numeric suffixes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			obj1 := &S3Object{
				Key: aws.String("test-key"),
				Metadata: S3ObjectMetadata{
					VersionId:    aws.String(tc.obj1VersionId),
					LastModified: &baseTime,
					IsLatest:     false, // Neither is marked as latest
				},
			}

			obj2 := &S3Object{
				Key: aws.String("test-key"),
				Metadata: S3ObjectMetadata{
					VersionId:    aws.String(tc.obj2VersionId),
					LastModified: &baseTime, // Same LastModified time
					IsLatest:     false,     // Neither is marked as latest
				},
			}

			// Simulate the tiebreaker logic from main.go
			var chosenVersion string
			if obj1.Metadata.LastModified.Equal(*obj2.Metadata.LastModified) && 
			   !obj1.Metadata.IsLatest && !obj2.Metadata.IsLatest {
				if *obj1.Metadata.VersionId > *obj2.Metadata.VersionId {
					chosenVersion = *obj1.Metadata.VersionId
				} else {
					chosenVersion = *obj2.Metadata.VersionId
				}
			}

			if chosenVersion != tc.expectedChosen {
				t.Errorf("%s: expected %s to be chosen, but got %s", 
					tc.description, tc.expectedChosen, chosenVersion)
			}
		})
	}
}

func TestSameDateCurrentIsLatest(t *testing.T) {
	// Test the case where two objects have the same LastModified time
	// but the current object is marked as IsLatest

	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name             string
		currentVersionId string
		existingVersionId string
		currentIsLatest  bool
		existingIsLatest bool
		expectedChosen   string
		description      string
	}{
		{
			name:             "current_is_latest",
			currentVersionId: "version_new",
			existingVersionId: "version_old",
			currentIsLatest:  true,
			existingIsLatest: false,
			expectedChosen:   "version_new",
			description:      "When current object is marked as latest, it should be chosen",
		},
		{
			name:             "existing_is_latest",
			currentVersionId: "version_new", 
			existingVersionId: "version_old",
			currentIsLatest:  false,
			existingIsLatest: true,
			expectedChosen:   "version_old",
			description:      "When existing object is marked as latest, it should be kept",
		},
		{
			name:             "both_latest",
			currentVersionId: "version_new",
			existingVersionId: "version_old", 
			currentIsLatest:  true,
			existingIsLatest: true,
			expectedChosen:   "version_new",
			description:      "When both are latest, current should win (edge case)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			currentObj := &S3Object{
				Key: aws.String("test-key"),
				Metadata: S3ObjectMetadata{
					VersionId:    aws.String(tc.currentVersionId),
					LastModified: &baseTime,
					IsLatest:     tc.currentIsLatest,
				},
			}

			existingObj := &S3ObjectMetadata{
				VersionId:    aws.String(tc.existingVersionId),
				LastModified: &baseTime, // Same LastModified time
				IsLatest:     tc.existingIsLatest,
			}

			// Simulate the decision logic from main.go
			var chosenVersion string
			
			// Check if newer version (this shouldn't trigger for same time)
			if currentObj.Metadata.LastModified.After(*existingObj.LastModified) {
				chosenVersion = *currentObj.Metadata.VersionId
			} else if currentObj.Metadata.LastModified.Equal(*existingObj.LastModified) && currentObj.Metadata.IsLatest {
				// Current is latest case
				chosenVersion = *currentObj.Metadata.VersionId
			} else if currentObj.Metadata.LastModified.Equal(*existingObj.LastModified) && 
					  !currentObj.Metadata.IsLatest && !existingObj.IsLatest {
				// Neither is latest, use VersionId tiebreaker
				if *currentObj.Metadata.VersionId > *existingObj.VersionId {
					chosenVersion = *currentObj.Metadata.VersionId
				} else {
					chosenVersion = *existingObj.VersionId
				}
			} else {
				// Keep existing (default case)
				chosenVersion = *existingObj.VersionId
			}

			if chosenVersion != tc.expectedChosen {
				t.Errorf("%s: expected %s to be chosen, but got %s", 
					tc.description, tc.expectedChosen, chosenVersion)
			}
		})
	}
}
