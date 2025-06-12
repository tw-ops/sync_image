package service

import (
	"context"
	"strings"
	"testing"

	"sync-image/internal/config"
	"sync-image/pkg/errors"
	"sync-image/pkg/logger"
)

func TestFormatErrorDetails(t *testing.T) {
	cfg := &config.Config{
		GitHub: config.GitHubConfig{
			User:  "test-user",
			Repo:  "test-repo",
			RunID: "123456",
		},
	}

	log := logger.NewLogger("debug")

	service := &DefaultSyncService{
		config: cfg,
		logger: log,
	}

	// 创建一个测试错误
	appErr := errors.NewDockerError("Docker 构建失败", nil).
		WithContext("source_image", "nginx:latest").
		WithContext("target_image", "registry.example.com/test/nginx:latest").
		WithContext("platform", "linux/amd64")

	details := service.formatErrorDetails(appErr)

	// 验证错误详情包含预期内容
	expectedContents := []string{
		"错误类型: DOCKER_ERROR",
		"错误消息: Docker 构建失败",
		"上下文信息:",
		"source_image: nginx:latest",
		"target_image: registry.example.com/test/nginx:latest",
		"platform: linux/amd64",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(details, expected) {
			t.Errorf("错误详情中缺少预期内容: %s\n实际详情:\n%s", expected, details)
		}
	}
}

func TestGenerateResult(t *testing.T) {
	cfg := &config.Config{
		GitHub: config.GitHubConfig{
			User:  "test-user",
			Repo:  "test-repo",
			RunID: "123456",
		},
	}

	log := logger.NewLogger("debug")

	// 创建一个 mock 构建器
	mockBuilder := &MockBuilder{
		archInfo: "🏗️ **上游镜像架构**: linux/amd64\n📋 **请求构建架构**: linux/amd64\n✅ **实际构建架构**: linux/amd64",
	}

	service := &DefaultSyncService{
		config:        cfg,
		dockerBuilder: mockBuilder,
		logger:        log,
	}

	tests := []struct {
		name        string
		success     bool
		err         error
		expectError bool
		expectArch  bool
	}{
		{
			name:        "成功案例",
			success:     true,
			err:         nil,
			expectError: false,
			expectArch:  true,
		},
		{
			name:        "失败案例",
			success:     false,
			err:         errors.NewDockerError("构建失败", nil),
			expectError: true,
			expectArch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.generateResult("nginx:latest", "registry.example.com/test/nginx:latest", "linux/amd64", tt.success, tt.err)

			if tt.success {
				if !strings.Contains(result, "✅ 转换完成") {
					t.Error("成功结果应包含 '✅ 转换完成'")
				}
				if !strings.Contains(result, "```bash") {
					t.Error("成功结果应包含 bash 代码块")
				}
			} else {
				if !strings.Contains(result, "❌ 转换失败") {
					t.Error("失败结果应包含 '❌ 转换失败'")
				}
				if tt.expectError && !strings.Contains(result, "详细错误信息") {
					t.Error("失败结果应包含详细错误信息")
				}
			}

			if tt.expectArch && !strings.Contains(result, "上游镜像架构") {
				t.Error("结果应包含架构信息")
			}

			// 验证包含构建日志链接
			if !strings.Contains(result, "查看构建日志") {
				t.Error("结果应包含构建日志链接")
			}
		})
	}
}

// MockBuilder 用于测试的 mock 构建器
type MockBuilder struct {
	archInfo string
}

func (m *MockBuilder) Login(ctx context.Context) error {
	return nil
}

func (m *MockBuilder) BuildAndPush(ctx context.Context, sourceImage, targetImage, platform string) error {
	return nil
}

func (m *MockBuilder) WriteDockerfile(sourceImage string) error {
	return nil
}

func (m *MockBuilder) Cleanup() error {
	return nil
}

func (m *MockBuilder) GetLastArchitectureInfo() string {
	return m.archInfo
}
