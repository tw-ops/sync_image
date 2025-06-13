package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"

	"sync-image/internal/config"
	"sync-image/pkg/errors"
	"sync-image/pkg/logger"
	"sync-image/pkg/utils"
)

// Builder Docker 构建器接口
type Builder interface {
	Login(ctx context.Context) error
	BuildAndPush(ctx context.Context, sourceImage, targetImage, platform string) error
	WriteDockerfile(sourceImage string) error
	Cleanup() error
	GetLastArchitectureInfo() string // 获取最后一次构建的架构信息
}

// SDKBuilder 使用 Docker SDK 的构建器实现
type SDKBuilder struct {
	client       *client.Client
	config       *config.DockerConfig
	logger       logger.Logger
	lastArchInfo string // 最后一次构建的架构信息
}

// createDockerClient 创建 Docker 客户端
func createDockerClient(log logger.Logger) (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("创建 Docker 客户端失败: %w", err)
	}

	// 测试连接
	ctx := context.Background()
	_, pingErr := cli.Ping(ctx)
	if pingErr != nil {
		cli.Close()
		return nil, fmt.Errorf("Docker 连接测试失败: %w", pingErr)
	}

	log.Debug("Docker 客户端创建成功")
	return cli, nil
}

// NewBuilder 创建新的 Docker 构建器（使用 SDK 版本）
func NewBuilder(cfg *config.DockerConfig, log logger.Logger) Builder {
	cli, err := createDockerClient(log)
	if err != nil {
		log.Error("创建 Docker 客户端失败，程序无法继续: %v", err)

		// 提供更详细的错误信息和解决建议
		log.Error("Docker 连接失败解决建议:")
		log.Error("1. 确保 Docker daemon 正在运行")
		log.Error("2. 确保当前用户在 docker 组中")
		log.Error("3. 检查 Docker socket 权限: ls -la /var/run/docker.sock")
		log.Error("4. 在容器中运行时，确保正确挂载 Docker socket 并设置用户组")
		log.Error("5. 检查 DOCKER_HOST 环境变量是否正确设置")

		panic(fmt.Sprintf("Docker SDK 不可用: %v", err))
	}

	log.Info("使用 Docker SDK 构建器")
	log.Info("Docker 连接测试成功")
	return &SDKBuilder{
		client: cli,
		config: cfg,
		logger: log,
	}
}

// Login 登录到 Docker 注册表
func (b *SDKBuilder) Login(ctx context.Context) error {
	if !b.hasCredentials() {
		b.logger.Debug("跳过 Docker 登录（无凭据配置）")
		return nil
	}

	registryAddr := b.getRegistryAddress()
	b.logger.Debug("使用 Docker SDK 登录到注册表: `%s`", registryAddr)

	authConfig := b.createAuthConfig()

	_, err := b.client.RegistryLogin(ctx, authConfig)
	if err != nil {
		return errors.NewDockerError("Docker SDK 登录失败", err).
			WithContext("registry", registryAddr).
			WithContext("username", b.config.Username)
	}

	b.logger.Info("成功使用 Docker SDK 登录到注册表")
	return nil
}

// BuildAndPush 构建并推送镜像
func (b *SDKBuilder) BuildAndPush(ctx context.Context, sourceImage, targetImage, platform string) error {
	b.logger.Info("使用 Docker SDK 开始构建镜像: %s -> %s", sourceImage, targetImage)

	// 首先确保 Docker 登录
	if err := b.ensureDockerLogin(ctx); err != nil {
		return fmt.Errorf("Docker 登录失败: %w", err)
	}

	// 检测上游镜像支持的架构
	upstreamArchs, err := b.inspectImageArchitectures(ctx, sourceImage)
	if err != nil {
		b.logger.Warn("无法检测上游镜像架构，使用默认策略: %v", err)
		upstreamArchs = []string{"linux/amd64"} // 默认假设单架构
	}

	// 设置目标平台
	targetPlatforms := b.config.Platforms
	if platform != "" {
		targetPlatforms = platform
	}

	// 根据上游镜像架构和目标平台决定构建策略
	return b.chooseBuildStrategy(ctx, sourceImage, targetImage, targetPlatforms, upstreamArchs)
}

