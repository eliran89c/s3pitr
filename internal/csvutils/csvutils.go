package csvutils

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/eliran89c/s3pitr/pkg/s3scanner"
)

type ObjectFilterFunc func(key string, metadata *s3scanner.S3ObjectMetadata) bool

func GenerateReport(w *csv.Writer, db *badger.DB, bucketName string, filters ...ObjectFilterFunc) error {
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var objMetadata *s3scanner.S3ObjectMetadata
			objKey := string(item.Key())

			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &objMetadata)
			})
			if err != nil {
				return err
			}

			shouldWrite := true
			for _, filter := range filters {
				if !filter(objKey, objMetadata) {
					shouldWrite = false
					break
				}
			}

			if shouldWrite {
				encodedKey := url.PathEscape(objKey)
				record := []string{bucketName, encodedKey, *objMetadata.VersionId}
				if err := w.Write(record); err != nil {
					return fmt.Errorf("failed to write record: %v", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to process items: %v", err)
	}

	return nil
}

func SkipDeleteMarkers(key string, metadata *s3scanner.S3ObjectMetadata) bool {
	return !metadata.IsDeleteMarker
}

func SkipLatest(key string, metadata *s3scanner.S3ObjectMetadata) bool {
	return !metadata.IsLatest
}
