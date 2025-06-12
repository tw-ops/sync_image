// Package main 镜像同步服务主程序
//
// 这是一个 Docker 镜像同步工具，主要功能是将 Google Container Registry (gcr.io)、
// Kubernetes Registry (k8s.gcr.io)、Quay.io、GitHub Container Registry (ghcr.io)
// 等国外镜像仓库的镜像同步到华为云 SWR 镜像仓库。
//
// 程序通过监听 GitHub Issues 来触发镜像同步任务，支持多架构镜像构建和推送。
package main

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"sync-image/internal/config"
	"sync-image/internal/docker"
	githubclient "sync-image/internal/github"
	"sync-image/internal/registry"
	"sync-image/internal/service"
	"sync-image/pkg/logger"
)

var (
	// 版本信息（构建时注入）
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"

	// 命令行参数
	configFile  = kingpin.Flag("config", "配置文件路径").Short('c').String()
	showVersion = kingpin.Flag("version", "显示版本信息").Short('v').Bool()

	// GitHub 参数
	githubToken = kingpin.Flag("github.token", "GitHub token").Short('t').String()
	githubUser  = kingpin.Flag("github.user", "GitHub Owner").Short('u').String()
	githubRepo  = kingpin.Flag("github.repo", "GitHub Repo").Short('p').String()
	githubRunID = kingpin.Flag("github.run_id", "GitHub Run ID").Short('i').String()

	// Docker 参数
	dockerRegistry  = kingpin.Flag("docker.registry", "Docker Registry").Short('r').Default("").String()
	dockerNamespace = kingpin.Flag("docker.namespace", "Docker Registry Namespace").Short('n').String()
	dockerUsername  = kingpin.Flag("docker.user", "Docker Registry User").Short('a').String()
	dockerPassword  = kingpin.Flag("docker.secret", "Docker Registry Password").Short('s').String()

	// 应用参数
	logLevel = kingpin.Flag("log.level", "日志级别").Default("info").String()
	debug    = kingpin.Flag("debug", "启用调试模式").Bool()
)

func main() {
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("sync-image version %s\n", Version)
		fmt.Printf("Git commit: %s\n", Commit)
		fmt.Printf("Build time: %s\n", BuildTime)
		os.Exit(0)
	}

	// 初始化日志
	log := logger.NewLogger(*logLevel)
	logger.SetGlobalLogger(log)

	if *debug {
		log.SetLevel(logger.DEBUG)
	}

	log.Info("启动镜像同步服务 v%s (commit: %s)", Version, Commit)

	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		log.Error("加载配置失败: %v", err)
		os.Exit(1)
	}

	// 打印安全配置信息
	safeConfig := cfg.GetSafeConfig()
	log.Debug("配置加载完成: %+v", safeConfig)

	// 创建应用实例
	app, err := createApp(cfg, log)
	if err != nil {
		log.Error("创建应用实例失败: %v", err)
		os.Exit(1)
	}

	// 确保清理资源
	defer func() {
		if err := app.syncService.Cleanup(); err != nil {
			log.Error("清理资源失败: %v", err)
		}
	}()

	// 运行应用
	ctx := context.Background()
	if err := app.syncService.ProcessIssues(ctx); err != nil {
		log.Error("处理 Issues 失败: %v", err)
		os.Exit(1)
	}

	log.Info("镜像同步服务完成")
}

