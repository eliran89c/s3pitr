# S3 Point-In-Time Report (s3pitr)
S3 Point-In-Time Report (s3pitr) is a tool designed to generate a detailed report of an S3 bucket's state at a specific point in time. The bucket must be versioned to utilize this tool. The generated report can be used in conjunction with AWS S3 Batch Operations to restore the bucket to the desired point in time by copying the files listed in the report.


## Compiling and Usage
### Compiling
To compile the project, make sure you have Go(1.20+) installed on your system. Then, navigate to the project directory and run:

```bash
go build -o s3pitr main.go
```

This will create an executable file named s3pitr in your project directory.

### Usage
To use s3pitr, execute the binary with the required flags:

```bash
./s3pitr -timestamp "2023-01-01T12:00:00" -bucket my-bucket
```

### Flags
* `-timestamp` (required): The target timestamp to which you want to restore the bucket. The format should be 'YYYY-MM-DDTHH:MM:SS'.
* `-bucket` (required): The name of the S3 bucket you want to scan and restore.
* `-maxConcurrentScans`: Maximum number of concurrent folder scans (default: 100).
* `-reportName`: The name of the report file (default: "report.csv").
* `-include-latest`: By default, the latest versions of objects are excluded from the report. Set this flag to true to include the latest versions in the report. This is useful when you want to copy all the files to a new bucket.
* `-include-delete-markers`: Controls whether or not to include objects with delete markers in the report. By default, objects with delete markers are excluded from the report, under the assumption that if a file was deleted before the target time, it's not needed for the restore operation. If you want to include delete markers in the report, set this flag to true.


## AWS S3 Batch Operations
After generating the report, you can use AWS S3 Batch Operations to copy the objects listed in the report to restore the bucket to the desired point in time. Follow the [official AWS documentation](https://docs.aws.amazon.com/AmazonS3/latest/userguide/batch-ops.html) to learn how to perform batch operations on your S3 bucket.