package registry

import (
	"fmt"
	"strings"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	swr "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2/model"
	region "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2/region"

	"sync-image/internal/config"
	"sync-image/pkg/errors"
	"sync-image/pkg/logger"
)

// HuaweiSWRClient 华为云 SWR 客户端接口
type HuaweiSWRClient interface {
	MakeImagePublic(imageName string) error
}

// DefaultHuaweiSWRClient 默认华为云 SWR 客户端实现
type DefaultHuaweiSWRClient struct {
	client *swr.SwrClient
	config *config.HuaweiSWRConfig
	logger logger.Logger
}

// NewHuaweiSWRClient 创建新的华为云 SWR 客户端
func NewHuaweiSWRClient(cfg *config.HuaweiSWRConfig, log logger.Logger) (HuaweiSWRClient, error) {
	auth, err := basic.NewCredentialsBuilder().
		WithAk(cfg.AccessKey).
		WithSk(cfg.SecretKey).
		SafeBuild()
	if err != nil {
		return nil, errors.NewRegistryError("创建华为云认证失败", err)
	}

	client := swr.NewSwrClient(
		swr.SwrClientBuilder().
			WithRegion(region.ValueOf(cfg.Region)).
			WithCredential(auth).
			Build())

	return &DefaultHuaweiSWRClient{
		client: client,
		config: cfg,
		logger: log,
	}, nil
}

// MakeImagePublic 将镜像设置为公开
func (c *DefaultHuaweiSWRClient) MakeImagePublic(imageName string) error {
	c.logger.Debug("设置镜像为公开: %s", imageName)

	namespace, repository, err := c.parseImageName(imageName)
	if err != nil {
		return errors.NewRegistryError("解析镜像名称失败", err).
			WithContext("image_name", imageName)
	}

	request := &model.UpdateRepoRequest{
		Namespace:  namespace,
		Repository: repository,
		Body: &model.UpdateRepoRequestBody{
			IsPublic: true,
		},
	}

	c.logger.Debug("更新仓库权限: namespace=%s, repository=%s", namespace, repository)

	_, err = c.client.UpdateRepo(request)
	if err != nil {
		return errors.NewRegistryError("设置镜像公开权限失败", err).
			WithContext("namespace", namespace).
			WithContext("repository", repository)
	}

	c.logger.Info("成功设置镜像为公开: %s", imageName)
	return nil
}

// parseImageName 解析镜像名称，提取命名空间和仓库名
func (c *DefaultHuaweiSWRClient) parseImageName(imageName string) (namespace, repository string, err error) {
	// 移除注册表前缀
	parts := strings.Split(imageName, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("无效的镜像名称格式: %s", imageName)
	}

	// 提取命名空间和仓库名
	namespace = parts[1]
	repoWithTag := parts[2]

	// 移除标签
	if strings.Contains(repoWithTag, ":") {
		repository = strings.Split(repoWithTag, ":")[0]
	} else {
		repository = repoWithTag
	}

	if namespace == "" || repository == "" {
		return "", "", fmt.Errorf("无法从镜像名称中提取命名空间或仓库名: %s", imageName)
	}

	return namespace, repository, nil
}

// RegistryManager 镜像仓库管理器
type RegistryManager struct {
	huaweiClient HuaweiSWRClient
	logger       logger.Logger
}

// NewRegistryManager 创建新的镜像仓库管理器
func NewRegistryManager(huaweiClient HuaweiSWRClient, log logger.Logger) *RegistryManager {
	return &RegistryManager{
		huaweiClient: huaweiClient,
		logger:       log,
	}
}

// ProcessImage 处理镜像，设置权限等
func (m *RegistryManager) ProcessImage(imageName string) error {
	m.logger.Info("开始处理镜像: %s", imageName)

	// 设置镜像为公开
	if err := m.huaweiClient.MakeImagePublic(imageName); err != nil {
		return fmt.Errorf("设置镜像公开权限失败: %w", err)
	}

	m.logger.Info("镜像处理完成: %s", imageName)
	return nil
}

// ValidateImageName 验证镜像名称格式
func ValidateImageName(imageName string) error {
	if imageName == "" {
		return errors.NewValidationError("镜像名称不能为空")
	}

	parts := strings.Split(imageName, "/")
	if len(parts) < 2 {
		return errors.NewValidationError(fmt.Sprintf("镜像名称格式无效: %s", imageName))
	}

	// 检查是否包含注册表域名
	if !strings.Contains(parts[0], ".") {
		return errors.NewValidationError(fmt.Sprintf("镜像名称缺少注册表域名: %s", imageName))
	}

	return nil
}
