package s3scanner

import (
	"encoding/json"
	"time"
)

// S3Object represents an object within an S3 bucket.
type S3Object struct {
	Key      *string
	Metadata S3ObjectMetadata
}

// S3ObjectMetadata holds metadata associated with an S3Object.
type S3ObjectMetadata struct {
	VersionId      *string
	LastModified   *time.Time
	IsDeleteMarker bool
	IsLatest       bool
}

// Serialize returns the JSON byte representation of an S3ObjectMetadata instance, ignoring any marshaling errors.
func (o *S3ObjectMetadata) Serialize() []byte {
	m, _ := json.Marshal(o)
	return m
}
