package s3scanner

import (
	"sync/atomic"
)

const (
	listObjectPrice = 0.000005 // 0.005 per 1k requests
)

// BucketStatistics contains information about the number of pages and objects processed
// during the scanning operation.
type BucketStatistics struct {
	Pages   uint64
	Objects uint64
}

type bucket struct {
	name    string
	folders chan *bucketFolder
}

type bucketFolder struct {
	prefix    string
	delimiter string
}

func (b *bucket) addFolder(prefix string) {
	b.folders <- &bucketFolder{prefix: prefix}
}

func (b *bucket) closeFolders() {
	close(b.folders)
}

func (folder *bucketFolder) isRoot() bool {
	return folder.prefix == "" && folder.delimiter == "/"
}

func (stats *BucketStatistics) addPages(p int) {
	atomic.AddUint64(&stats.Pages, uint64(p))
}

func (stats *BucketStatistics) addObjects(o int) {
	atomic.AddUint64(&stats.Objects, uint64(o))
}

// Cost calculates the cost of the scanning operation based on the number of pages processed.
// It returns the cost as a float32.
func (stats *BucketStatistics) Cost() float32 {
	return float32(stats.Pages) * listObjectPrice
}

func newBucket(name string) *bucket {
	b := &bucket{
		name:    name,
		folders: make(chan *bucketFolder, 1),
	}
	b.folders <- &bucketFolder{delimiter: "/"} // manually add the root folder
	return b
}