// buildWithBuildx 使用 buildx 进行多架构构建
func (b *SDKBuilder) buildWithBuildx(ctx context.Context, sourceImage, targetImage, platforms string) error {
	// 创建临时 Dockerfile
	if err := b.WriteDockerfile(sourceImage); err != nil {
		return err
	}
	defer b.cleanupDockerfile()

	// 检查 buildx 环境是否可用
	if err := b.checkBuildxEnvironment(); err != nil {
		return fmt.Errorf("buildx 环境检查失败: %w", err)
	}

	// 使用 buildx 命令进行多架构构建
	return b.execBuildxCommand(targetImage, platforms)
}

// buildSingleArch 单架构构建使用 SDK
func (b *SDKBuilder) buildSingleArch(ctx context.Context, sourceImage, targetImage, platform string) error {
	// 创建构建上下文
	buildContext, err := b.createBuildContext(sourceImage)
	if err != nil {
		return errors.NewDockerError("创建构建上下文失败", err)
	}
	defer buildContext.Close()

	// 构建选项
	buildOptions := types.ImageBuildOptions{
		Tags:           []string{targetImage},
		Dockerfile:     "Dockerfile",
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		NoCache:        false,
		SuppressOutput: false,
		Platform:       platform,
	}

	// 执行构建
	buildResponse, err := b.client.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return errors.NewDockerError("Docker SDK 构建失败", err).
			WithContext("source_image", sourceImage).
			WithContext("target_image", targetImage).
			WithContext("platform", platform)
	}
	defer buildResponse.Body.Close()

	// 读取构建输出
	if err := b.readBuildOutput(buildResponse.Body); err != nil {
		return errors.NewDockerError("读取构建输出失败", err)
	}

	// 推送镜像
	if err := b.pushImage(ctx, targetImage); err != nil {
		return errors.NewDockerError("推送镜像失败", err).
			WithContext("target_image", targetImage)
	}

	b.logger.Info("成功使用 Docker SDK 构建并推送单架构镜像: `%s`", targetImage)
	return nil
}

// createBuildContext 创建构建上下文
func (b *SDKBuilder) createBuildContext(sourceImage string) (io.ReadCloser, error) {
	// 创建 Dockerfile 内容
	dockerfileContent := fmt.Sprintf("FROM %s\n", sourceImage)

	// 创建 tar 归档
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	// 添加 Dockerfile 到 tar
	dockerfileHeader := &tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfileContent)),
		Mode: 0644,
	}

	if err := tw.WriteHeader(dockerfileHeader); err != nil {
		return nil, fmt.Errorf("写入 Dockerfile 头失败: %w", err)
	}

	if _, err := tw.Write([]byte(dockerfileContent)); err != nil {
		return nil, fmt.Errorf("写入 Dockerfile 内容失败: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("关闭 tar writer 失败: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// readBuildOutput 读取构建输出
func (b *SDKBuilder) readBuildOutput(reader io.Reader) error {
	decoder := json.NewDecoder(reader)
	for {
		var message struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}

		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("解析构建输出失败: %w", err)
		}

		if message.Error != "" {
			return fmt.Errorf("构建错误: %s", message.Error)
		}

		if message.Stream != "" {
			b.logger.Debug("构建输出: `%s`", strings.TrimSpace(message.Stream))
		}
	}

	return nil
}

// pushImage 推送镜像
func (b *SDKBuilder) pushImage(ctx context.Context, imageName string) error {
	b.logger.Debug("推送镜像: `%s`", imageName)

	// 创建认证配置
	authConfig := b.createAuthConfig()

	// 编码认证信息
	authConfigBytes, err := json.Marshal(authConfig)
	if err != nil {
		return fmt.Errorf("编码认证信息失败: %w", err)
	}
	authStr := base64.URLEncoding.EncodeToString(authConfigBytes)

	// 推送选项
	pushOptions := types.ImagePushOptions{
		RegistryAuth: authStr,
	}

	// 执行推送
	pushResponse, err := b.client.ImagePush(ctx, imageName, pushOptions)
	if err != nil {
		return fmt.Errorf("推送镜像失败: %w", err)
	}
	defer pushResponse.Close()

	// 读取推送输出
	if err := b.readPushOutput(pushResponse); err != nil {
		return fmt.Errorf("读取推送输出失败: %w", err)
	}

	b.logger.Info("成功推送镜像: `%s`", imageName)
	return nil
}

// readPushOutput 读取推送输出
func (b *SDKBuilder) readPushOutput(reader io.Reader) error {
	decoder := json.NewDecoder(reader)
	for {
		var message struct {
			Status   string `json:"status"`
			Progress string `json:"progress"`
			Error    string `json:"error"`
		}

		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("解析推送输出失败: %w", err)
		}

		if message.Error != "" {
			return fmt.Errorf("推送错误: %s", message.Error)
		}

		if message.Status != "" {
			b.logger.Debug("推送状态: %s %s", message.Status, message.Progress)
		}
	}

	return nil
}

