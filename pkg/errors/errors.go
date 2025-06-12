package errors

import (
	"fmt"
)

// ErrorType 错误类型
type ErrorType string

const (
	// ConfigError 配置错误
	ConfigError ErrorType = "CONFIG_ERROR"
	// GitHubError GitHub API 错误
	GitHubError ErrorType = "GITHUB_ERROR"
	// DockerError Docker 操作错误
	DockerError ErrorType = "DOCKER_ERROR"
	// RegistryError 镜像仓库错误
	RegistryError ErrorType = "REGISTRY_ERROR"
	// ValidationError 验证错误
	ValidationError ErrorType = "VALIDATION_ERROR"
	// SystemError 系统错误
	SystemError ErrorType = "SYSTEM_ERROR"
)

// AppError 应用程序错误
type AppError struct {
	Type    ErrorType
	Message string
	Cause   error
	Context map[string]interface{}
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap 返回底层错误
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithContext 添加上下文信息
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewError 创建新的应用程序错误
func NewError(errType ErrorType, message string) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// WrapError 包装现有错误
func WrapError(errType ErrorType, message string, cause error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// 预定义的错误创建函数

// NewConfigError 创建配置错误
func NewConfigError(message string) *AppError {
	return NewError(ConfigError, message)
}

// NewGitHubError 创建 GitHub 错误
func NewGitHubError(message string, cause error) *AppError {
	return WrapError(GitHubError, message, cause)
}

// NewDockerError 创建 Docker 错误
func NewDockerError(message string, cause error) *AppError {
	return WrapError(DockerError, message, cause)
}

// NewRegistryError 创建镜像仓库错误
func NewRegistryError(message string, cause error) *AppError {
	return WrapError(RegistryError, message, cause)
}

// NewValidationError 创建验证错误
func NewValidationError(message string) *AppError {
	return NewError(ValidationError, message)
}

// NewSystemError 创建系统错误
func NewSystemError(message string, cause error) *AppError {
	return WrapError(SystemError, message, cause)
}

// IsErrorType 检查错误是否为指定类型
func IsErrorType(err error, errType ErrorType) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == errType
	}
	return false
}

// GetErrorContext 获取错误上下文
func GetErrorContext(err error) map[string]interface{} {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Context
	}
	return nil
}

// FormatUserError 格式化用户友好的错误信息
func FormatUserError(err error, username string) string {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Type {
		case ValidationError:
			return fmt.Sprintf("@%s 输入验证失败: %s", username, appErr.Message)
		case DockerError:
			return fmt.Sprintf("@%s Docker 操作失败: %s", username, appErr.Message)
		case RegistryError:
			return fmt.Sprintf("@%s 镜像仓库操作失败: %s", username, appErr.Message)
		case GitHubError:
			return fmt.Sprintf("@%s GitHub 操作失败: %s", username, appErr.Message)
		default:
			return fmt.Sprintf("@%s 操作失败: %s", username, appErr.Message)
		}
	}
	return fmt.Sprintf("@%s 操作失败: %v", username, err)
}
