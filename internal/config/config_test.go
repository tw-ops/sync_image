package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	
	// 检查默认值
	if cfg.Docker.Platforms != "linux/amd64,linux/arm64" {
		t.Errorf("Expected default platforms to be 'linux/amd64,linux/arm64', got %q", cfg.Docker.Platforms)
	}
	
	if cfg.HuaweiSWR.Region != "cn-southwest-2" {
		t.Errorf("Expected default region to be 'cn-southwest-2', got %q", cfg.HuaweiSWR.Region)
	}
	
	if cfg.App.LogLevel != "info" {
		t.Errorf("Expected default log level to be 'info', got %q", cfg.App.LogLevel)
	}
	
	// 检查默认规则
	expectedRules := map[string]string{
		"^gcr.io":          "",
		"^docker.io":       "docker",
		"^k8s.gcr.io":      "google-containers",
		"^registry.k8s.io": "google-containers",
		"^quay.io":         "quay",
		"^ghcr.io":         "ghcr",
	}
	
	for pattern, expected := range expectedRules {
		if actual, exists := cfg.Rules[pattern]; !exists {
			t.Errorf("Expected rule %q to exist", pattern)
		} else if actual != expected {
			t.Errorf("Expected rule %q to be %q, got %q", pattern, expected, actual)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				GitHub: GitHubConfig{
					Token: "test-token",
					User:  "test-user",
					Repo:  "test-repo",
				},
				Docker: DockerConfig{
					Username: "test-user",
					Password: "test-password",
				},
				HuaweiSWR: HuaweiSWRConfig{
					AccessKey: "test-ak",
					SecretKey: "test-sk",
				},
			},
			expectError: false,
		},
		{
			name: "missing github token",
			config: &Config{
				GitHub: GitHubConfig{
					User: "test-user",
					Repo: "test-repo",
				},
				Docker: DockerConfig{
					Username: "test-user",
					Password: "test-password",
				},
				HuaweiSWR: HuaweiSWRConfig{
					AccessKey: "test-ak",
					SecretKey: "test-sk",
				},
			},
			expectError: true,
		},
		{
			name: "missing docker username",
			config: &Config{
				GitHub: GitHubConfig{
					Token: "test-token",
					User:  "test-user",
					Repo:  "test-repo",
				},
				Docker: DockerConfig{
					Password: "test-password",
				},
				HuaweiSWR: HuaweiSWRConfig{
					AccessKey: "test-ak",
					SecretKey: "test-sk",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// 保存原始环境变量
	originalVars := map[string]string{
		"GITHUB_TOKEN":    os.Getenv("GITHUB_TOKEN"),
		"DOCKER_USER":     os.Getenv("DOCKER_USER"),
		"DOCKER_PASSWORD": os.Getenv("DOCKER_PASSWORD"),
		"AK":              os.Getenv("AK"),
		"SK":              os.Getenv("SK"),
	}
	
	// 清理函数
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()
	
	// 设置测试环境变量
	testVars := map[string]string{
		"GITHUB_TOKEN":    "test-github-token",
		"DOCKER_USER":     "test-docker-user",
		"DOCKER_PASSWORD": "test-docker-password",
		"AK":              "test-ak",
		"SK":              "test-sk",
	}
	
	for key, value := range testVars {
		os.Setenv(key, value)
	}
	
	// 测试加载环境变量
	config := DefaultConfig()
	loadFromEnv(config)
	
	if config.GitHub.Token != "test-github-token" {
		t.Errorf("Expected GitHub token to be 'test-github-token', got %q", config.GitHub.Token)
	}
	
	if config.Docker.Username != "test-docker-user" {
		t.Errorf("Expected Docker username to be 'test-docker-user', got %q", config.Docker.Username)
	}
	
	if config.Docker.Password != "test-docker-password" {
		t.Errorf("Expected Docker password to be 'test-docker-password', got %q", config.Docker.Password)
	}
	
	if config.HuaweiSWR.AccessKey != "test-ak" {
		t.Errorf("Expected Huawei access key to be 'test-ak', got %q", config.HuaweiSWR.AccessKey)
	}
	
	if config.HuaweiSWR.SecretKey != "test-sk" {
		t.Errorf("Expected Huawei secret key to be 'test-sk', got %q", config.HuaweiSWR.SecretKey)
	}
}

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short string",
			input:    "short",
			expected: "****",
		},
		{
			name:     "long string",
			input:    "this-is-a-very-long-secret-key",
			expected: "this****-key",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "****",
		},
		{
			name:     "exactly 8 characters",
			input:    "12345678",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitive(tt.input)
			if result != tt.expected {
				t.Errorf("maskSensitive(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