// WriteDockerfile 写入 Dockerfile（多架构构建时需要）
func (b *SDKBuilder) WriteDockerfile(sourceImage string) error {
	b.logger.Debug("写入 Dockerfile，源镜像: `%s`", sourceImage)

	content := fmt.Sprintf("FROM %s\n", sourceImage)

	file, err := os.Create("Dockerfile")
	if err != nil {
		return fmt.Errorf("创建 Dockerfile 失败: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("写入 Dockerfile 失败: %w", err)
	}

	b.logger.Debug("成功写入 Dockerfile")
	return nil
}

// cleanupDockerfile 清理 Dockerfile
func (b *SDKBuilder) cleanupDockerfile() {
	if err := os.Remove("Dockerfile"); err != nil && !os.IsNotExist(err) {
		b.logger.Warn("清理 Dockerfile 失败: %v", err)
	}
}

// checkBuildxEnvironment 检查并设置 buildx 环境以支持多平台构建
func (b *SDKBuilder) checkBuildxEnvironment() error {
	b.logger.Debug("检查 buildx 环境")

	// 检查 buildx 是否可用
	checkCmd := exec.Command("docker", "buildx", "version")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("buildx 不可用: %w", err)
	}

	// 检查当前构建器
	lsCmd := exec.Command("docker", "buildx", "ls")
	output, err := lsCmd.Output()
	if err != nil {
		b.logger.Warn("无法列出 buildx 构建器: %v", err)
		// 如果无法列出构建器，尝试创建一个
		return b.ensureMultiPlatformBuilder()
	}

	outputStr := string(output)
	b.logger.Debug("当前 buildx 构建器:\n```\n%s\n```", outputStr)

	// 检查是否有支持多平台的活跃构建器
	if b.hasMultiPlatformBuilder(outputStr) {
		b.logger.Info("发现支持多平台的 buildx 构建器")
		return nil
	}

	// 如果没有合适的构建器，创建一个
	b.logger.Info("当前构建器不支持多平台，创建新的构建器")
	return b.ensureMultiPlatformBuilder()
}

// hasMultiPlatformBuilder 检查是否有支持多平台的构建器
func (b *SDKBuilder) hasMultiPlatformBuilder(output string) bool {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// 查找活跃的构建器（带*标记）
		if strings.Contains(line, "*") {
			// 检查是否是 docker-container 或 kubernetes 驱动
			if strings.Contains(line, "docker-container") || strings.Contains(line, "kubernetes") {
				b.logger.Debug("发现活跃的多平台构建器: %s", strings.TrimSpace(line))
				return true
			}
		}
	}
	return false
}

// ensureMultiPlatformBuilder 确保有支持多平台的构建器
func (b *SDKBuilder) ensureMultiPlatformBuilder() error {
	b.logger.Info("创建支持多平台的 buildx 构建器")

	// 创建新的构建器
	builderName := "multiplatform-builder"
	createCmd := exec.Command("docker", "buildx", "create",
		"--name", builderName,
		"--driver", "docker-container",
		"--driver-opt", "image=moby/buildkit:v0.9.3",
		"--use")

	var createOut bytes.Buffer
	createCmd.Stdout = &createOut
	createCmd.Stderr = &createOut

	if err := createCmd.Run(); err != nil {
		createOutput := createOut.String()
		// 如果构建器已存在，尝试使用它
		if strings.Contains(createOutput, "already exists") {
			b.logger.Debug("构建器已存在，尝试使用现有构建器")
			useCmd := exec.Command("docker", "buildx", "use", builderName)
			if useErr := useCmd.Run(); useErr != nil {
				return fmt.Errorf("使用现有构建器失败: %w", useErr)
			}
		} else {
			b.logger.Error("创建构建器失败，详细输出:\n```\n%s\n```", createOutput)
			return fmt.Errorf("创建 buildx 构建器失败: %w", err)
		}
	}

	// 启动构建器
	b.logger.Debug("启动 buildx 构建器")
	bootstrapCmd := exec.Command("docker", "buildx", "inspect", "--bootstrap")
	var bootstrapOut bytes.Buffer
	bootstrapCmd.Stdout = &bootstrapOut
	bootstrapCmd.Stderr = &bootstrapOut

	if err := bootstrapCmd.Run(); err != nil {
		bootstrapOutput := bootstrapOut.String()
		b.logger.Warn("启动构建器失败，但继续尝试构建: %v\n输出:\n```\n%s\n```", err, bootstrapOutput)
	}

	b.logger.Info("多平台 buildx 构建器设置完成")
	return nil
}

