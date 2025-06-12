package utils

import (
	"testing"
)

func TestIsValidImageName(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		expected  bool
	}{
		{
			name:      "valid docker hub image",
			imageName: "nginx:latest",
			expected:  true,
		},
		{
			name:      "valid gcr image",
			imageName: "gcr.io/google-containers/pause:3.2",
			expected:  true,
		},
		{
			name:      "empty string",
			imageName: "",
			expected:  false,
		},
		{
			name:      "image with dangerous characters",
			imageName: "nginx; rm -rf /",
			expected:  false,
		},
		{
			name:      "image with pipe character",
			imageName: "nginx|malicious",
			expected:  false,
		},
		{
			name:      "too long image name",
			imageName: string(make([]byte, 300)),
			expected:  false,
		},
		{
			name:      "valid image with tag",
			imageName: "registry.k8s.io/pause:3.8",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidImageName(tt.imageName)
			if result != tt.expected {
				t.Errorf("IsValidImageName(%q) = %v, expected %v", tt.imageName, result, tt.expected)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string",
			input:    "nginx:latest",
			expected: "nginx:latest",
		},
		{
			name:     "string with newlines",
			input:    "nginx\nlatest",
			expected: "nginxlatest",
		},
		{
			name:     "string with dangerous characters",
			input:    "nginx; rm -rf /",
			expected: "nginx rm -rf /",
		},
		{
			name:     "string with tabs and spaces",
			input:    "  nginx\tlatest  ",
			expected: "nginxlatest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseIssueTitle(t *testing.T) {
	tests := []struct {
		name             string
		title            string
		expectedImage    string
		expectedPlatform string
	}{
		{
			name:             "simple image",
			title:            "[PORTER]nginx:latest",
			expectedImage:    "nginx:latest",
			expectedPlatform: "",
		},
		{
			name:             "image with platform",
			title:            "[PORTER]nginx:latest|linux/amd64",
			expectedImage:    "nginx:latest",
			expectedPlatform: "linux/amd64",
		},
		{
			name:             "gcr image",
			title:            "[PORTER]gcr.io/google-containers/pause:3.2",
			expectedImage:    "gcr.io/google-containers/pause:3.2",
			expectedPlatform: "",
		},
		{
			name:             "title with extra spaces",
			title:            "  [PORTER]  nginx:latest  ",
			expectedImage:    "nginx:latest",
			expectedPlatform: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, platform := ParseIssueTitle(tt.title)
			if image != tt.expectedImage {
				t.Errorf("ParseIssueTitle(%q) image = %q, expected %q", tt.title, image, tt.expectedImage)
			}
			if platform != tt.expectedPlatform {
				t.Errorf("ParseIssueTitle(%q) platform = %q, expected %q", tt.title, platform, tt.expectedPlatform)
			}
		})
	}
}

func TestExtractImageInfo(t *testing.T) {
	tests := []struct {
		name              string
		imageName         string
		expectedRegistry  string
		expectedNamespace string
		expectedRepo      string
		expectedTag       string
	}{
		{
			name:              "docker hub library image",
			imageName:         "nginx:latest",
			expectedRegistry:  "docker.io",
			expectedNamespace: "library",
			expectedRepo:      "nginx",
			expectedTag:       "latest",
		},
		{
			name:              "docker hub user image",
			imageName:         "user/nginx:latest",
			expectedRegistry:  "docker.io",
			expectedNamespace: "user",
			expectedRepo:      "nginx",
			expectedTag:       "latest",
		},
		{
			name:              "gcr image",
			imageName:         "gcr.io/google-containers/pause:3.2",
			expectedRegistry:  "gcr.io",
			expectedNamespace: "google-containers",
			expectedRepo:      "pause",
			expectedTag:       "3.2",
		},
		{
			name:              "image without tag",
			imageName:         "nginx",
			expectedRegistry:  "docker.io",
			expectedNamespace: "library",
			expectedRepo:      "nginx",
			expectedTag:       "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, namespace, repo, tag := ExtractImageInfo(tt.imageName)
			if registry != tt.expectedRegistry {
				t.Errorf("ExtractImageInfo(%q) registry = %q, expected %q", tt.imageName, registry, tt.expectedRegistry)
			}
			if namespace != tt.expectedNamespace {
				t.Errorf("ExtractImageInfo(%q) namespace = %q, expected %q", tt.imageName, namespace, tt.expectedNamespace)
			}
			if repo != tt.expectedRepo {
				t.Errorf("ExtractImageInfo(%q) repo = %q, expected %q", tt.imageName, repo, tt.expectedRepo)
			}
			if tag != tt.expectedTag {
				t.Errorf("ExtractImageInfo(%q) tag = %q, expected %q", tt.imageName, tag, tt.expectedTag)
			}
		})
	}
}
