package s3scanner

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3ClientAPI is an interface that defines a minimal set of methods required
// from the S3 client for the Scanner to function. This interface makes it
// easier to use a mock client for testing purposes without relying on
// an actual S3 service.
type S3ClientAPI interface {
	ListObjectVersions(ctx context.Context, params *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
}

// Scanner is a concurrent S3 bucket scanner.
type Scanner struct {
	ctx        context.Context
	client     S3ClientAPI
	logger     *log.Logger
	workerPool chan struct{}
}

// NonVersionedBucketError is an error type representing an error
// encountered when attempting to scan a non-versioned S3 bucket.
// This error is returned by the Scan function if the specified bucket
// does not have versioning enabled.
type NonVersionedBucketError struct {
	BucketName string
}

func (e *NonVersionedBucketError) Error() string {
	return fmt.Sprintf("Bucket %s is not versioned", e.BucketName)
}

func (s *Scanner) acquireWorker() {
	<-s.workerPool
}

func (s *Scanner) releaseWorker() {
	s.workerPool <- struct{}{}
}

// SetLogger allows users to set a custom logger for the Scanner instance.
// The provided logger will be used for logging messages during the scanning process.
func (s *Scanner) SetLogger(logger *log.Logger) {
	s.logger = logger
}

// NewScanner creates a new Scanner instance with the specified context and maximum number of concurrent folder workers.
// It returns a pointer to the Scanner and an error if any occurred.
func NewScanner(s3Client S3ClientAPI, ctx context.Context, maxConcurrentScans int) (*Scanner, error) {

	// init worker pool
	workerPool := make(chan struct{}, maxConcurrentScans)
	for i := 0; i < maxConcurrentScans; i++ {
		workerPool <- struct{}{}
	}

	return &Scanner{
		ctx:        ctx,
		client:     s3Client,
		workerPool: workerPool,
		logger:     log.New(ioutil.Discard, "", 0),
	}, nil
}

func (s *Scanner) fetchFolder(b *bucket, folder *bucketFolder, objCh chan<- *S3Object) (int, error) {
	var nextKey, nextVersion *string
	pageCount := 0

	for {
		pageCount++
		resp, err := s.client.ListObjectVersions(s.ctx, &s3.ListObjectVersionsInput{
			Bucket:          aws.String(b.name),
			KeyMarker:       nextKey,
			VersionIdMarker: nextVersion,
			Prefix:          aws.String(folder.prefix),
			Delimiter:       aws.String(folder.delimiter),
		})

		if err != nil {
			return pageCount, fmt.Errorf("failed to list object versions for bucket %s with prefix %s: %v", b.name, folder.prefix, err)
		}

		for _, del := range resp.DeleteMarkers {
			objCh <- &S3Object{
				Key: del.Key,
				Metadata: S3ObjectMetadata{
					VersionId:      del.VersionId,
					LastModified:   del.LastModified,
					IsDeleteMarker: true,
					IsLatest:       *del.IsLatest,
				},
			}
		}

		for _, ver := range resp.Versions {
			objCh <- &S3Object{
				Key: ver.Key,
				Metadata: S3ObjectMetadata{
					VersionId:      ver.VersionId,
					LastModified:   ver.LastModified,
					IsLatest:       *ver.IsLatest,
					IsDeleteMarker: false,
				},
			}
		}

		for _, commonPrefix := range resp.CommonPrefixes {
			b.addFolder(*commonPrefix.Prefix)
		}

		if !*resp.IsTruncated {
			break
		}

		nextKey = resp.NextKeyMarker
		nextVersion = resp.NextVersionIdMarker
	}
	return pageCount, nil
}

// Scan performs a concurrent scan of the specified S3 bucket, processing each object using the provided function.
// It returns a pointer to a BucketStatistics instance containing the number of pages and objects processed, and an error if any occurred.
func (s *Scanner) Scan(bucketName, prefix string, fn func(o *S3Object) error) (*BucketStatistics, error) {

	// check if the bucket is versioned
	res, err := s.client.GetBucketVersioning(s.ctx, &s3.GetBucketVersioningInput{
		Bucket: &bucketName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket versioning for %s: %v", bucketName, err)
	}

	// Check if the bucket is versioned
	if res.Status != types.BucketVersioningStatusEnabled {
		return nil, &NonVersionedBucketError{BucketName: bucketName}
	}

	var wg sync.WaitGroup
	stats := new(BucketStatistics)
	b := newBucket(bucketName, prefix)

	for folder := range b.folders {
		s.acquireWorker()
		wg.Add(1)

		go func(folder *bucketFolder) {
			defer s.releaseWorker()

			objCh := make(chan *S3Object)
			defer close(objCh)

			// process objects
			go func(objCh <-chan *S3Object) {
				defer wg.Done()

				// panic recovery
				defer func() {
					if r := recover(); r != nil {
						s.logger.Println("Object processing function panicked:", r)
					}
				}()

				i := 0
				for obj := range objCh {
					i++
					if err := fn(obj); err != nil {
						s.logger.Println("Error in object processing function:", err)
					}
				}
				stats.addObjects(i)

			}(objCh)

			pages, err := s.fetchFolder(b, folder, objCh)
			stats.addPages(pages)

			if err != nil {
				s.logger.Printf("Failed to fetch prefix '%s': %v\n", folder.prefix, err)
			}

			// close the prefix channel
			if b.isRoot(folder) {
				b.closeFolders()
			}

		}(folder)
	}

	// Wait for all the worker goroutines to finish processing prefixes
	wg.Wait()

	return stats, nil
}
