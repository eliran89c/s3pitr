package s3scanner

import (
	"testing"
)

func TestNewExclusionMatcher(t *testing.T) {
	testCases := []struct {
		name         string
		excludePaths []string
		rootPrefixes []string
		expected     *ExclusionMatcher
	}{
		{
			name:         "empty_exclusions",
			excludePaths: []string{},
			rootPrefixes: []string{},
			expected:     &ExclusionMatcher{},
		},
		{
			name:         "single_root_exclusion",
			excludePaths: []string{"logs/"},
			rootPrefixes: []string{""},
			expected: &ExclusionMatcher{
				rootExclusions: []string{"logs/"},
			},
		},
		{
			name:         "single_object_exclusion",
			excludePaths: []string{"app/cache/temp/"},
			rootPrefixes: []string{""},
			expected: &ExclusionMatcher{
				objectExclusions: []string{"app/cache/temp/"},
			},
		},
		{
			name:         "mixed_exclusions",
			excludePaths: []string{"logs/", "app/cache/temp/"},
			rootPrefixes: []string{""},
			expected: &ExclusionMatcher{
				rootExclusions:   []string{"logs/"},
				objectExclusions: []string{"app/cache/temp/"},
			},
		},
		{
			name:         "with_root_prefix",
			excludePaths: []string{"a/b/c/logs/", "a/b/c/data/cache/temp/"},
			rootPrefixes: []string{"a/b/c/"},
			expected: &ExclusionMatcher{
				rootExclusions:   []string{"a/b/c/logs/"},
				objectExclusions: []string{"a/b/c/data/cache/temp/"},
			},
		},
		{
			name:         "bucket_level_exclusion",
			excludePaths: []string{"prefix-a/", "prefix-b/logs/"},
			rootPrefixes: []string{"prefix-a/", "prefix-b/"},
			expected: &ExclusionMatcher{
				bucketExclusions: []string{"prefix-a/"},
				rootExclusions:   []string{"prefix-b/logs/"},
			},
		},
		{
			name:         "mixed_with_bucket_exclusions",
			excludePaths: []string{"prefix-a/", "prefix-b/logs/", "prefix-b/data/cache/temp/"},
			rootPrefixes: []string{"prefix-a/", "prefix-b/"},
			expected: &ExclusionMatcher{
				bucketExclusions: []string{"prefix-a/"},
				rootExclusions:   []string{"prefix-b/logs/"},
				objectExclusions: []string{"prefix-b/data/cache/temp/"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NewExclusionMatcher(tc.excludePaths, tc.rootPrefixes)

			if len(result.bucketExclusions) != len(tc.expected.bucketExclusions) {
				t.Errorf("Expected %d bucket exclusions, got %d", len(tc.expected.bucketExclusions), len(result.bucketExclusions))
			}

			if len(result.rootExclusions) != len(tc.expected.rootExclusions) {
				t.Errorf("Expected %d root exclusions, got %d", len(tc.expected.rootExclusions), len(result.rootExclusions))
			}

			if len(result.objectExclusions) != len(tc.expected.objectExclusions) {
				t.Errorf("Expected %d object exclusions, got %d", len(tc.expected.objectExclusions), len(result.objectExclusions))
			}

			for i, expected := range tc.expected.bucketExclusions {
				if i >= len(result.bucketExclusions) || result.bucketExclusions[i] != expected {
					t.Errorf("Expected bucket exclusion %s, got %s", expected, result.bucketExclusions[i])
				}
			}

			for i, expected := range tc.expected.rootExclusions {
				if i >= len(result.rootExclusions) || result.rootExclusions[i] != expected {
					t.Errorf("Expected root exclusion %s, got %s", expected, result.rootExclusions[i])
				}
			}

			for i, expected := range tc.expected.objectExclusions {
				if i >= len(result.objectExclusions) || result.objectExclusions[i] != expected {
					t.Errorf("Expected object exclusion %s, got %s", expected, result.objectExclusions[i])
				}
			}
		})
	}
}

func TestIsRootLevelExclusion(t *testing.T) {
	testCases := []struct {
		name         string
		exclude      string
		rootPrefixes []string
		expected     bool
	}{
		{
			name:         "no_prefix_root_level",
			exclude:      "logs/",
			rootPrefixes: []string{""},
			expected:     true,
		},
		{
			name:         "no_prefix_non_root_level",
			exclude:      "app/cache/",
			rootPrefixes: []string{""},
			expected:     false,
		},
		{
			name:         "with_prefix_root_level",
			exclude:      "a/b/c/logs/",
			rootPrefixes: []string{"a/b/c/"},
			expected:     true,
		},
		{
			name:         "with_prefix_non_root_level",
			exclude:      "a/b/c/app/cache/",
			rootPrefixes: []string{"a/b/c/"},
			expected:     false,
		},
		{
			name:         "exclude_not_under_prefix",
			exclude:      "different/path/",
			rootPrefixes: []string{"a/b/c/"},
			expected:     false,
		},
		{
			name:         "multiple_prefixes_match_first",
			exclude:      "a/b/c/logs/",
			rootPrefixes: []string{"a/b/c/", "x/y/z/"},
			expected:     true,
		},
		{
			name:         "multiple_prefixes_match_second",
			exclude:      "x/y/z/logs/",
			rootPrefixes: []string{"a/b/c/", "x/y/z/"},
			expected:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isRootLevelExclusion(tc.exclude, tc.rootPrefixes)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t for exclude=%s, rootPrefixes=%v", tc.expected, result, tc.exclude, tc.rootPrefixes)
			}
		})
	}
}