// loadConfig 加载配置
func loadConfig() (*config.Config, error) {
	// 使用原有的 LoadConfig，但跳过验证
	cfg, err := loadConfigWithoutValidation(*configFile)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	// 从环境变量加载配置（如果配置文件加载成功，这里会补充环境变量）
	loadConfigFromEnvironment(cfg)

	// 命令行参数覆盖配置文件和环境变量（最高优先级）
	if *githubToken != "" {
		cfg.GitHub.Token = *githubToken
	}
	if *githubUser != "" {
		cfg.GitHub.User = *githubUser
	}
	if *githubRepo != "" {
		cfg.GitHub.Repo = *githubRepo
	}
	if *githubRunID != "" {
		cfg.GitHub.RunID = *githubRunID
	}
	if *dockerRegistry != "" {
		cfg.Docker.Registry = *dockerRegistry
	}
	if *dockerNamespace != "" {
		cfg.Docker.Namespace = *dockerNamespace
	}
	if *dockerUsername != "" {
		cfg.Docker.Username = *dockerUsername
	}
	if *dockerPassword != "" {
		cfg.Docker.Password = *dockerPassword
	}

	return cfg, nil
}

// loadConfigWithoutValidation 加载配置但不进行验证
func loadConfigWithoutValidation(configPath string) (*config.Config, error) {
	// 直接使用 config 包的 LoadConfig，但忽略验证错误
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// 如果是验证错误，我们忽略它，因为命令行参数会覆盖
		if fmt.Sprintf("%v", err) != "config validation failed: GitHub token is required" {
			return nil, err
		}
		// 如果是验证错误，创建默认配置并从环境变量加载
		cfg = config.DefaultConfig()
		// 从环境变量加载所有配置
		loadConfigFromEnvironment(cfg)
	}

	return cfg, nil
}

// loadConfigFromEnvironment 从环境变量加载配置
func loadConfigFromEnvironment(cfg *config.Config) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cfg.GitHub.Token = token
	}
	if user := os.Getenv("GITHUB_USER"); user != "" {
		cfg.GitHub.User = user
	}
	if repo := os.Getenv("GITHUB_REPO"); repo != "" {
		cfg.GitHub.Repo = repo
	}
	if runID := os.Getenv("GITHUB_RUN_ID"); runID != "" {
		cfg.GitHub.RunID = runID
	}
	if registry := os.Getenv("DOCKER_REGISTRY"); registry != "" {
		cfg.Docker.Registry = registry
	}
	if namespace := os.Getenv("DOCKER_NAMESPACE"); namespace != "" {
		cfg.Docker.Namespace = namespace
	}
	if username := os.Getenv("DOCKER_USER"); username != "" {
		cfg.Docker.Username = username
	}
	if password := os.Getenv("DOCKER_PASSWORD"); password != "" {
		cfg.Docker.Password = password
	}
	if ak := os.Getenv("AK"); ak != "" {
		cfg.HuaweiSWR.AccessKey = ak
	}
	if sk := os.Getenv("SK"); sk != "" {
		cfg.HuaweiSWR.SecretKey = sk
	}
}

// App 应用程序结构
type App struct {
	config      *config.Config
	syncService service.SyncService
	logger      logger.Logger
}

// createApp 创建应用程序实例
func createApp(cfg *config.Config, log logger.Logger) (*App, error) {
	// 创建 GitHub 客户端
	githubClient := githubclient.NewClient(&cfg.GitHub, log)
	issueProcessor := githubclient.NewIssueProcessor(githubClient, &cfg.GitHub, log)

	// 创建 Docker 构建器
	dockerBuilder := docker.NewBuilder(&cfg.Docker, log)
	imageTransformer := docker.NewImageTransformer(cfg.Rules, log)

	// 创建华为云 SWR 客户端
	huaweiClient, err := registry.NewHuaweiSWRClient(&cfg.HuaweiSWR, log)
	if err != nil {
		return nil, fmt.Errorf("创建华为云 SWR 客户端失败: %w", err)
	}
	registryManager := registry.NewRegistryManager(huaweiClient, log)

	// 创建同步服务
	syncService := service.NewSyncService(
		cfg,
		githubClient,
		issueProcessor,
		dockerBuilder,
		imageTransformer,
		registryManager,
		log,
	)

	return &App{
		config:      cfg,
		syncService: syncService,
		logger:      log,
	}, nil
}