// ensureDockerLogin 确保 Docker 登录（统一的登录方法）
func (b *SDKBuilder) ensureDockerLogin(ctx context.Context) error {
	if !b.hasCredentials() {
		b.logger.Debug("跳过 Docker 登录（无凭据配置）")
		return nil
	}

	registryAddr := b.getRegistryAddress()

	// 1. 首先进行 SDK 登录
	if err := b.Login(ctx); err != nil {
		return fmt.Errorf("Docker SDK 登录失败: %w", err)
	}

	// 2. 然后进行 CLI 登录（用于 buildx）
	return b.ensureCLILogin(registryAddr)
}

// ensureCLILogin 确保 CLI 环境下的 Docker 登录（用于 buildx）
func (b *SDKBuilder) ensureCLILogin(registryAddr string) error {
	b.logger.Debug("确保 CLI 环境下的 Docker 登录: `%s`", registryAddr)

	var loginCmd *exec.Cmd
	if b.config.Registry == "" {
		// Docker Hub 登录
		loginCmd = exec.Command("docker", "login", "-u", b.config.Username, "--password-stdin")
	} else {
		// 私有仓库登录
		loginCmd = exec.Command("docker", "login", b.config.Registry, "-u", b.config.Username, "--password-stdin")
	}

	loginCmd.Stdin = strings.NewReader(b.config.Password)

	var loginOut bytes.Buffer
	loginCmd.Stdout = &loginOut
	loginCmd.Stderr = &loginOut

	if err := loginCmd.Run(); err != nil {
		loginOutput := loginOut.String()
		b.logger.Error("CLI 环境 Docker 登录失败:\n```\n%s\n```", loginOutput)
		return fmt.Errorf("CLI 环境 Docker 登录失败: %w", err)
	}

	b.logger.Debug("CLI 环境 Docker 登录成功")
	return nil
}

// execBuildxCommand 执行 buildx 命令
func (b *SDKBuilder) execBuildxCommand(targetImage, platforms string) error {
	// 构建参数
	args := []string{"buildx", "build"}

	// 设置平台
	args = append(args, "--platform", platforms)

	// 设置标签和其他参数
	args = append(args, "-t", targetImage, "--progress", "plain", ".", "--push")

	b.logger.Debug("执行 Docker buildx 命令: `docker %s`", strings.Join(args, " "))

	// 清理参数以防止注入
	cleanArgs := make([]string, len(args))
	for i, arg := range args {
		cleanArgs[i] = utils.SanitizeString(arg)
	}

	cmd := exec.Command("docker", cleanArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := out.String()

	if err != nil {
		// 记录详细输出到日志，但不包含在错误信息中
		b.logger.Error("buildx 构建失败，详细输出:\n```\n%s\n```", output)
		return fmt.Errorf("buildx 命令执行失败: %w\n命令: `docker %s`",
			err, strings.Join(cleanArgs, " "))
	}

	b.logger.Info("成功执行 buildx 多架构构建")
	return nil
}

// Cleanup 清理资源
func (b *SDKBuilder) Cleanup() error {
	b.logger.Debug("清理 Docker SDK 资源")

	if b.client != nil {
		if err := b.client.Close(); err != nil {
			b.logger.Warn("关闭 Docker 客户端失败: %v", err)
			return err
		}
	}

	return nil
}

// inspectImageArchitectures 检测镜像支持的架构
func (b *SDKBuilder) inspectImageArchitectures(ctx context.Context, imageName string) ([]string, error) {
	b.logger.Debug("检测镜像架构: `%s`", imageName)

	// 直接使用 docker manifest inspect 获取架构信息
	return b.getRemoteImageArchitectures(ctx, imageName)
}

// getRemoteImageArchitectures 从远程获取镜像架构信息
func (b *SDKBuilder) getRemoteImageArchitectures(ctx context.Context, imageName string) ([]string, error) {
	b.logger.Debug("从远程获取镜像架构信息: `%s`", imageName)

	// 使用 docker manifest inspect 命令获取详细信息
	cmd := exec.Command("docker", "manifest", "inspect", imageName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取镜像架构信息: %w", err)
	}

	// 解析 manifest 信息
	return b.parseManifestArchitectures(output)
}

// parseManifestArchitectures 解析 manifest 输出获取架构信息
func (b *SDKBuilder) parseManifestArchitectures(output []byte) ([]string, error) {
	var manifest struct {
		MediaType string `json:"mediaType"`
		Manifests []struct {
			Platform struct {
				Architecture string `json:"architecture"`
				OS           string `json:"os"`
			} `json:"platform"`
		} `json:"manifests"`
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
	}

	if err := json.Unmarshal(output, &manifest); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}

	var architectures []string

	// 检查是否是 manifest list (多架构)
	if len(manifest.Manifests) > 0 {
		for _, m := range manifest.Manifests {
			if m.Platform.OS != "" && m.Platform.Architecture != "" {
				platform := fmt.Sprintf("%s/%s", m.Platform.OS, m.Platform.Architecture)
				architectures = append(architectures, platform)
			}
		}
	} else if manifest.Architecture != "" && manifest.OS != "" {
		// 单架构镜像
		platform := fmt.Sprintf("%s/%s", manifest.OS, manifest.Architecture)
		architectures = append(architectures, platform)
	}

	// 去重和过滤无效架构
	architectures = b.cleanArchitectures(architectures)

	if len(architectures) == 0 {
		return nil, fmt.Errorf("未找到架构信息")
	}

	b.logger.Debug("检测到镜像架构: `%v`", architectures)
	return architectures, nil
}

