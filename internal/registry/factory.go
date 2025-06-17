package registry

import (
	"fmt"

	"sync-image/internal/config"
	"sync-image/pkg/logger"
)

// RegistryManagerFactory 仓库管理器工厂
type RegistryManagerFactory struct {
	config   *config.Config
	detector *RegistryTypeDetector
	logger   logger.Logger
}

// NewRegistryManagerFactory 创建新的仓库管理器工厂
func NewRegistryManagerFactory(cfg *config.Config, log logger.Logger) *RegistryManagerFactory {
	return &RegistryManagerFactory{
		config:   cfg,
		detector: NewRegistryTypeDetector(log),
		logger:   log,
	}
}

// CreateProcessor 根据仓库URL创建对应的处理器
func (f *RegistryManagerFactory) CreateProcessor(registryURL string) (RegistryProcessor, error) {
	f.logger.Debug("为仓库创建处理器: %s", registryURL)

	registryType := f.detector.DetectRegistryType(registryURL)
	f.logger.Info("检测到仓库类型: %s -> %s", registryURL, registryType)

	// 所有仓库都使用通用处理器，华为云特殊处理集成在通用处理器中
	return f.createGenericProcessor(), nil
}

// createHuaweiSWRProcessor function removed - now integrated into generic processor

// createGenericProcessor 创建通用处理器（集成后处理机制）
func (f *RegistryManagerFactory) createGenericProcessor() RegistryProcessor {
	f.logger.Debug("创建通用处理器")

	// 获取有效的通用仓库配置
	genericConfig := f.config.GetEffectiveGenericConfig()

	if genericConfig != nil {
		// 创建后处理器管理器
		postProcessorFactory := NewPostProcessorFactory(f.config, f.logger)
		postProcessorManager := postProcessorFactory.CreateManager()

		return NewEnhancedGenericProcessor(genericConfig, postProcessorManager, f.logger)
	}

	// 否则创建基础通用处理器
	return NewGenericProcessor(f.logger)
}

// GetSupportedRegistryTypes 获取支持的仓库类型列表
func (f *RegistryManagerFactory) GetSupportedRegistryTypes() []RegistryType {
	return []RegistryType{
		RegistryTypeGeneric,
	}
}

// ValidateRegistryConfig 验证特定仓库类型的配置
func (f *RegistryManagerFactory) ValidateRegistryConfig(registryType RegistryType) error {
	switch registryType {
	case RegistryTypeGeneric:
		// 通用处理器不需要强制配置验证
		// 华为云特殊处理是可选的，在通用处理器中处理
		return nil

	default:
		// 所有仓库类型都使用通用处理器，不需要强制配置验证
		return nil
	}
}

// NewRegistryManager 创建新的镜像仓库管理器（兼容旧接口）
func NewRegistryManager(processor RegistryProcessor, log logger.Logger) *RegistryManager {
	return &RegistryManager{
		processor: processor,
		logger:    log,
	}
}

// RegistryManager 镜像仓库管理器（重构版本）
type RegistryManager struct {
	processor RegistryProcessor
	logger    logger.Logger
}

// ProcessImage 处理镜像，设置权限等
func (m *RegistryManager) ProcessImage(imageName string) error {
	m.logger.Info("开始处理镜像: %s (使用 %s)", imageName, m.processor.GetName())

	if err := m.processor.ProcessImage(imageName); err != nil {
		return fmt.Errorf("镜像处理失败: %w", err)
	}

	m.logger.Info("镜像处理完成: %s", imageName)
	return nil
}

// GetProcessorType 获取当前处理器类型
func (m *RegistryManager) GetProcessorType() RegistryType {
	return m.processor.GetType()
}

// GetProcessorName 获取当前处理器名称
func (m *RegistryManager) GetProcessorName() string {
	return m.processor.GetName()
}
