package s3scanner

import (
	"strings"
)

type ExclusionMatcher struct {
	rootExclusions   []string
	objectExclusions []string
}

func NewExclusionMatcher(excludePaths []string, rootPrefixes []string) *ExclusionMatcher {
	if len(excludePaths) == 0 {
		return &ExclusionMatcher{}
	}

	matcher := &ExclusionMatcher{}
	matcher.classifyExclusions(excludePaths, rootPrefixes)
	return matcher
}

func (e *ExclusionMatcher) classifyExclusions(excludePaths []string, rootPrefixes []string) {
	for _, exclude := range excludePaths {
		if isRootLevelExclusion(exclude, rootPrefixes) {
			e.rootExclusions = append(e.rootExclusions, exclude)
		} else {
			e.objectExclusions = append(e.objectExclusions, exclude)
		}
	}
}

func (e *ExclusionMatcher) ShouldSkipRootFolder(folderPrefix string) bool {
	for _, rootExclude := range e.rootExclusions {
		if strings.HasPrefix(folderPrefix, rootExclude) {
			return true
		}
	}
	return false
}

func (e *ExclusionMatcher) ShouldSkipObject(objectKey string) bool {
	for _, objExclude := range e.objectExclusions {
		if strings.HasPrefix(objectKey, objExclude) {
			return true
		}
	}
	return false
}

func isRootLevelExclusion(exclude string, rootPrefixes []string) bool {
	// If no prefixes, check if exclude is a root-level folder
	if len(rootPrefixes) == 0 {
		return strings.Count(exclude, "/") == 1
	}

	// Check against each root prefix
	for _, rootPrefix := range rootPrefixes {
		if rootPrefix == "" {
			// Empty prefix case - root level is any path with exactly one folder level
			if strings.Count(exclude, "/") == 1 {
				return true
			}
		} else {
			// Non-empty prefix case
			if strings.HasPrefix(exclude, rootPrefix) {
				// Get the path after the root prefix
				remainingPath := strings.TrimPrefix(exclude, rootPrefix)

				// Root level exclusion: exactly one more folder level
				if strings.Count(remainingPath, "/") == 1 {
					return true
				}
			}
		}
	}

	return false
}
