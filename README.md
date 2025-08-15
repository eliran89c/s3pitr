# S3 Point-In-Time Report (s3pitr)
S3 Point-In-Time Report (s3pitr) is a tool designed to generate a detailed report of an S3 bucket's state at a specific point in time. The bucket must be versioned to utilize this tool. The generated report can be used in conjunction with AWS S3 Batch Operations to restore the bucket to the desired point in time by copying the files listed in the report.


## Motivation
s3pitr was developed to address the need for an efficient, fast, and easy-to-use tool for generating Point-In-Time Reports (PITR) for Amazon S3 buckets. Such a tool is particularly useful in scenarios involving ransomware attacks or accidental deletions. By inputting a timestamp, s3pitr creates a manifest that is compatible with AWS Batch Operations, enabling users to restore an S3 bucket to its desired state by copying or recreating the necessary objects.


Before the creation of s3pitr, the following alternatives were examined:


1. **AWS Inventory reports:** Generating these reports can take over 24 hours, making them unsuitable for situations where a shorter Recovery Time Objective (RTO) is required. In some cases, waiting for a lengthy period between reports is not feasible. Furthermore, creating a manifest of relevant objects for restoration requires significant manual effort in CSV manipulation.
2. **AWS blog article on PITR for S3 buckets:** [This article](https://aws.amazon.com/blogs/storage/point-in-time-restore-for-amazon-s3-buckets/) suggests an alternative with a tighter RTO and PITR report generation. However, the method is more complicated, involving multiple steps and components. It also requires ongoing monitoring and management of operations and events after object creation.


s3pitr overcomes these limitations by offering a straightforward, fast, and effective solution for generating PITR reports for S3 buckets, especially in situations where shorter RTOs are crucial.


## Installation and Usage

### Requirements
Before using s3pitr, make sure you meet the following requirements:
1. **AWS Permissions:** To run s3pitr, your AWS account must have sufficient permissions. The following AWS Identity and Access Management (IAM) permissions are required:
    * `s3:GetBucketVersioning`: This permission is necessary to retrieve the versioning status of the specified S3 bucket.
    * `s3:ListObjectVersions`: This permission is required to list the object versions in the specified S3 bucket.

## Installation

You have several options to install s3pitr:

### Option 1: macOS (Using Homebrew)

If you're on macOS, you can use Homebrew to install s3pitr:

```
brew install eliran89c/tap/s3pitr
```

### Option 2: Pre-built Binaries (Linux and Windows)

You can download pre-built binaries for Linux and Windows from the [releases page](https://github.com/eliran89c/s3pitr/releases) on GitHub. Choose the appropriate binary for your operating system and architecture.

### Option 3: Using Go

You can install s3pitr directly from GitHub using Go:

```
go install github.com/eliran89c/s3pitr@latest
```

This will download the source code, compile it, and install the `s3pitr` binary in your `$GOPATH/bin` directory. Make sure your `$GOPATH/bin` is in your system's PATH.

### Usage
To use s3pitr, execute the binary with the required flags:

```bash
# Basic usage - scan entire bucket
s3pitr -timestamp "2023-01-01T12:00:00" -bucket my-bucket

# Scan specific prefixes only
s3pitr -timestamp "2023-01-01T12:00:00" -bucket my-bucket -prefix logs -prefix data

# Scan entire bucket but exclude certain paths
s3pitr -timestamp "2023-01-01T12:00:00" -bucket my-bucket -exclude temp -exclude cache

# Combine prefixes and exclusions
s3pitr -timestamp "2023-01-01T12:00:00" -bucket my-bucket -prefix app -exclude app/temp -exclude app/logs
```

### Flags
* `-timestamp` (required): The target timestamp to which you want to restore the bucket. The format should be 'YYYY-MM-DDTHH:MM:SS'.
* `-bucket` (required): The name of the S3 bucket you want to scan and restore.
* `-maxConcurrentScans`: Maximum number of concurrent folder scans (default: 100).
* `-reportName`: The name of the report file (default: "report.csv").
* `-include-latest`: By default, the latest versions of objects are excluded from the report. Set this flag to true to include the latest versions in the report. This is useful when you want to copy all the files to a new bucket.
* `-include-delete-markers`: Controls whether or not to include objects with delete markers in the report. By default, objects with delete markers are excluded from the report, under the assumption that if a file was deleted before the target time, it's not needed for the restore operation. If you want to include delete markers in the report, set this flag to true.
* `-prefix`: Prefix to filter objects in the report. Can be specified multiple times to scan multiple prefixes (e.g., `-prefix logs -prefix data`). By default `s3pitr` will scan the entire bucket.
* `-exclude`: Paths to exclude from scanning. Can be specified multiple times to exclude multiple paths (e.g., `-exclude temp -exclude cache`). Use this to skip specific folders or objects during the scan.
* `-profile`: The AWS profile to use for credentials.
* `-region`: AWS region to use.
* `-role-arn`: AWS IAM role ARN to assume.
* `-version`: Print s3pitr version.


## AWS S3 Batch Operations
After generating the report, you can use AWS S3 Batch Operations to copy the objects listed in the report to restore the bucket to the desired point in time. Follow the [official AWS documentation](https://docs.aws.amazon.com/AmazonS3/latest/userguide/batch-ops.html) to learn how to perform batch operations on your S3 bucket.
