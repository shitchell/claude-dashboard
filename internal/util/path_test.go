package util

import (
	"testing"
)

func TestDecodeProjectPath(t *testing.T) {
	tests := []struct {
		name        string
		encoded     string
		expected    string
		expectError bool
	}{
		{
			name:        "Simple path with dashes",
			encoded:     "-home-user-code-myproject",
			expected:    "/home/user/code/myproject",
			expectError: false,
		},
		{
			name:        "Root path only",
			encoded:     "-",
			expected:    "/",
			expectError: false,
		},
		{
			name:        "Path with multiple levels",
			encoded:     "-home-user-code-github-repo-subdir",
			expected:    "/home/user/code/github/repo/subdir",
			expectError: false,
		},
		{
			name:        "Path with URL-encoded space",
			encoded:     "-home-user-My%20Documents-project",
			expected:    "/home/user/My Documents/project",
			expectError: false,
		},
		{
			name:        "Path with URL-encoded special characters",
			encoded:     "-home-user-code-project%40v2",
			expected:    "/home/user/code/project@v2",
			expectError: false,
		},
		{
			name:        "Path with percent-encoded hash",
			encoded:     "-home-user-code-project%23test",
			expected:    "/home/user/code/project#test",
			expectError: false,
		},
		{
			name:        "Empty string",
			encoded:     "",
			expected:    ".",
			expectError: false,
		},
		{
			name:        "Single directory",
			encoded:     "-tmp",
			expected:    "/tmp",
			expectError: false,
		},
		{
			name:        "Invalid percent encoding",
			encoded:     "-home-user-code-%ZZ",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeProjectPath(tt.encoded)

			if tt.expectError {
				if err == nil {
					t.Errorf("DecodeProjectPath(%q) expected error, got nil", tt.encoded)
				}
				return
			}

			if err != nil {
				t.Errorf("DecodeProjectPath(%q) unexpected error: %v", tt.encoded, err)
				return
			}

			if result != tt.expected {
				t.Errorf("DecodeProjectPath(%q) = %q, want %q", tt.encoded, result, tt.expected)
			}
		})
	}
}

func TestProjectNameFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Standard path",
			path:     "/home/user/code/myproject",
			expected: "myproject",
		},
		{
			name:     "Path with trailing slash",
			path:     "/home/user/code/myproject/",
			expected: "myproject",
		},
		{
			name:     "Single directory",
			path:     "/myproject",
			expected: "myproject",
		},
		{
			name:     "Root path",
			path:     "/",
			expected: "/",
		},
		{
			name:     "Relative path",
			path:     "code/myproject",
			expected: "myproject",
		},
		{
			name:     "Just filename",
			path:     "myproject",
			expected: "myproject",
		},
		{
			name:     "Path with spaces",
			path:     "/home/user/My Projects/my project",
			expected: "my project",
		},
		{
			name:     "Empty string",
			path:     "",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProjectNameFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("ProjectNameFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestDecodeProjectPathAndExtractName(t *testing.T) {
	// Integration test: decode a path and extract the project name
	encoded := "-home-user-code-awesome%2Dproject"

	decoded, err := DecodeProjectPath(encoded)
	if err != nil {
		t.Fatalf("DecodeProjectPath failed: %v", err)
	}

	expected := "/home/user/code/awesome-project"
	if decoded != expected {
		t.Errorf("Decoded path = %q, want %q", decoded, expected)
	}

	projectName := ProjectNameFromPath(decoded)
	expectedName := "awesome-project"
	if projectName != expectedName {
		t.Errorf("Project name = %q, want %q", projectName, expectedName)
	}
}
