package resolve

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var FileExtensions = []string{".json", ".yaml", ".yml"}

func ReadFileContent(filename string) ([]byte, error) {
	if isURL(filename) {
		return readRemoteFileContent(filename)
	}
	return os.ReadFile(filename)
}

//nolint:revive
func ResolveAllFiles(filenames []string, recursive bool) ([]string, error) {
	result := []string{}
	for _, f := range filenames {
		files, err := resolveFilenamesForPatterns(f, recursive)
		if err != nil {
			return nil, fmt.Errorf("error resolving filenames: %w", err)
		}
		result = append(result, files...)
	}
	// Ensure consistent order
	sort.Strings(result)
	return result, nil
}

func resolveFilenamesForPatterns(path string, recursive bool) ([]string, error) {
	var results []string

	// Check if the path is a URL

	if isURL(path) {
		// Add URL directly to results
		results = append(results, path)
	} else if strings.Contains(path, "*") {
		// Handle glob patterns
		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, fmt.Errorf("error resolving glob pattern: %w", err)
		}
		results = append(results, matches...)
	} else {
		// Check if the path is a directory or file
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path: %w", err)
		}

		if info.IsDir() {
			// Walk the directory
			err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() && !recursive && p != path {
					return filepath.SkipDir
				}
				if !d.IsDir() {
					if !ignoreFile(filepath.Clean(p), FileExtensions) {
						results = append(results, filepath.Clean(p))
					}
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("error walking directory: %w", err)
			}
		} else {
			// Only apply the extension filter to files in directories; ignore it for directly specified files.
			results = append(results, filepath.Clean(path))
		}
	}

	// Ensure consistent order
	sort.Strings(results)
	return results, nil
}

func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func ignoreFile(path string, extensions []string) bool {
	if len(extensions) == 0 {
		return false
	}
	ext := filepath.Ext(path)
	for _, s := range extensions {
		if s == ext {
			return false
		}
	}
	return true
}

func readRemoteFileContent(inputURL string) ([]byte, error) {
	// Parse and validate the URL
	parsedURL, err := url.Parse(inputURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL: %s", inputURL)
	}

	// Make the HTTP GET request
	response, err := http.Get(parsedURL.String())
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Check for HTTP errors
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot GET file content from: %s", inputURL)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