// chooseBuildStrategy 选择构建策略
func (b *SDKBuilder) chooseBuildStrategy(ctx context.Context, sourceImage, targetImage, targetPlatforms string, upstreamArchs []string) error {
	requestedPlatforms := strings.Split(targetPlatforms, ",")

	// 清理平台字符串
	for i, platform := range requestedPlatforms {
		requestedPlatforms[i] = strings.TrimSpace(platform)
	}

	b.logger.Info("上游镜像支持架构: `%v`", upstreamArchs)
	b.logger.Info("请求构建架构: `%v`", requestedPlatforms)

	// 检查上游镜像是否支持所有请求的架构
	supportedPlatforms := b.filterSupportedPlatforms(requestedPlatforms, upstreamArchs)

	if len(supportedPlatforms) == 0 {
		return errors.NewValidationError(fmt.Sprintf("上游镜像不支持任何请求的架构。上游支持: %v, 请求: %v", upstreamArchs, requestedPlatforms))
	}

	// 生成架构信息
	b.generateArchitectureInfo(upstreamArchs, requestedPlatforms, supportedPlatforms)

	// 如果支持的平台少于请求的平台，记录警告
	if len(supportedPlatforms) < len(requestedPlatforms) {
		unsupported := b.getUnsupportedPlatforms(requestedPlatforms, upstreamArchs)
		b.logger.Warn("上游镜像不支持以下架构，将跳过: `%v`", unsupported)
	}

	actualPlatforms := strings.Join(supportedPlatforms, ",")

	// 根据实际支持的架构数量选择构建策略
	if len(supportedPlatforms) == 1 {
		b.logger.Info("上游为单架构镜像，使用 Docker SDK 构建")
		return b.buildSingleArch(ctx, sourceImage, targetImage, supportedPlatforms[0])
	} else {
		b.logger.Info("上游为多架构镜像，使用 buildx 构建")
		return b.buildWithBuildx(ctx, sourceImage, targetImage, actualPlatforms)
	}
}

// generateArchitectureInfo 生成架构信息
func (b *SDKBuilder) generateArchitectureInfo(upstreamArchs, requestedPlatforms, supportedPlatforms []string) {
	var info strings.Builder

	info.WriteString("🏗️ **架构信息**:\n")
	info.WriteString("```\n")
	info.WriteString(fmt.Sprintf("上游镜像架构: %s\n", strings.Join(upstreamArchs, ", ")))
	info.WriteString(fmt.Sprintf("请求构建架构: %s\n", strings.Join(requestedPlatforms, ", ")))
	info.WriteString(fmt.Sprintf("实际构建架构: %s\n", strings.Join(supportedPlatforms, ", ")))
	info.WriteString("```\n")

	if len(upstreamArchs) == 1 {
		info.WriteString("ℹ️ **说明**: 上游镜像为单架构镜像，同步的也是单架构镜像\n")
	} else {
		info.WriteString("ℹ️ **说明**: 上游镜像为多架构镜像，同步保持多架构\n")
	}

	// 如果有不支持的架构，添加说明
	if len(supportedPlatforms) < len(requestedPlatforms) {
		unsupported := b.getUnsupportedPlatforms(requestedPlatforms, upstreamArchs)
		info.WriteString(fmt.Sprintf("⚠️ **跳过架构**: `%s` (上游不支持)\n", strings.Join(unsupported, ", ")))
	}

	b.lastArchInfo = info.String()
}

