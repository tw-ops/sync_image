package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用程序配置结构
type Config struct {
	GitHub    GitHubConfig      `yaml:"github"`
	Docker    DockerConfig      `yaml:"docker"`
	HuaweiSWR HuaweiSWRConfig   `yaml:"huawei_swr"`
	Rules     map[string]string `yaml:"rules"`
	App       AppConfig         `yaml:"app"`
}

// GitHubConfig GitHub 相关配置
type GitHubConfig struct {
	Token string `yaml:"token"`
	User  string `yaml:"user"`
	Repo  string `yaml:"repo"`
	RunID string `yaml:"run_id"`
}

// DockerConfig Docker 相关配置
type DockerConfig struct {
	Registry  string `yaml:"registry"`
	Namespace string `yaml:"namespace"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Platforms string `yaml:"platforms"`
}

// HuaweiSWRConfig 华为云 SWR 配置
type HuaweiSWRConfig struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
}

// AppConfig 应用程序配置
type AppConfig struct {
	LogLevel string `yaml:"log_level"`
	Debug    bool   `yaml:"debug"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Docker: DockerConfig{
			Platforms: "linux/amd64,linux/arm64",
		},
		HuaweiSWR: HuaweiSWRConfig{
			Region: "cn-southwest-2",
		},
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

	// Docker 配置
	if registry := os.Getenv("DOCKER_REGISTRY"); registry != "" {
		config.Docker.Registry = registry
	}
	if namespace := os.Getenv("DOCKER_NAMESPACE"); namespace != "" {
		config.Docker.Namespace = namespace
	}
	if username := os.Getenv("DOCKER_USER"); username != "" {
		config.Docker.Username = username
	}
	if password := os.Getenv("DOCKER_PASSWORD"); password != "" {
		config.Docker.Password = password
	}

	// 华为云 SWR 配置
	if ak := os.Getenv("AK"); ak != "" {
		config.HuaweiSWR.AccessKey = ak
	}
	if sk := os.Getenv("SK"); sk != "" {
		config.HuaweiSWR.SecretKey = sk
	}
	if region := os.Getenv("HUAWEI_REGION"); region != "" {
		config.HuaweiSWR.Region = region
	}

	// 应用配置
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.App.LogLevel = logLevel
	}
	if debug := os.Getenv("DEBUG"); debug != "" {
		config.App.Debug = strings.ToLower(debug) == "true"
	}
}

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
	if config.Docker.Username == "" {
		return fmt.Errorf("Docker username is required")
	}
	if config.Docker.Password == "" {
		return fmt.Errorf("Docker password is required")
	}
	if config.HuaweiSWR.AccessKey == "" {
		return fmt.Errorf("Huawei SWR access key is required")
	}
	if config.HuaweiSWR.SecretKey == "" {
		return fmt.Errorf("Huawei SWR secret key is required")
	}

	return nil
}

// GetSafeConfig 返回脱敏后的配置用于日志记录
func (c *Config) GetSafeConfig() *Config {
	safe := *c
	safe.GitHub.Token = maskSensitive(c.GitHub.Token)
	safe.Docker.Password = maskSensitive(c.Docker.Password)
	safe.HuaweiSWR.AccessKey = maskSensitive(c.HuaweiSWR.AccessKey)
	safe.HuaweiSWR.SecretKey = maskSensitive(c.HuaweiSWR.SecretKey)
	return &safe
}

// maskSensitive 对敏感信息进行脱敏处理
func maskSensitive(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
