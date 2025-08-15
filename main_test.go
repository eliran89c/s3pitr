package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestPathListSet(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty_string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single_path_no_slash",
			input:    "logs",
			expected: []string{"logs/"},
		},
		{
			name:     "single_path_with_trailing_slash",
			input:    "logs/",
			expected: []string{"logs/"},
		},
		{
			name:     "single_path_with_leading_slash",
			input:    "/logs",
			expected: []string{"logs/"},
		},
		{
			name:     "single_path_with_both_slashes",
			input:    "/logs/",
			expected: []string{"logs/"},
		},
		{
			name:     "multiple_paths_basic",
			input:    "logs,data,cache",
			expected: []string{"logs/", "data/", "cache/"},
		},
		{
			name:     "multiple_paths_with_slashes",
			input:    "/logs/,/data/,cache/",
			expected: []string{"logs/", "data/", "cache/"},
		},
		{
			name:     "paths_with_spaces",
			input:    " logs , data , cache ",
			expected: []string{"logs/", "data/", "cache/"},
		},
		{
			name:     "paths_with_leading_slashes_and_spaces",
			input:    " /logs/ , /data , cache/ ",
			expected: []string{"logs/", "data/", "cache/"},
		},
		{
			name:     "nested_paths",
			input:    "app/logs,app/data/temp,config/prod",
			expected: []string{"app/logs/", "app/data/temp/", "config/prod/"},
		},
		{
			name:     "nested_paths_with_slashes",
			input:    "/app/logs/,/app/data/temp/,/config/prod/",
			expected: []string{"app/logs/", "app/data/temp/", "config/prod/"},
		},
		{
			name:     "empty_paths_in_list",
			input:    "logs,,data,",
			expected: []string{"logs/", "data/"},
		},
		{
			name:     "empty_paths_with_spaces",
			input:    "logs, ,data, , ",
			expected: []string{"logs/", "data/"},
		},
		{
			name:     "just_slash_with_valid_paths",
			input:    "logs,/,data",
			expected: []string{"/"},
		},
		{
			name:     "complex_mixed_case",
			input:    " /app/logs/ , , /data , cache/ , / , temp ",
			expected: []string{"/"},
		},
		{
			name:     "single_character_paths",
			input:    "a,b,c",
			expected: []string{"a/", "b/", "c/"},
		},
		{
			name:     "single_character_with_slashes",
			input:    "/a/,/b/,/c/",
			expected: []string{"a/", "b/", "c/"},
		},
		{
			name:     "deep_nested_path",
			input:    "very/deep/nested/folder/structure",
			expected: []string{"very/deep/nested/folder/structure/"},
		},
		{
			name:     "deep_nested_with_leading_slash",
			input:    "/very/deep/nested/folder/structure/",
			expected: []string{"very/deep/nested/folder/structure/"},
		},
		{
			name:     "only_commas",
			input:    ",,,",
			expected: []string{},
		},
		{
			name:     "only_spaces_and_commas",
			input:    " , , , ",
			expected: []string{},
		},
		{
			name:     "paths_with_special_chars",
			input:    "logs-2023,data_backup,cache.tmp",
			expected: []string{"logs-2023/", "data_backup/", "cache.tmp/"},
		},
		{
			name:     "paths_with_numbers",
			input:    "logs2023,data123,cache456",
			expected: []string{"logs2023/", "data123/", "cache456/"},
		},
		{
			name:     "tabs_and_newlines",
			input:    "\tlogs\t,\ndata\n,cache",
			expected: []string{"logs/", "data/", "cache/"},
		},
		{
			name:     "single_slash",
			input:    "/",
			expected: []string{"/"},
		},
		{
			name:     "double_slash",
			input:    "//",
			expected: []string{"/"},
		},
		{
			name:     "multiple_slashes",
			input:    "/,//,///",
			expected: []string{"/"},
		},
		{
			name:     "slash_with_other_paths",
			input:    "logs,/,data",
			expected: []string{"/"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pathList PathList
			err := pathList.Set(tc.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			result := []string(pathList)

			// Handle nil vs empty slice comparison
			if len(result) == 0 && len(tc.expected) == 0 {
				// Both are empty, consider them equal
			} else if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}

			// Additional verification - ensure all non-slash results end with "/"
			for _, path := range result {
				if path != "" && path != "/" && path != "//" && !strings.HasSuffix(path, "/") {
					t.Errorf("Path %s should end with '/'", path)
				}
			}
		})
	}
}

func TestPathListSetProperties(t *testing.T) {
	testInputs := []string{
		"logs",
		"/logs/",
		" logs ",
		"logs,data",
		"/logs/,/data/",
		" /logs/ , /data/ ",
		"",
		",",
		" , , ",
	}

	for _, input := range testInputs {
		var pathList PathList
		err := pathList.Set(input)
		if err != nil {
			t.Errorf("Unexpected error for input %q: %v", input, err)
		}

		result := []string(pathList)

		// Property 1: All non-empty results should end with "/"
		for _, path := range result {
			if path != "" && !strings.HasSuffix(path, "/") {
				t.Errorf("Input %q produced path %q that doesn't end with '/'", input, path)
			}
		}

		// Property 2: No results should be empty strings
		for _, path := range result {
			if path == "" {
				t.Errorf("Input %q produced empty string in result", input)
			}
		}
	}
}

func TestPathListMultipleSet(t *testing.T) {
	var pathList PathList

	pathList.Set("logs")
	pathList.Set("data")

	expected := []string{"logs/", "data/"}
	result := []string(pathList)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Multiple Set calls: expected %v, got %v", expected, result)
	}
}

func TestPathListRootOverride(t *testing.T) {
	var pathList PathList

	pathList.Set("logs")
	pathList.Set("data")
	pathList.Set("/")

	expected := []string{"/"}
	result := []string(pathList)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Root override: expected %v, got %v", expected, result)
	}

	pathList.Set("more")
	result = []string(pathList)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("After root set, additional Set should be ignored: expected %v, got %v", expected, result)
	}
}