// GetLastArchitectureInfo 获取最后一次构建的架构信息
func (b *SDKBuilder) GetLastArchitectureInfo() string {
	return b.lastArchInfo
}

// filterSupportedPlatforms 过滤上游支持的平台
func (b *SDKBuilder) filterSupportedPlatforms(requested, upstream []string) []string {
	var supported []string

	for _, req := range requested {
		for _, up := range upstream {
			if req == up {
				supported = append(supported, req)
				break
			}
		}
	}

	return supported
}

// cleanArchitectures 清理和去重架构列表
func (b *SDKBuilder) cleanArchitectures(architectures []string) []string {
	seen := make(map[string]bool)
	var cleaned []string

	for _, arch := range architectures {
		// 跳过无效的架构
		if strings.Contains(arch, "unknown") || arch == "" {
			continue
		}

		// 去重
		if !seen[arch] {
			seen[arch] = true
			cleaned = append(cleaned, arch)
		}
	}

	return cleaned
}

// createAuthConfig 创建统一的认证配置
func (b *SDKBuilder) createAuthConfig() registry.AuthConfig {
	authConfig := registry.AuthConfig{
		Username:      b.config.Username,
		Password:      b.config.Password,
		ServerAddress: b.config.Registry,
	}

	if b.config.Registry == "" {
		authConfig.ServerAddress = "https://index.docker.io/v1/"
	}

	return authConfig
}

// hasCredentials 检查是否有登录凭据
func (b *SDKBuilder) hasCredentials() bool {
	return b.config.Username != "" && b.config.Password != ""
}

// getRegistryAddress 获取注册表地址
func (b *SDKBuilder) getRegistryAddress() string {
	if b.config.Registry == "" {
		return "Docker Hub"
	}
	return b.config.Registry
}

// getUnsupportedPlatforms 获取不支持的平台
func (b *SDKBuilder) getUnsupportedPlatforms(requested, upstream []string) []string {
	var unsupported []string

	for _, req := range requested {
		found := false
		for _, up := range upstream {
			if req == up {
				found = true
				break
			}
		}
		if !found {
			unsupported = append(unsupported, req)
		}
	}

	return unsupported
}

// ImageTransformer 镜像名称转换器
type ImageTransformer struct {
	parser *utils.ImageNameParser
	rules  map[string]string
	logger logger.Logger
}

// NewImageTransformer 创建新的镜像名称转换器
func NewImageTransformer(rules map[string]string, log logger.Logger) *ImageTransformer {
	return &ImageTransformer{
		parser: utils.NewImageNameParser(rules),
		rules:  rules,
		logger: log,
	}
}

// Transform 转换镜像名称
func (t *ImageTransformer) Transform(originalImage, targetRegistry, targetNamespace string) (sourceImage, targetImage string, err error) {
	t.logger.Debug("开始转换镜像名称: %s", originalImage)

	// 标准化源镜像名称
	sourceImage = t.parser.NormalizeImageName(originalImage)

	// 应用转换规则
	transformedName := t.parser.TransformImageName(sourceImage, t.rules)

	// 构建目标镜像名称
	targetImage = utils.BuildTargetImageName(transformedName, targetRegistry, targetNamespace)

	t.logger.Info("镜像名称转换完成: %s -> %s", sourceImage, targetImage)

	return sourceImage, targetImage, nil
}

// ValidateTransformation 验证转换结果
func (t *ImageTransformer) ValidateTransformation(sourceImage, targetImage string) error {
	if sourceImage == "" {
		return errors.NewValidationError("源镜像名称不能为空")
	}

	if targetImage == "" {
		return errors.NewValidationError("目标镜像名称不能为空")
	}

	if !utils.IsValidImageName(sourceImage) {
		return errors.NewValidationError(fmt.Sprintf("无效的源镜像名称: %s", sourceImage))
	}

	if !utils.IsValidImageName(targetImage) {
		return errors.NewValidationError(fmt.Sprintf("无效的目标镜像名称: %s", targetImage))
	}

	return nil
}
