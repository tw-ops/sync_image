// Package main provides the main entry point for the image sync tool
//
// This is a Docker image synchronization tool that syncs images from
// Google Container Registry (gcr.io), Kubernetes Registry (k8s.gcr.io),
// Quay.io, GitHub Container Registry (ghcr.io) and other foreign registries
// to Huawei SWR registry.
//
// The tool listens to GitHub Issues to trigger image synchronization and
// supports multi-architecture image builds.
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
	// Version information (injected at build time)
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"

	// Command line parameters
	configFile  = kingpin.Flag("config", "Configuration file path").Short('c').String()
	showVersion = kingpin.Flag("version", "Show version information").Short('v').Bool()

	// GitHub parameters
	githubToken = kingpin.Flag("github.token", "GitHub token").Short('t').String()
	githubUser  = kingpin.Flag("github.user", "GitHub Owner").Short('u').String()
	githubRepo  = kingpin.Flag("github.repo", "GitHub Repo").Short('p').String()
	githubRunID = kingpin.Flag("github.run_id", "GitHub Run ID").Short('i').String()

	// Application parameters
	logLevel = kingpin.Flag("log.level", "Log level").Default("info").String()
	debug    = kingpin.Flag("debug", "Enable debug mode").Bool()
)

func main() {
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// Show version information
	if *showVersion {
		// Create temporary logger for version info
		versionLogger := logger.NewLogger("info")
		versionLogger.Info("sync-image version %s", Version)
		versionLogger.Info("Git commit: %s", Commit)
		versionLogger.Info("Build time: %s", BuildTime)
		os.Exit(0)
	}

	// Initialize logger
	log := logger.NewLogger(*logLevel)
	logger.SetGlobalLogger(log)

	if *debug {
		log.SetLevel(logger.DEBUG)
	}

	log.Info("Starting image sync tool v%s (commit: %s)", Version, Commit)

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Error("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	// Print safe configuration information
	safeConfig := cfg.GetSafeConfig()
	log.Debug("Loaded configuration: %+v", safeConfig)

	// Create application instance
	app, err := createApp(cfg, log)
	if err != nil {
		log.Error("Failed to create application instance: %v", err)
		os.Exit(1)
	}

	// Ensure resource cleanup
	defer func() {
		if err := app.syncService.Cleanup(); err != nil {
			log.Error("Failed to cleanup resources: %v", err)
		}
	}()

	// Run application
	ctx := context.Background()
	if err := app.syncService.ProcessIssues(ctx); err != nil {
		log.Error("Failed to process Issues: %v", err)
		os.Exit(1)
	}

	log.Info("Image synchronization completed successfully")
}

// loadConfig loads configuration
func loadConfig() (*config.Config, error) {
	// Use original LoadConfig function without validation
	cfg, err := loadConfigWithoutValidation(*configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Load configuration from environment variables (higher priority than config file)
	loadConfigFromEnvironment(cfg)

	// Command line parameters have highest priority over config file and environment variables
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
	// Docker command line parameters have been completely removed, use config file or environment variables

	return cfg, nil
}

// createBuilderConfig creates builder configuration from config
func createBuilderConfig(cfg *config.Config) *docker.BuilderConfig {
	// Prefer using generic configuration
	if genericConfig := cfg.GetEffectiveGenericConfig(); genericConfig != nil {
		return &docker.BuilderConfig{
			Registry:  genericConfig.Registry,
			Namespace: genericConfig.Namespace,
			Username:  genericConfig.Username,
			Password:  genericConfig.Password,
			Platforms: cfg.Platforms,
		}
	}

	// If no generic configuration, create default configuration
	return &docker.BuilderConfig{
		Registry:  "",
		Namespace: "",
		Username:  "",
		Password:  "",
		Platforms: cfg.Platforms,
	}
}

// loadConfigWithoutValidation loads configuration without validation
func loadConfigWithoutValidation(configPath string) (*config.Config, error) {
	// Directly use config package LoadConfig function without validation
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// If validation fails, ignore it because command line parameters will override
		if fmt.Sprintf("%v", err) != "config validation failed: GitHub token is required" {
			return nil, err
		}
		// If validation error, create default configuration and load from environment
		cfg = config.DefaultConfig()
		// Load configuration from environment variables
		loadConfigFromEnvironment(cfg)
	}

	return cfg, nil
}

// loadConfigFromEnvironment loads configuration from environment variables
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
	// No longer support DOCKER_* environment variables, use GENERIC_* environment variables
	// No longer support old Huawei cloud environment variables, use HUAWEI_SWR_* environment variables
}

// App application structure
type App struct {
	config      *config.Config
	syncService service.SyncService
	logger      logger.Logger
}

// createApp creates application instance
func createApp(cfg *config.Config, log logger.Logger) (*App, error) {
	// Create GitHub client
	githubClient := githubclient.NewClient(&cfg.GitHub, log)
	issueProcessor := githubclient.NewIssueProcessor(githubClient, &cfg.GitHub, log)

	// Create Docker builder configuration
	builderConfig := createBuilderConfig(cfg)
	dockerBuilder := docker.NewBuilder(builderConfig, log)
	imageTransformer := docker.NewImageTransformer(cfg.Rules, log)

	// Create registry manager factory
	registryFactory := registry.NewRegistryManagerFactory(cfg, log)

	// Create sync service
	syncService := service.NewSyncService(
		cfg,
		githubClient,
		issueProcessor,
		dockerBuilder,
		imageTransformer,
		registryFactory,
		log,
	)

	return &App{
		config:      cfg,
		syncService: syncService,
		logger:      log,
	}, nil
}