func TestShouldSkipRootFolder(t *testing.T) {
	testCases := []struct {
		name         string
		excludePaths []string
		rootPrefixes []string
		folderPrefix string
		expected     bool
	}{
		{
			name:         "skip_root_exclusion",
			excludePaths: []string{"logs/"},
			rootPrefixes: []string{""},
			folderPrefix: "logs/",
			expected:     true,
		},
		{
			name:         "skip_subfolder_of_root_exclusion",
			excludePaths: []string{"logs/"},
			rootPrefixes: []string{""},
			folderPrefix: "logs/error/",
			expected:     true,
		},
		{
			name:         "dont_skip_different_folder",
			excludePaths: []string{"logs/"},
			rootPrefixes: []string{""},
			folderPrefix: "data/",
			expected:     false,
		},
		{
			name:         "no_exclusions",
			excludePaths: []string{},
			rootPrefixes: []string{""},
			folderPrefix: "logs/",
			expected:     false,
		},
		{
			name:         "object_level_exclusion_not_skipped",
			excludePaths: []string{"app/cache/temp/"},
			rootPrefixes: []string{""},
			folderPrefix: "app/",
			expected:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewExclusionMatcher(tc.excludePaths, tc.rootPrefixes)
			result := matcher.ShouldSkipRootFolder(tc.folderPrefix)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t for folderPrefix=%s", tc.expected, result, tc.folderPrefix)
			}
		})
	}
}

func TestIsBucketLevelExclusion(t *testing.T) {
	testCases := []struct {
		name         string
		exclude      string
		rootPrefixes []string
		expected     bool
	}{
		{
			name:         "exclude_matches_root_prefix",
			exclude:      "prefix-a/",
			rootPrefixes: []string{"prefix-a/", "prefix-b/"},
			expected:     true,
		},
		{
			name:         "exclude_not_in_root_prefixes",
			exclude:      "prefix-c/",
			rootPrefixes: []string{"prefix-a/", "prefix-b/"},
			expected:     false,
		},
		{
			name:         "empty_root_prefixes",
			exclude:      "prefix-a/",
			rootPrefixes: []string{},
			expected:     false,
		},
		{
			name:         "single_root_prefix_match",
			exclude:      "data/",
			rootPrefixes: []string{"data/"},
			expected:     true,
		},
		{
			name:         "exact_match_required",
			exclude:      "prefix-a/logs/",
			rootPrefixes: []string{"prefix-a/"},
			expected:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isBucketLevelExclusion(tc.exclude, tc.rootPrefixes)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t for exclude=%s, rootPrefixes=%v", tc.expected, result, tc.exclude, tc.rootPrefixes)
			}
		})
	}
}

func TestShouldSkipBucket(t *testing.T) {
	testCases := []struct {
		name         string
		excludePaths []string
		rootPrefixes []string
		bucketName   string
		expected     bool
	}{
		{
			name:         "skip_excluded_bucket",
			excludePaths: []string{"prefix-a/", "logs/"},
			rootPrefixes: []string{"prefix-a/", "prefix-b/"},
			bucketName:   "prefix-a/",
			expected:     true,
		},
		{
			name:         "dont_skip_allowed_bucket",
			excludePaths: []string{"prefix-a/"},
			rootPrefixes: []string{"prefix-a/", "prefix-b/"},
			bucketName:   "prefix-b/",
			expected:     false,
		},
		{
			name:         "no_bucket_exclusions",
			excludePaths: []string{"logs/"},
			rootPrefixes: []string{""},
			bucketName:   "any-bucket",
			expected:     false,
		},
		{
			name:         "empty_exclusions",
			excludePaths: []string{},
			rootPrefixes: []string{"prefix-a/"},
			bucketName:   "prefix-a/",
			expected:     false,
		},
		{
			name:         "multiple_bucket_exclusions",
			excludePaths: []string{"prefix-a/", "prefix-c/", "logs/"},
			rootPrefixes: []string{"prefix-a/", "prefix-b/", "prefix-c/"},
			bucketName:   "prefix-c/",
			expected:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewExclusionMatcher(tc.excludePaths, tc.rootPrefixes)
			result := matcher.ShouldSkipBucket(tc.bucketName)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t for bucketName=%s", tc.expected, result, tc.bucketName)
			}
		})
	}
}

func TestShouldSkipObject(t *testing.T) {
	testCases := []struct {
		name         string
		excludePaths []string
		rootPrefixes []string
		objectKey    string
		expected     bool
	}{
		{
			name:         "skip_object_in_excluded_path",
			excludePaths: []string{"app/cache/temp/"},
			rootPrefixes: []string{""},
			objectKey:    "app/cache/temp/file.txt",
			expected:     true,
		},
		{
			name:         "dont_skip_object_in_allowed_path",
			excludePaths: []string{"app/cache/temp/"},
			rootPrefixes: []string{""},
			objectKey:    "app/data/file.txt",
			expected:     false,
		},
		{
			name:         "no_exclusions",
			excludePaths: []string{},
			rootPrefixes: []string{""},
			objectKey:    "any/file.txt",
			expected:     false,
		},
		{
			name:         "root_exclusion_not_filtered_here",
			excludePaths: []string{"logs/"},
			rootPrefixes: []string{""},
			objectKey:    "logs/error.log",
			expected:     false, // Root exclusions are filtered at folder level
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewExclusionMatcher(tc.excludePaths, tc.rootPrefixes)
			result := matcher.ShouldSkipObject(tc.objectKey)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t for objectKey=%s", tc.expected, result, tc.objectKey)
			}
		})
	}
}
