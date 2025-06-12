package utils

import (
	"regexp"
	"strings"
)

// ImageNameParser 镜像名称解析器
type ImageNameParser struct {
	compiledRules map[string]*regexp.Regexp
}

// NewImageNameParser 创建新的镜像名称解析器
func NewImageNameParser(rules map[string]string) *ImageNameParser {
	compiled := make(map[string]*regexp.Regexp)
	for pattern := range rules {
		if re, err := regexp.Compile(pattern); err == nil {
			compiled[pattern] = re
		}
	}

	return &ImageNameParser{
		compiledRules: compiled,
	}
}

// NormalizeImageName 标准化镜像名称
func (p *ImageNameParser) NormalizeImageName(imageName string) string {
	imageName = strings.TrimSpace(imageName)

	// 处理 Docker Hub 默认镜像
	if !strings.Contains(imageName, ".") ||
		(!strings.Contains(imageName, "/") && strings.Index(imageName, ".") > strings.Index(imageName, "/")) {
		imageName = "docker.io/" + imageName
	} else if !strings.Contains(imageName, "/") {
		// 对于只有名称加标签的情况，如"nginx:alpine"，直接添加默认域名
		imageName = "docker.io/library/" + imageName
	}

	return imageName
}

// TransformImageName 根据规则转换镜像名称
func (p *ImageNameParser) TransformImageName(imageName string, rules map[string]string) string {
	result := imageName

	// 移除摘要部分
	if strings.Contains(result, "@") {
		result = strings.Split(result, "@")[0]
	}

	// 应用转换规则
	for pattern, replacement := range rules {
		if re, exists := p.compiledRules[pattern]; exists {
			result = re.ReplaceAllString(result, replacement)
		}
	}

	return result
}

// ExtractImageInfo 从镜像名称中提取信息
func ExtractImageInfo(imageName string) (registry, namespace, repository, tag string) {
	// 分离标签
	parts := strings.Split(imageName, ":")
	nameWithoutTag := parts[0]
	if len(parts) > 1 {
		tag = parts[1]
	} else {
		tag = "latest"
	}

	// 分离注册表、命名空间和仓库
	segments := strings.Split(nameWithoutTag, "/")

	switch len(segments) {
	case 1:
		// 只有仓库名，默认为 Docker Hub
		registry = "docker.io"
		namespace = "library"
		repository = segments[0]
	case 2:
		if strings.Contains(segments[0], ".") {
			// 第一部分包含点，认为是注册表
			registry = segments[0]
			namespace = ""
			repository = segments[1]
		} else {
			// 默认为 Docker Hub
			registry = "docker.io"
			namespace = segments[0]
			repository = segments[1]
		}
	case 3:
		registry = segments[0]
		namespace = segments[1]
		repository = segments[2]
	default:
		// 更多段，取第一个为注册表，最后一个为仓库，中间的合并为命名空间
		registry = segments[0]
		repository = segments[len(segments)-1]
		namespace = strings.Join(segments[1:len(segments)-1], "/")
	}

	return
}

// BuildTargetImageName 构建目标镜像名称
func BuildTargetImageName(transformedName, targetRegistry, targetNamespace string) string {
	// 提取最后一个段作为仓库名
	segments := strings.Split(transformedName, "/")
	repository := segments[len(segments)-1]

	var result string
	if targetNamespace != "" {
		result = targetNamespace + "/" + repository
	} else {
		result = repository
	}

	if targetRegistry != "" {
		result = targetRegistry + "/" + result
	}

	return result
}

// ParseIssueTitle 解析 Issue 标题
func ParseIssueTitle(title string) (imageName, platform string) {
	// 去掉前缀 [PORTER] 并去除前后空格
	cleaned := strings.TrimSpace(strings.Replace(title, "[PORTER]", "", 1))

	// 检查是否包含平台信息
	parts := strings.Split(cleaned, "|")
	imageName = strings.TrimSpace(parts[0])

	if len(parts) > 1 {
		platform = strings.TrimSpace(parts[1])
	}

	return
}

// ExtractRepoInfo 从 GitHub 仓库 URL 中提取所有者和仓库名
func ExtractRepoInfo(repoURL string) (owner, repo string) {
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		owner = parts[len(parts)-2]
		repo = parts[len(parts)-1]
	}
	return
}

// SanitizeString 清理字符串，移除潜在的危险字符
func SanitizeString(s string) string {
	// 移除控制字符和潜在的注入字符
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "&", "")
	s = strings.ReplaceAll(s, "|", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "$", "")

	return strings.TrimSpace(s)
}

// IsValidImageName 验证镜像名称是否有效
func IsValidImageName(imageName string) bool {
	if imageName == "" {
		return false
	}

	// 长度限制
	if len(imageName) > 255 {
		return false
	}

	// 基本的镜像名称格式验证
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]*[a-zA-Z0-9]$`)
	if !validPattern.MatchString(imageName) {
		return false
	}

	// 检查是否包含危险字符
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "[", "]", "<", ">"}
	for _, char := range dangerousChars {
		if strings.Contains(imageName, char) {
			return false
		}
	}

	return true
}
