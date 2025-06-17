package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用程序配置结构
type Config struct {
	GitHub     GitHubConfig      `yaml:"github"`
	Registries RegistriesConfig  `yaml:"registries"` // 多云配置
	Rules      map[string]string `yaml:"rules"`
	App        AppConfig         `yaml:"app"`
	Platforms  string            `yaml:"platforms"` // 移到顶层配置
}

// GitHubConfig GitHub 相关配置
type GitHubConfig struct {
	Token string `yaml:"token"`
	User  string `yaml:"user"`
	Repo  string `yaml:"repo"`
	RunID string `yaml:"run_id"`
}

// RegistriesConfig 多云仓库配置
type RegistriesConfig struct {
	HuaweiSWR *HuaweiSWRConfig       `yaml:"huawei_swr,omitempty"`
	Generic   *GenericRegistryConfig `yaml:"generic,omitempty"`
}

// HuaweiSWRConfig 华为云 SWR 配置（保留特殊处理）
type HuaweiSWRConfig struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
}

// GenericRegistryConfig 通用仓库配置
// 适用于Docker Hub、私有仓库等所有其他仓库
type GenericRegistryConfig struct {
	Registry  string `yaml:"registry"`  // 仓库地址，如 registry.cn-hangzhou.aliyuncs.com
	Namespace string `yaml:"namespace"` // 命名空间
	Username  string `yaml:"username"`  // 用户名
	Password  string `yaml:"password"`  // 密码或访问令牌
}

