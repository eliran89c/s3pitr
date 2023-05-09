package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/eliran89c/s3pitr/internal/csvutils"
	"github.com/eliran89c/s3pitr/pkg/s3scanner"

	"github.com/briandowns/spinner"
	badger "github.com/dgraph-io/badger/v3"
)

const (
	localDBName = ".s3pitr"
)

var (
	targetRestoreTime  time.Time
	bucketName         string
	reportName         string
	maxConcurrentScans int
	reportFilters      []csvutils.ObjectFilterFunc
)

func main() {
	err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}

	// init writer
	csvFile, err := os.Create(reportName)
	if err != nil {
		log.Fatal("Failed to create CSV file: ", err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Create context and initialize S3 scanner
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal("Failed to load AWS SDK configuration: ", err)
	}

	client := s3.NewFromConfig(cfg)

	scanner, err := s3scanner.NewScanner(client, ctx, maxConcurrentScans)
	if err != nil {
		log.Fatal("Failed to load s3scanner: ", err)
	}

	// Create and configure BadgerDB
	opts := badger.DefaultOptions(localDBName)
	opts.Logger = nil //disable badger logger
	defer os.RemoveAll(localDBName)

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Set spinner
	spinner := spinner.New(spinner.CharSets[32], 100*time.Millisecond)
	spinner.Prefix = fmt.Sprintf("Scanning bucket: %v ", bucketName)
	spinner.Start()
	defer spinner.Stop()

	// Scan S3 bucket and store objects in BadgerDB
	scanResult, err := scanner.Scan(bucketName, func(obj *s3scanner.S3Object) error {
		dbObject := s3scanner.S3ObjectMetadata{}
		keyBytes := []byte(*obj.Key)

		// skip files created after targetTime
		if obj.Metadata.LastModified.After(targetRestoreTime) {
			return nil
		}

		err = db.Update(func(txn *badger.Txn) error {
			item, err := txn.Get(keyBytes)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					// If not exists in the DB, store the current object and continue
					return txn.Set(keyBytes, obj.Metadata.Serialize())
				}
				return err
			}

			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &dbObject)
			})
			if err != nil {
				return err
			}

			// Store the newer version
			if obj.Metadata.LastModified.After(*dbObject.LastModified) {
				return txn.Set(keyBytes, obj.Metadata.Serialize())
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("Error handling key %s: %v\n", *obj.Key, err)
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	// Create csv report
	spinner.Prefix = fmt.Sprintf("Generating reports ")
	if err = csvutils.GenerateReport(writer, db, bucketName, reportFilters...); err != nil {
		log.Fatal("Error creating CSV report: ", err)
	}

	// Print scan results
	log.Println("\n---Statistics---")
	log.Println("Number of Pages: ", scanResult.Pages)
	log.Println("Number of Objects: ", scanResult.Objects)
	log.Printf("Scanning Cost: %0.5f$\n", scanResult.Cost())
}

func parseFlags() error {
	var timestampInput, reportNameInput string
	var includeLatest, includeDeleteMarkers bool

	flagsSet := flag.NewFlagSet("app", flag.ExitOnError)

	flagsSet.StringVar(&timestampInput, "timestamp", "", "Restore target timestamp in the format 'YYYY-MM-DDTHH:MM:SS' (default: now)")
	flagsSet.StringVar(&bucketName, "bucket", "", "The name of the S3 bucket to scan and restore (required)")
	flagsSet.IntVar(&maxConcurrentScans, "maxConcurrentScans", 100, "Maximum number of concurrent folder scans")
	flagsSet.StringVar(&reportNameInput, "reportName", "report.csv", "Name of the report file (default: report.csv)")
	flagsSet.BoolVar(&includeLatest, "include-latest", false, "Include the latest versions of the objects in the report (default: false)")
	flagsSet.BoolVar(&includeDeleteMarkers, "include-delete-markers", false, "Include delete markers in the report (default: false)")

	err := flagsSet.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if bucketName == "" {
		return fmt.Errorf("bucket flags is required")
	}

	if timestampInput == "" {
		// If not provided, use current time (good for inventory reports)
		targetRestoreTime = time.Now()
	} else {
		layout := "2006-01-02T15:04:05" // This is the reference layout for the input format

		targetRestoreTime, err = time.Parse(layout, timestampInput)
		if err != nil {
			return fmt.Errorf("error parsing provided timestamp: %v", err)
		}
	}

	if !strings.HasSuffix(strings.ToLower(reportNameInput), ".csv") {
		reportNameInput += ".csv"
	}
	reportName = reportNameInput

	// Append filter functions based on user input
	if !includeLatest {
		reportFilters = append(reportFilters, csvutils.SkipLatest)
	}
	if !includeDeleteMarkers {
		reportFilters = append(reportFilters, csvutils.SkipDeleteMarkers)
	}

	return nil
}
