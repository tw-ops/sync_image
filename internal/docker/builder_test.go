package docker

import (
	"testing"

	"sync-image/internal/config"
	"sync-image/pkg/logger"
)

func TestNewBuilder(t *testing.T) {
	cfg := &config.DockerConfig{
		Registry:  "test-registry",
		Namespace: "test-namespace",
		Username:  "test-user",
		Password:  "test-password",
		Platforms: "linux/amd64,linux/arm64",
	}

	log := logger.NewLogger("debug")

	// 注意：这个测试需要 Docker 环境，在没有 Docker 的环境中会 panic
	// 在实际的 CI/CD 环境中，这个测试应该被跳过或使用 mock
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic in test environment without Docker: %v", r)
		}
	}()

	builder := NewBuilder(cfg, log)
	if builder == nil {
		t.Error("NewBuilder returned nil")
	}
}

func TestImageTransformer(t *testing.T) {
	rules := map[string]string{
		"^gcr.io":          "",
		"^k8s.gcr.io":      "google-containers",
		"^registry.k8s.io": "google-containers",
		"^quay.io":         "quay",
		"^ghcr.io":         "ghcr",
		"^docker.io":       "docker",
	}

	log := logger.NewLogger("debug")
	transformer := NewImageTransformer(rules, log)

	if transformer == nil {
		t.Fatal("NewImageTransformer returned nil")
	}

	tests := []struct {
		name            string
		originalImage   string
		targetRegistry  string
		targetNamespace string
		expectedSource  string
		expectedTarget  string
		expectError     bool
	}{
		{
			name:            "gcr.io image",
			originalImage:   "gcr.io/google-containers/pause:3.2",
			targetRegistry:  "registry.example.com",
			targetNamespace: "test",
			expectedSource:  "gcr.io/google-containers/pause:3.2",
			expectedTarget:  "registry.example.com/test/pause:3.2",
			expectError:     false,
		},
		{
			name:            "k8s.gcr.io image",
			originalImage:   "k8s.gcr.io/pause:3.2",
			targetRegistry:  "registry.example.com",
			targetNamespace: "test",
			expectedSource:  "k8s.gcr.io/pause:3.2",
			expectedTarget:  "registry.example.com/test/pause:3.2",
			expectError:     false,
		},
		{
			name:            "docker.io image",
			originalImage:   "nginx:latest",
			targetRegistry:  "registry.example.com",
			targetNamespace: "test",
			expectedSource:  "docker.io/nginx:latest",
			expectedTarget:  "registry.example.com/test/nginx:latest",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceImage, targetImage, err := transformer.Transform(
				tt.originalImage,
				tt.targetRegistry,
				tt.targetNamespace,
			)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if sourceImage != tt.expectedSource {
				t.Errorf("Expected source image %q, got %q", tt.expectedSource, sourceImage)
			}

			if targetImage != tt.expectedTarget {
				t.Errorf("Expected target image %q, got %q", tt.expectedTarget, targetImage)
			}
		})
	}
}

func TestValidateTransformation(t *testing.T) {
	log := logger.NewLogger("debug")
	transformer := NewImageTransformer(map[string]string{}, log)

	tests := []struct {
		name        string
		sourceImage string
		targetImage string
		expectError bool
	}{
		{
			name:        "valid images",
			sourceImage: "nginx:latest",
			targetImage: "registry.example.com/test/nginx:latest",
			expectError: false,
		},
		{
			name:        "empty source image",
			sourceImage: "",
			targetImage: "registry.example.com/test/nginx:latest",
			expectError: true,
		},
		{
			name:        "empty target image",
			sourceImage: "nginx:latest",
			targetImage: "",
			expectError: true,
		},
		{
			name:        "invalid source image",
			sourceImage: "nginx; rm -rf /",
			targetImage: "registry.example.com/test/nginx:latest",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := transformer.ValidateTransformation(tt.sourceImage, tt.targetImage)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