// AppConfig 应用程序配置
type AppConfig struct {
	LogLevel string `yaml:"log_level"`
	Debug    bool   `yaml:"debug"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Platforms: "linux/amd64,linux/arm64",
		Rules: map[string]string{
			"^gcr.io":          "",
			"^docker.io":       "docker",
			"^k8s.gcr.io":      "google-containers",
			"^registry.k8s.io": "google-containers",
			"^quay.io":         "quay",
			"^ghcr.io":         "ghcr",
		},
		App: AppConfig{
			LogLevel: "info",
			Debug:    false,
		},
	}
}

// LoadConfig 从文件和环境变量加载配置
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	// 从文件加载配置
	if configPath != "" {
		if err := loadFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// 从环境变量加载配置
	loadFromEnv(config)

	// 不再提供向后兼容性支持

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// loadFromFile 从 YAML 文件加载配置
func loadFromFile(config *Config, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// 尝试直接解析为完整配置
	if err := yaml.Unmarshal(data, config); err == nil {
		return nil
	}

	// 如果失败，尝试只解析规则部分
	rules := make(map[string]string)
	if err := yaml.Unmarshal(data, &rules); err == nil {
		config.Rules = rules
		return nil
	}

	return fmt.Errorf("无法解析配置文件: %s", filePath)
}

// loadFromEnv 从环境变量加载配置
func loadFromEnv(config *Config) {
	// GitHub 配置
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		config.GitHub.Token = token
	}
	if user := os.Getenv("GITHUB_USER"); user != "" {
		config.GitHub.User = user
	}
	if repo := os.Getenv("GITHUB_REPO"); repo != "" {
		config.GitHub.Repo = repo
	}
	if runID := os.Getenv("GITHUB_RUN_ID"); runID != "" {
		config.GitHub.RunID = runID
	}

	// 平台架构配置
	if platforms := os.Getenv("PLATFORMS"); platforms != "" {
		config.Platforms = platforms
	}

	// 华为云 SWR 配置
	if ak := os.Getenv("HUAWEI_SWR_ACCESS_KEY"); ak != "" {
		if config.Registries.HuaweiSWR == nil {
			config.Registries.HuaweiSWR = &HuaweiSWRConfig{Region: "cn-southwest-2"}
		}
		config.Registries.HuaweiSWR.AccessKey = ak
	}
	if sk := os.Getenv("HUAWEI_SWR_SECRET_KEY"); sk != "" {
		if config.Registries.HuaweiSWR == nil {
			config.Registries.HuaweiSWR = &HuaweiSWRConfig{Region: "cn-southwest-2"}
		}
		config.Registries.HuaweiSWR.SecretKey = sk
	}
	if region := os.Getenv("HUAWEI_SWR_REGION"); region != "" {
		if config.Registries.HuaweiSWR == nil {
			config.Registries.HuaweiSWR = &HuaweiSWRConfig{}
		}
		config.Registries.HuaweiSWR.Region = region
	}

	// 新的多云配置环境变量
	loadRegistriesFromEnv(config)

	// 应用配置
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.App.LogLevel = logLevel
	}
	if debug := os.Getenv("DEBUG"); debug != "" {
		config.App.Debug = strings.ToLower(debug) == "true"
	}
}

// loadRegistriesFromEnv 从环境变量加载多云仓库配置
func loadRegistriesFromEnv(config *Config) {
	// 华为云 SWR 配置
	if ak := os.Getenv("HUAWEI_SWR_ACCESS_KEY"); ak != "" {
		if config.Registries.HuaweiSWR == nil {
			config.Registries.HuaweiSWR = &HuaweiSWRConfig{}
		}
		config.Registries.HuaweiSWR.AccessKey = ak
	}
	if sk := os.Getenv("HUAWEI_SWR_SECRET_KEY"); sk != "" {
		if config.Registries.HuaweiSWR == nil {
			config.Registries.HuaweiSWR = &HuaweiSWRConfig{}
		}
		config.Registries.HuaweiSWR.SecretKey = sk
	}
	if region := os.Getenv("HUAWEI_SWR_REGION"); region != "" {
		if config.Registries.HuaweiSWR == nil {
			config.Registries.HuaweiSWR = &HuaweiSWRConfig{}
		}
		config.Registries.HuaweiSWR.Region = region
	}

	// 通用仓库配置（适用于Docker Hub、私有仓库等）
	if registry := os.Getenv("GENERIC_REGISTRY"); registry != "" {
		if config.Registries.Generic == nil {
			config.Registries.Generic = &GenericRegistryConfig{}
		}
		config.Registries.Generic.Registry = registry
	}
	if namespace := os.Getenv("GENERIC_NAMESPACE"); namespace != "" {
		if config.Registries.Generic == nil {
			config.Registries.Generic = &GenericRegistryConfig{}
		}
		config.Registries.Generic.Namespace = namespace
	}
	if username := os.Getenv("GENERIC_USERNAME"); username != "" {
		if config.Registries.Generic == nil {
			config.Registries.Generic = &GenericRegistryConfig{}
		}
		config.Registries.Generic.Username = username
	}
	if password := os.Getenv("GENERIC_PASSWORD"); password != "" {
		if config.Registries.Generic == nil {
			config.Registries.Generic = &GenericRegistryConfig{}
		}
		config.Registries.Generic.Password = password
	}
}

// 不再提供向后兼容性支持

// validateConfig 验证配置的有效性
func validateConfig(config *Config) error {
	if config.GitHub.Token == "" {
		return fmt.Errorf("GitHub token is required")
	}
	if config.GitHub.User == "" {
		return fmt.Errorf("GitHub user is required")
	}
	if config.GitHub.Repo == "" {
		return fmt.Errorf("GitHub repo is required")
	}
	// 所有仓库配置都是可选的，不强制要求

	// 华为云配置现在是可选的，只在配置了的情况下验证
	if config.Registries.HuaweiSWR != nil {
		if err := validateHuaweiSWRConfig(config.Registries.HuaweiSWR); err != nil {
			return fmt.Errorf("Huawei SWR config validation failed: %w", err)
		}
	}

	// 验证通用仓库配置
	if config.Registries.Generic != nil {
		if err := validateGenericRegistryConfig(config.Registries.Generic); err != nil {
			return fmt.Errorf("Generic registry config validation failed: %w", err)
		}
	}

	return nil
}

// validateHuaweiSWRConfig 验证华为云SWR配置
func validateHuaweiSWRConfig(config *HuaweiSWRConfig) error {
	if config.AccessKey == "" {
		return fmt.Errorf("access key is required")
	}
	if config.SecretKey == "" {
		return fmt.Errorf("secret key is required")
	}
	if config.Region == "" {
		return fmt.Errorf("region is required")
	}
	return nil
}

// validateGenericRegistryConfig 验证通用仓库配置
func validateGenericRegistryConfig(config *GenericRegistryConfig) error {
	// 通用配置验证非常宽松，允许匿名访问公共仓库
	// 只验证基本的配置一致性，不强制要求认证信息
	if config.Registry == "" {
		return fmt.Errorf("registry is required when generic config is provided")
	}
	// username 和 password 是可选的，支持匿名访问公共仓库
	// namespace 是可选的，某些仓库可能不需要
	return nil
}

// GetSafeConfig 返回脱敏后的配置用于日志记录
func (c *Config) GetSafeConfig() *Config {
	safe := *c
	safe.GitHub.Token = maskSensitive(c.GitHub.Token)

	// 华为云配置已移至 registries 配置中

	// 脱敏新的多云配置
	safe.Registries = maskRegistriesConfig(c.Registries)

	return &safe
}

// maskRegistriesConfig 脱敏多云仓库配置
func maskRegistriesConfig(registries RegistriesConfig) RegistriesConfig {
	safe := registries

	if registries.HuaweiSWR != nil {
		safe.HuaweiSWR = &HuaweiSWRConfig{
			AccessKey: maskSensitive(registries.HuaweiSWR.AccessKey),
			SecretKey: maskSensitive(registries.HuaweiSWR.SecretKey),
			Region:    registries.HuaweiSWR.Region,
		}
	}

	if registries.Generic != nil {
		safe.Generic = &GenericRegistryConfig{
			Registry:  registries.Generic.Registry,
			Namespace: registries.Generic.Namespace,
			Username:  registries.Generic.Username,
			Password:  maskSensitive(registries.Generic.Password),
		}
	}

	return safe
}

// GetEffectiveHuaweiSWRConfig 获取有效的华为云SWR配置
func (c *Config) GetEffectiveHuaweiSWRConfig() *HuaweiSWRConfig {
	// 只使用新的华为云配置
	return c.Registries.HuaweiSWR
}

// GetEffectiveGenericConfig 获取有效的通用仓库配置
func (c *Config) GetEffectiveGenericConfig() *GenericRegistryConfig {
	// 只使用新的通用配置
	return c.Registries.Generic
}

// HasRegistryConfig 检查是否配置了特定类型的仓库
func (c *Config) HasRegistryConfig(registryType string) bool {
	switch registryType {
	case "huawei_swr":
		return c.GetEffectiveHuaweiSWRConfig() != nil
	case "generic":
		return c.GetEffectiveGenericConfig() != nil
	default:
		// 对于其他仓库类型，检查是否有通用配置
		return c.GetEffectiveGenericConfig() != nil
	}
}

// maskSensitive 对敏感信息进行脱敏处理
func maskSensitive(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
