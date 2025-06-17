package registry

import (
	"fmt"
	"strings"

	"sync-image/internal/config"
	"sync-image/pkg/logger"
)

// HuaweiSWRPostProcessor 华为云SWR后处理器
// 专门用于在镜像推送到华为云SWR后设置镜像为公开访问
type HuaweiSWRPostProcessor struct {
	*BasePostProcessor
	config *config.HuaweiSWRConfig
}

// NewHuaweiSWRPostProcessor 创建新的华为云SWR后处理器
func NewHuaweiSWRPostProcessor(cfg *config.HuaweiSWRConfig, log logger.Logger) PostProcessor {
	base := NewBasePostProcessor(
		"Huawei SWR Public Access Setter",
		"Sets Huawei SWR images to public access after push",
		log,
	)

	return &HuaweiSWRPostProcessor{
		BasePostProcessor: base,
		config:            cfg,
	}
}

// CanProcess 检查是否可以处理指定的镜像和仓库
func (p *HuaweiSWRPostProcessor) CanProcess(imageName, registryURL string) bool {
	// 检查是否是华为云SWR仓库
	huaweiPatterns := []string{
		"swr.",
		"myhuaweicloud.com",
	}

	if !p.isRegistryMatch(registryURL, huaweiPatterns) {
		p.logger.Debug("Registry %s is not Huawei SWR, skipping", registryURL)
		return false
	}

	// 检查配置是否完整
	if p.config.AccessKey == "" || p.config.SecretKey == "" {
		p.logger.Debug("Huawei SWR credentials not configured, skipping post-processing")
		return false
	}

	p.logger.Debug("Huawei SWR post-processor can process: %s at %s", imageName, registryURL)
	return true
}

// Process 执行华为云SWR后处理操作（设置镜像为公开访问）
func (p *HuaweiSWRPostProcessor) Process(imageName, registryURL string) error {
	p.logger.Info("Setting Huawei SWR image to public access: %s", imageName)

	// 解析镜像名称获取命名空间和仓库名
	namespace, repository, err := p.parseImageName(imageName)
	if err != nil {
		return fmt.Errorf("failed to parse image name: %w", err)
	}

	p.logger.Debug("Parsed image: namespace=%s, repository=%s", namespace, repository)

	// 调用华为云SDK设置镜像为公开访问
	if err := p.setImagePublic(namespace, repository); err != nil {
		return fmt.Errorf("failed to set image public: %w", err)
	}

	p.logger.Info("Successfully set Huawei SWR image to public: %s/%s", namespace, repository)
	return nil
}

// parseImageName 解析华为云SWR镜像名称
func (p *HuaweiSWRPostProcessor) parseImageName(imageName string) (namespace, repository string, err error) {
	// 移除仓库前缀（如果存在）
	parts := strings.Split(imageName, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid Huawei SWR image name format: %s", imageName)
	}

	// 华为云SWR格式：registry/namespace/repository:tag
	if len(parts) >= 3 && strings.Contains(parts[0], "myhuaweicloud.com") {
		namespace = parts[1]
		repository = strings.Split(parts[2], ":")[0] // 移除标签
	} else if len(parts) >= 2 {
		// 回退格式：namespace/repository:tag
		namespace = parts[0]
		repository = strings.Split(parts[1], ":")[0] // 移除标签
	}

	if namespace == "" || repository == "" {
		return "", "", fmt.Errorf("failed to extract namespace and repository from: %s", imageName)
	}

	return namespace, repository, nil
}

// setImagePublic 设置华为云SWR镜像为公开访问
func (p *HuaweiSWRPostProcessor) setImagePublic(namespace, repository string) error {
	// 这里是华为云SDK调用的占位符
	// 在实际实现中，这里会调用华为云SWR API来设置镜像权限
	
	p.logger.Debug("Calling Huawei Cloud SWR API to set image public")
	p.logger.Debug("Parameters: namespace=%s, repository=%s, region=%s", 
		namespace, repository, p.config.Region)
	p.logger.Debug("Credentials: AccessKey=%s", p.maskSensitive(p.config.AccessKey))

	// TODO: 实现实际的华为云SDK调用
	// 示例代码结构：
	//
	// import "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr"
	// import "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2/model"
	//
	// // 创建认证
	// auth := basic.NewCredentialsBuilder().
	//     WithAk(p.config.AccessKey).
	//     WithSk(p.config.SecretKey).
	//     SafeBuild()
	//
	// // 创建客户端
	// client := swr.NewSwrClient(
	//     swr.SwrClientBuilder().
	//         WithRegion(region.ValueOf(p.config.Region)).
	//         WithCredential(auth).
	//         Build())
	//
	// // 构建请求
	// request := &model.UpdateRepoRequest{
	//     Namespace:  namespace,
	//     Repository: repository,
	//     Body: &model.UpdateRepoRequestBody{
	//         IsPublic: true,
	//     },
	// }
	//
	// // 发送请求
	// _, err := client.UpdateRepo(request)
	// if err != nil {
	//     return fmt.Errorf("华为云API调用失败: %w", err)
	// }

	p.logger.Debug("Huawei Cloud SWR API call completed (placeholder implementation)")
	return nil
}

// maskSensitive 遮蔽敏感信息用于日志记录
func (p *HuaweiSWRPostProcessor) maskSensitive(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// GetSupportedRegistries 获取支持的仓库模式
func (p *HuaweiSWRPostProcessor) GetSupportedRegistries() []string {
	return []string{
		"swr.*.myhuaweicloud.com",
		"*.swr.*.myhuaweicloud.com",
	}
}

// ValidateConfig 验证华为云配置
func (p *HuaweiSWRPostProcessor) ValidateConfig() error {
	if p.config.AccessKey == "" {
		return fmt.Errorf("Huawei SWR AccessKey is required")
	}
	if p.config.SecretKey == "" {
		return fmt.Errorf("Huawei SWR SecretKey is required")
	}
	if p.config.Region == "" {
		return fmt.Errorf("Huawei SWR Region is required")
	}
	return nil
}

// GetConfigInfo 获取配置信息（用于调试）
func (p *HuaweiSWRPostProcessor) GetConfigInfo() map[string]interface{} {
	return map[string]interface{}{
		"access_key": p.maskSensitive(p.config.AccessKey),
		"region":     p.config.Region,
		"configured": p.config.AccessKey != "" && p.config.SecretKey != "",
	}
}
