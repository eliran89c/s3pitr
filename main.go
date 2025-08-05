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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/eliran89c/s3pitr/internal/csvutils"
	"github.com/eliran89c/s3pitr/pkg/s3scanner"

	"github.com/briandowns/spinner"
	badger "github.com/dgraph-io/badger/v3"
)

const (
	localDBName = ".s3pitr"
)

var (
	startTime          = time.Now()
	targetRestoreTime  time.Time
	bucketName         string
	reportName         string
	prefix             string
	maxConcurrentScans int
	reportFilters      []csvutils.ObjectFilterFunc
	profile            string
	region             string
	roleArn            string

	version = "dev"
	arch    = "dev"
)

func main() {
	err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}

	csvFile, err := os.Create(reportName)
	if err != nil {
		log.Fatal("Failed to create CSV file: ", err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	ctx := context.Background()

	cfg, err := getClientConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	client := s3.NewFromConfig(cfg)

	scanner, err := s3scanner.NewScanner(client, ctx, maxConcurrentScans)
	if err != nil {
		log.Fatal("Failed to load s3scanner: ", err)
	}

	opts := badger.DefaultOptions(localDBName)
	opts.Logger = nil //disable badger logger
	defer os.RemoveAll(localDBName)

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	spinner := spinner.New(spinner.CharSets[32], 100*time.Millisecond)
	if prefix == "" {
		spinner.Prefix = fmt.Sprintf("Scanning bucket: %v ", bucketName)
	} else {
		spinner.Prefix = fmt.Sprintf("Scanning bucket: %v with prefix: %v", bucketName, prefix)
	}
	spinner.Start()

	// Scan S3 bucket and store objects in BadgerDB
	scanResult, err := scanner.Scan(bucketName, prefix, func(obj *s3scanner.S3Object) error {
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

			if obj.Metadata.LastModified.Equal(*dbObject.LastModified) && obj.Metadata.IsLatest {
				// If the last modified time is equal and the current object is marked as latest,
				// we update the existing object to be the latest version.
				return txn.Set(keyBytes, obj.Metadata.Serialize())
			}

			if obj.Metadata.LastModified.Equal(*dbObject.LastModified) && !obj.Metadata.IsLatest && !dbObject.IsLatest {
				// If both objects have the same last modified time and neither is marked as latest,
				// use VersionId lexicographic comparison as a consistent tiebreaker.
				if *obj.Metadata.VersionId > *dbObject.VersionId {
					return txn.Set(keyBytes, obj.Metadata.Serialize())
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("error handling key %s: %v", *obj.Key, err)
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	spinner.Prefix = fmt.Sprintln("Generating reports")
	if err = csvutils.GenerateReport(writer, db, bucketName, reportFilters...); err != nil {
		log.Fatal("Error creating CSV report: ", err)
	}

	spinner.Stop()

	// Print scan results
	fmt.Println("---Statistics---")
	fmt.Println("Number of Pages: ", scanResult.Pages)
	fmt.Println("Number of Objects: ", scanResult.Objects)
	fmt.Printf("Scanning Cost: %0.5f$\n", scanResult.Cost())
	fmt.Printf("Execution Time: %s\n", time.Since(startTime).Round(time.Second))
}

func parseFlags() error {
	var timestampInput, reportNameInput string
	var includeLatest, includeDeleteMarkers, printVer bool

	flagsSet := flag.NewFlagSet("app", flag.ExitOnError)

	flagsSet.StringVar(&timestampInput, "timestamp", "", "Restore target timestamp in the format 'YYYY-MM-DDTHH:MM:SS' (default: now)")
	flagsSet.StringVar(&bucketName, "bucket", "", "The name of the S3 bucket to scan and restore (required)")
	flagsSet.IntVar(&maxConcurrentScans, "maxConcurrentScans", 100, "Maximum number of concurrent folder scans")
	flagsSet.StringVar(&reportNameInput, "reportName", "report.csv", "Name of the report file (default: report.csv)")
	flagsSet.BoolVar(&includeLatest, "include-latest", false, "Include the latest versions of the objects in the report (default: false)")
	flagsSet.BoolVar(&includeDeleteMarkers, "include-delete-markers", false, "Include delete markers in the report (default: false)")
	flagsSet.StringVar(&prefix, "prefix", "", "Prefix to filter objects in the report (default: all objects)")
	flagsSet.StringVar(&profile, "profile", "", "AWS profile to use for credentials")
	flagsSet.StringVar(&region, "region", "", "AWS region to use")
	flagsSet.StringVar(&roleArn, "role-arn", "", "AWS IAM role ARN to assume")
	flagsSet.BoolVar(&printVer, "version", false, "Print version information")

	err := flagsSet.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if printVer {
		fmt.Println("s3pitr")
		fmt.Println("Version:", version)
		fmt.Println("Architecture:", arch)
		os.Exit(0)
	}

	if prefix != "" {
		prefix = strings.TrimPrefix(prefix, "/")

		if len(prefix) > 0 && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
	}

	if bucketName == "" {
		return fmt.Errorf("bucket flags is required")
	}

	if timestampInput == "" {
		// If not provided, use current time (good for inventory reports)
		targetRestoreTime = startTime
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

	if !includeLatest {
		reportFilters = append(reportFilters, csvutils.SkipLatest)
	}
	if !includeDeleteMarkers {
		reportFilters = append(reportFilters, csvutils.SkipDeleteMarkers)
	}

	return nil
}

func getClientConfig(ctx context.Context) (aws.Config, error) {
	var options []func(*config.LoadOptions) error

	if profile != "" {
		options = append(options, config.WithSharedConfigProfile(profile))
	}

	if region != "" {
		options = append(options, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS SDK configuration: %w", err)
	}

	if roleArn != "" {
		stsClient := sts.NewFromConfig(cfg)
		stsCreds := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
		cfg.Credentials = aws.NewCredentialsCache(stsCreds)
	}

	return cfg, nil
}
