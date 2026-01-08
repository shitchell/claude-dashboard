package util

import (
	"net/url"
	"path/filepath"
	"strings"
)

// DecodeProjectPath decodes a Claude Code project directory name back to
// the original filesystem path.
//
// Claude Code stores sessions in directories named after the project path,
// with the path URL-encoded. For example:
//
//	~/.claude/projects/-home-user-code-myproject/
//
// The directory name "-home-user-code-myproject" encodes the path
// "/home/user/code/myproject".
//
// The encoding scheme is:
//   - "/" is replaced with "-"
//   - Special characters are URL-encoded (percent-encoded)
//
// This function reverses that encoding to recover the original path.
func DecodeProjectPath(encodedPath string) (string, error) {
	// Replace leading dash and subsequent dashes with slashes
	// The encoding uses "-" as a separator for path components
	decoded := strings.ReplaceAll(encodedPath, "-", "/")

	// URL-decode any percent-encoded characters
	decoded, err := url.PathUnescape(decoded)
	if err != nil {
		return "", err
	}

	// Clean the path to normalize any double slashes or other oddities
	decoded = filepath.Clean(decoded)

	return decoded, nil
}

// ProjectNameFromPath extracts the base project name from a full path.
// For example, "/home/user/code/myproject" returns "myproject".
func ProjectNameFromPath(path string) string {
	return filepath.Base(path)
}
