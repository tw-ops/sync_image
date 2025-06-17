package registry

import (
	"fmt"
	"strings"

	"sync-image/internal/config"
	"sync-image/pkg/logger"
)

// PostProcessor 镜像后处理器接口
// 用于在镜像推送完成后执行额外的操作，如设置权限、添加标签等
type PostProcessor interface {
	// GetName 获取后处理器名称
	GetName() string

	// CanProcess 检查是否可以处理指定的镜像和仓库
	CanProcess(imageName, registryURL string) bool

	// Process 执行后处理操作
	Process(imageName, registryURL string) error

	// GetDescription 获取后处理器描述
	GetDescription() string
}

// PostProcessorManager 后处理器管理器
type PostProcessorManager struct {
	processors []PostProcessor
	logger     logger.Logger
}

// NewPostProcessorManager 创建新的后处理器管理器
func NewPostProcessorManager(log logger.Logger) *PostProcessorManager {
	return &PostProcessorManager{
		processors: make([]PostProcessor, 0),
		logger:     log,
	}
}

// RegisterProcessor 注册后处理器
func (m *PostProcessorManager) RegisterProcessor(processor PostProcessor) {
	m.processors = append(m.processors, processor)
	m.logger.Debug("Registered post-processor: %s", processor.GetName())
}

// ProcessImage 对镜像执行所有适用的后处理操作
func (m *PostProcessorManager) ProcessImage(imageName, registryURL string) error {
	m.logger.Debug("Starting post-processing for image: %s at registry: %s", imageName, registryURL)

	var errors []string
	processedCount := 0

	for _, processor := range m.processors {
		if processor.CanProcess(imageName, registryURL) {
			m.logger.Info("Applying post-processor: %s", processor.GetName())
			if err := processor.Process(imageName, registryURL); err != nil {
				errorMsg := fmt.Sprintf("Post-processor %s failed: %v", processor.GetName(), err)
				m.logger.Warn(errorMsg)
				errors = append(errors, errorMsg)
			} else {
				m.logger.Info("Post-processor %s completed successfully", processor.GetName())
				processedCount++
			}
		}
	}

	if len(errors) > 0 {
		m.logger.Warn("Some post-processors failed, but continuing: %s", strings.Join(errors, "; "))
		// 不返回错误，因为后处理失败不应该影响主流程
	}

	if processedCount > 0 {
		m.logger.Info("Post-processing completed: %d processors applied", processedCount)
	} else {
		m.logger.Debug("No applicable post-processors found for this image")
	}

	return nil
}

// GetRegisteredProcessors 获取已注册的后处理器列表
func (m *PostProcessorManager) GetRegisteredProcessors() []PostProcessor {
	return m.processors
}

// GetProcessorCount 获取已注册的后处理器数量
func (m *PostProcessorManager) GetProcessorCount() int {
	return len(m.processors)
}

// PostProcessorFactory 后处理器工厂
type PostProcessorFactory struct {
	config *config.Config
	logger logger.Logger
}

// NewPostProcessorFactory 创建新的后处理器工厂
func NewPostProcessorFactory(cfg *config.Config, log logger.Logger) *PostProcessorFactory {
	return &PostProcessorFactory{
		config: cfg,
		logger: log,
	}
}

// CreateManager 创建配置好的后处理器管理器
func (f *PostProcessorFactory) CreateManager() *PostProcessorManager {
	manager := NewPostProcessorManager(f.logger)

	// 注册华为云SWR后处理器
	if huaweiConfig := f.config.GetEffectiveHuaweiSWRConfig(); huaweiConfig != nil {
		if huaweiConfig.AccessKey != "" && huaweiConfig.SecretKey != "" {
			huaweiProcessor := NewHuaweiSWRPostProcessor(huaweiConfig, f.logger)
			manager.RegisterProcessor(huaweiProcessor)
		} else {
			f.logger.Debug("Huawei SWR credentials not configured, skipping Huawei post-processor")
		}
	}

	// 未来可以在这里注册其他云服务商的后处理器
	// 例如：
	// if aliConfig := f.config.GetEffectiveAliCloudConfig(); aliConfig != nil {
	//     aliProcessor := NewAliCloudPostProcessor(aliConfig, f.logger)
	//     manager.RegisterProcessor(aliProcessor)
	// }

	f.logger.Info("Post-processor manager created with %d processors", manager.GetProcessorCount())
	return manager
}

// BasePostProcessor 基础后处理器实现
type BasePostProcessor struct {
	name        string
	description string
	logger      logger.Logger
}

// NewBasePostProcessor 创建基础后处理器
func NewBasePostProcessor(name, description string, log logger.Logger) *BasePostProcessor {
	return &BasePostProcessor{
		name:        name,
		description: description,
		logger:      log,
	}
}

// GetName 获取后处理器名称
func (p *BasePostProcessor) GetName() string {
	return p.name
}

// GetDescription 获取后处理器描述
func (p *BasePostProcessor) GetDescription() string {
	return p.description
}

// isRegistryMatch 检查仓库URL是否匹配指定的模式
func (p *BasePostProcessor) isRegistryMatch(registryURL string, patterns []string) bool {
	// 清理URL，移除协议前缀
	cleanURL := strings.TrimPrefix(registryURL, "https://")
	cleanURL = strings.TrimPrefix(cleanURL, "http://")

	// 获取域名
	parts := strings.Split(cleanURL, "/")
	domain := parts[0]

	// 检查是否匹配任何模式
	for _, pattern := range patterns {
		if strings.Contains(domain, pattern) {
			return true
		}
	}

	return false
}
