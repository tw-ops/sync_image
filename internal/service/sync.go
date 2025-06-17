package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/google/go-github/v47/github"

	"sync-image/internal/config"
	"sync-image/internal/docker"
	githubclient "sync-image/internal/github"
	"sync-image/internal/registry"
	"sync-image/pkg/errors"
	"sync-image/pkg/logger"
)

// SyncService 同步服务接口
type SyncService interface {
	ProcessIssues(ctx context.Context) error
	Cleanup() error
}

// DefaultSyncService 默认同步服务实现
type DefaultSyncService struct {
	config           *config.Config
	githubClient     githubclient.Client
	issueProcessor   *githubclient.IssueProcessor
	dockerBuilder    docker.Builder
	imageTransformer *docker.ImageTransformer
	registryFactory  *registry.RegistryManagerFactory
	logger           logger.Logger
}

// NewSyncService 创建新的同步服务
func NewSyncService(
	cfg *config.Config,
	githubClient githubclient.Client,
	issueProcessor *githubclient.IssueProcessor,
	dockerBuilder docker.Builder,
	imageTransformer *docker.ImageTransformer,
	registryFactory *registry.RegistryManagerFactory,
	log logger.Logger,
) SyncService {
	return &DefaultSyncService{
		config:           cfg,
		githubClient:     githubClient,
		issueProcessor:   issueProcessor,
		dockerBuilder:    dockerBuilder,
		imageTransformer: imageTransformer,
		registryFactory:  registryFactory,
		logger:           log,
	}
}

// ProcessIssues 处理 Issues
func (s *DefaultSyncService) ProcessIssues(ctx context.Context) error {
	s.logger.Info("开始处理 Issues")

	// 获取待处理的 Issues
	issues, err := s.githubClient.GetPendingIssues(ctx)
	if err != nil {
		return fmt.Errorf("获取待处理 Issues 失败: %w", err)
	}

	if len(issues) == 0 {
		s.logger.Info("暂无需要搬运的镜像")
		return nil
	}

	// 处理第一个 Issue（每次只处理一个）
	issue := issues[0]
	return s.processSingleIssue(ctx, issue)
}

// processSingleIssue 处理单个 Issue
func (s *DefaultSyncService) processSingleIssue(ctx context.Context, issue *github.Issue) error {
	s.logger.Info("开始处理 Issue #%d", issue.GetNumber())

	var (
		sourceImage string
		targetImage string
		platform    string
		syncErr     error
	)

	// 处理 Issue 并获取镜像信息
	originalImage, platform, err := s.issueProcessor.ProcessIssue(ctx, issue)
	if err != nil {
		syncErr = err
	} else {
		// 执行镜像同步
		sourceImage, targetImage, syncErr = s.syncImage(ctx, originalImage, platform)
	}

	// 生成结果报告
	result := s.generateResult(sourceImage, targetImage, platform, syncErr == nil, syncErr)

	// 完成 Issue 处理
	if finishErr := s.issueProcessor.FinishIssue(ctx, issue, syncErr == nil, result, platform); finishErr != nil {
		s.logger.Error("完成 Issue 处理失败: %v", finishErr)
	}

	if syncErr != nil {
		return fmt.Errorf("镜像同步失败: %w", syncErr)
	}

	s.logger.Info("Issue #%d 处理完成", issue.GetNumber())
	return nil
}

// syncImage 同步镜像
func (s *DefaultSyncService) syncImage(ctx context.Context, originalImage, platform string) (sourceImage, targetImage string, err error) {
	s.logger.Info("开始同步镜像: %s", originalImage)

	// 获取有效的通用配置
	genericConfig := s.config.GetEffectiveGenericConfig()
	var registry, namespace string
	if genericConfig != nil {
		registry = genericConfig.Registry
		namespace = genericConfig.Namespace
	}

	// 转换镜像名称
	sourceImage, targetImage, err = s.imageTransformer.Transform(
		originalImage,
		registry,
		namespace,
	)
	if err != nil {
		return "", "", fmt.Errorf("镜像名称转换失败: %w", err)
	}

	// 验证转换结果
	if err := s.imageTransformer.ValidateTransformation(sourceImage, targetImage); err != nil {
		return sourceImage, targetImage, fmt.Errorf("镜像名称验证失败: %w", err)
	}

	s.logger.Info("镜像名称转换完成: %s -> %s", sourceImage, targetImage)

	// 构建并推送镜像（内部会自动处理登录和架构检测）
	if err := s.dockerBuilder.BuildAndPush(ctx, sourceImage, targetImage, platform); err != nil {
		return sourceImage, targetImage, fmt.Errorf("Docker 构建推送失败: %w", err)
	}

	// 动态创建仓库处理器并设置镜像权限
	if err := s.processImageWithDynamicRegistry(targetImage); err != nil {
		return sourceImage, targetImage, fmt.Errorf("设置镜像权限失败: %w", err)
	}

	s.logger.Info("镜像同步完成: %s", targetImage)
	return sourceImage, targetImage, nil
}

