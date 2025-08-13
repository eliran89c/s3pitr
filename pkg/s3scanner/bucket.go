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
	name             string
	root             string
	folders          chan *bucketFolder
	exclusionMatcher *ExclusionMatcher
}

type bucketFolder struct {
	prefix    string
	delimiter string
}

func (b *bucket) addFolder(prefix string) {
	if b.exclusionMatcher != nil && b.exclusionMatcher.ShouldSkipRootFolder(prefix) {
		return
	}
	b.folders <- &bucketFolder{prefix: prefix}
}

func (b *bucket) closeFolders() {
	close(b.folders)
}

func (b *bucket) isRoot(f *bucketFolder) bool {
	return f.prefix == b.root && f.delimiter == "/"
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

func newBucket(name, prefix string) *bucket {
	b := &bucket{
		name:    name,
		root:    prefix,
		folders: make(chan *bucketFolder, 1),
	}
	b.folders <- &bucketFolder{delimiter: "/", prefix: prefix} // manually add the root folder
	return b
}

func newBucketWithExclusions(name, prefix string, exclusionMatcher *ExclusionMatcher) *bucket {
	b := &bucket{
		name:             name,
		root:             prefix,
		folders:          make(chan *bucketFolder, 1),
		exclusionMatcher: exclusionMatcher,
	}
	b.folders <- &bucketFolder{delimiter: "/", prefix: prefix} // manually add the root folder
	return b
}