// processImageWithDynamicRegistry 动态创建仓库处理器并处理镜像
func (s *DefaultSyncService) processImageWithDynamicRegistry(targetImage string) error {
	s.logger.Debug("开始动态处理镜像: %s", targetImage)

	// 从目标镜像URL中提取仓库地址
	registryURL := s.extractRegistryURL(targetImage)
	s.logger.Debug("提取到仓库地址: %s", registryURL)

	// 创建对应的仓库处理器
	processor, err := s.registryFactory.CreateProcessor(registryURL)
	if err != nil {
		return fmt.Errorf("创建仓库处理器失败: %w", err)
	}

	s.logger.Info("使用处理器: %s 处理镜像: %s", processor.GetName(), targetImage)

	// 创建仓库管理器并处理镜像
	registryManager := registry.NewRegistryManager(processor, s.logger)
	if err := registryManager.ProcessImage(targetImage); err != nil {
		return fmt.Errorf("处理镜像失败: %w", err)
	}

	s.logger.Info("镜像处理完成，使用的处理器类型: %s", processor.GetType())
	return nil
}

// extractRegistryURL 从镜像名称中提取仓库URL
func (s *DefaultSyncService) extractRegistryURL(imageName string) string {
	// 镜像名称格式: registry.domain.com/namespace/image:tag
	parts := strings.Split(imageName, "/")
	if len(parts) > 0 {
		return parts[0]
	}

	// 如果没有明确的注册表域名，默认为 Docker Hub
	return "docker.io"
}

// generateResult 生成结果报告
func (s *DefaultSyncService) generateResult(sourceImage, targetImage, platform string, success bool, err error) string {
	result := ResultData{
		Success:          success,
		SourceImage:      sourceImage,
		TargetImage:      targetImage,
		Platform:         platform,
		GitHubUser:       s.config.GitHub.User,
		GitHubRepo:       s.config.GitHub.Repo,
		GitHubRunID:      s.config.GitHub.RunID,
		ErrorMessage:     "",
		ArchitectureInfo: s.dockerBuilder.GetLastArchitectureInfo(), // 获取架构信息
	}

	if !success && err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			result.ErrorMessage = errors.FormatUserError(appErr, s.config.GitHub.User)
			// 提供详细的错误信息
			result.ErrorDetails = s.formatErrorDetails(appErr)
		} else {
			result.ErrorMessage = fmt.Sprintf("操作失败: %v", err)
			result.ErrorDetails = err.Error()
		}
	}

	return s.renderTemplate(result)
}

// ResultData 结果数据结构
type ResultData struct {
	Success          bool
	SourceImage      string
	TargetImage      string
	Platform         string
	GitHubUser       string
	GitHubRepo       string
	GitHubRunID      string
	ErrorMessage     string
	ErrorDetails     string // 详细错误信息
	ArchitectureInfo string // 架构信息
}

// renderTemplate 渲染模板
func (s *DefaultSyncService) renderTemplate(data ResultData) string {
	const resultTemplate = `
{{ if .Success }}
**✅ 转换完成**

` + "```bash" + `
# 原镜像
{{ .SourceImage }}

# 转换后镜像
{{ .TargetImage }}

# 下载并重命名镜像
docker pull {{ .TargetImage }}{{ if .Platform }} --platform {{ .Platform }}{{ end }}

docker tag {{ .TargetImage }} {{ .SourceImage }}

docker images | grep $(echo {{ .SourceImage }} | awk -F':' '{print $1}')
` + "```" + `
{{ if .ArchitectureInfo }}

{{ .ArchitectureInfo }}
{{ end }}

---
📋 **构建详情**: [查看构建日志](https://github.com/{{ .GitHubUser }}/{{ .GitHubRepo }}/actions/runs/{{ .GitHubRunID }})
{{ else }}
**❌ 转换失败**

{{ if .ErrorMessage }}**错误原因**: {{ .ErrorMessage }}{{ end }}

{{ if .ErrorDetails }}
**详细错误信息**:
` + "```" + `
{{ .ErrorDetails }}
` + "```" + `
{{ end }}

{{ if .ArchitectureInfo }}
**架构信息**:
{{ .ArchitectureInfo }}
{{ end }}

---
🔍 **排查建议**:
1. 检查镜像名称是否正确
2. 确认上游镜像是否存在
3. 查看详细的构建日志

📋 **构建详情**: [查看构建日志](https://github.com/{{ .GitHubUser }}/{{ .GitHubRepo }}/actions/runs/{{ .GitHubRunID }})
{{ end }}`

	funcMap := template.FuncMap{
		"split": strings.Split,
	}

	tmpl, err := template.New("result").Funcs(funcMap).Parse(resultTemplate)
	if err != nil {
		s.logger.Error("解析模板失败: %v", err)
		return "模板解析失败"
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		s.logger.Error("渲染模板失败: %v", err)
		return "模板渲染失败"
	}

	return buf.String()
}

// formatErrorDetails 格式化详细错误信息
func (s *DefaultSyncService) formatErrorDetails(appErr *errors.AppError) string {
	var details strings.Builder

	// 错误类型
	details.WriteString(fmt.Sprintf("错误类型: `%s`\n", appErr.Type))

	// 错误消息
	details.WriteString(fmt.Sprintf("错误消息: %s\n", appErr.Message))

	// 底层错误
	if appErr.Cause != nil {
		details.WriteString(fmt.Sprintf("底层错误: `%s`\n", appErr.Cause.Error()))
	}

	// 上下文信息
	if len(appErr.Context) > 0 {
		details.WriteString("上下文信息:\n")
		for key, value := range appErr.Context {
			details.WriteString(fmt.Sprintf("  %s: `%v`\n", key, value))
		}
	}

	return details.String()
}

// Cleanup 清理资源
func (s *DefaultSyncService) Cleanup() error {
	s.logger.Debug("清理服务资源")

	if err := s.dockerBuilder.Cleanup(); err != nil {
		s.logger.Warn("清理 Docker 构建器失败: %v", err)
		return err
	}

	return nil
}
