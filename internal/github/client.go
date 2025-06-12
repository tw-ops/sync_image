package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v47/github"
	"golang.org/x/oauth2"

	"sync-image/internal/config"
	"sync-image/pkg/errors"
	"sync-image/pkg/logger"
	"sync-image/pkg/utils"
)

// Client GitHub 客户端接口
type Client interface {
	GetPendingIssues(ctx context.Context) ([]*github.Issue, error)
	AddComment(ctx context.Context, issue *github.Issue, comment string) error
	AddLabels(ctx context.Context, issue *github.Issue, labels []string) error
	CloseIssue(ctx context.Context, issue *github.Issue) error
}

// DefaultClient 默认 GitHub 客户端实现
type DefaultClient struct {
	client *github.Client
	config *config.GitHubConfig
	logger logger.Logger
}

// NewClient 创建新的 GitHub 客户端
func NewClient(cfg *config.GitHubConfig, log logger.Logger) Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.Token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	
	return &DefaultClient{
		client: github.NewClient(tc),
		config: cfg,
		logger: log,
	}
}

// GetPendingIssues 获取待处理的 Issues
func (c *DefaultClient) GetPendingIssues(ctx context.Context) ([]*github.Issue, error) {
	c.logger.Debug("获取待处理的 Issues")
	
	issues, _, err := c.client.Issues.ListByRepo(ctx, c.config.User, c.config.Repo, &github.IssueListByRepoOptions{
		State:     "open",
		Labels:    []string{"porter"},
		Sort:      "created",
		Direction: "desc",
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 1, // 每次只处理一个 Issue
		},
	})
	
	if err != nil {
		return nil, errors.NewGitHubError("获取 Issues 失败", err)
	}
	
	c.logger.Info("找到 %d 个待处理的 Issues", len(issues))
	return issues, nil
}

// AddComment 为 Issue 添加评论
func (c *DefaultClient) AddComment(ctx context.Context, issue *github.Issue, comment string) error {
	owner, repo := utils.ExtractRepoInfo(*issue.RepositoryURL)
	
	c.logger.Debug("为 Issue #%d 添加评论", issue.GetNumber())
	
	_, _, err := c.client.Issues.CreateComment(ctx, owner, repo, issue.GetNumber(), &github.IssueComment{
		Body: &comment,
	})
	
	if err != nil {
		return errors.NewGitHubError(
			fmt.Sprintf("为 Issue #%d 添加评论失败", issue.GetNumber()),
			err,
		).WithContext("issue_number", issue.GetNumber())
	}
	
	c.logger.Info("成功为 Issue #%d 添加评论", issue.GetNumber())
	return nil
}

// AddLabels 为 Issue 添加标签
func (c *DefaultClient) AddLabels(ctx context.Context, issue *github.Issue, labels []string) error {
	owner, repo := utils.ExtractRepoInfo(*issue.RepositoryURL)
	
	c.logger.Debug("为 Issue #%d 添加标签: %v", issue.GetNumber(), labels)
	
	_, _, err := c.client.Issues.AddLabelsToIssue(ctx, owner, repo, issue.GetNumber(), labels)
	
	if err != nil {
		return errors.NewGitHubError(
			fmt.Sprintf("为 Issue #%d 添加标签失败", issue.GetNumber()),
			err,
		).WithContext("issue_number", issue.GetNumber()).WithContext("labels", labels)
	}
	
	c.logger.Info("成功为 Issue #%d 添加标签: %v", issue.GetNumber(), labels)
	return nil
}

// CloseIssue 关闭 Issue
func (c *DefaultClient) CloseIssue(ctx context.Context, issue *github.Issue) error {
	owner, repo := utils.ExtractRepoInfo(*issue.RepositoryURL)
	
	c.logger.Debug("关闭 Issue #%d", issue.GetNumber())
	
	state := "closed"
	_, _, err := c.client.Issues.Edit(ctx, owner, repo, issue.GetNumber(), &github.IssueRequest{
		State: &state,
	})
	
	if err != nil {
		return errors.NewGitHubError(
			fmt.Sprintf("关闭 Issue #%d 失败", issue.GetNumber()),
			err,
		).WithContext("issue_number", issue.GetNumber())
	}
	
	c.logger.Info("成功关闭 Issue #%d", issue.GetNumber())
	return nil
}

// IssueProcessor Issue 处理器
type IssueProcessor struct {
	client Client
	config *config.GitHubConfig
	logger logger.Logger
}

// NewIssueProcessor 创建新的 Issue 处理器
func NewIssueProcessor(client Client, cfg *config.GitHubConfig, log logger.Logger) *IssueProcessor {
	return &IssueProcessor{
		client: client,
		config: cfg,
		logger: log,
	}
}

// ProcessIssue 处理单个 Issue
func (p *IssueProcessor) ProcessIssue(ctx context.Context, issue *github.Issue) (imageName, platform string, err error) {
	p.logger.Info("开始处理 Issue #%d: %s", issue.GetNumber(), issue.GetTitle())
	
	// 添加构建进展评论
	buildURL := fmt.Sprintf("https://github.com/%s/%s/actions/runs/%s", 
		p.config.User, p.config.Repo, p.config.RunID)
	progressComment := fmt.Sprintf("[构建进展](%s)", buildURL)
	
	if err := p.client.AddComment(ctx, issue, progressComment); err != nil {
		p.logger.Warn("添加构建进展评论失败: %v", err)
	}
	
	// 解析 Issue 标题
	imageName, platform = utils.ParseIssueTitle(issue.GetTitle())
	
	// 验证镜像名称
	if !utils.IsValidImageName(imageName) {
		return "", "", errors.NewValidationError(
			fmt.Sprintf("无效的镜像名称: %s", imageName),
		)
	}
	
	// 清理镜像名称
	imageName = utils.SanitizeString(imageName)
	platform = utils.SanitizeString(platform)
	
	p.logger.Info("解析得到镜像名称: %s, 平台: %s", imageName, platform)
	
	return imageName, platform, nil
}

// FinishIssue 完成 Issue 处理
func (p *IssueProcessor) FinishIssue(ctx context.Context, issue *github.Issue, success bool, result string, platform string) error {
	// 添加结果评论
	if err := p.client.AddComment(ctx, issue, result); err != nil {
		p.logger.Error("添加结果评论失败: %v", err)
	}
	
	// 添加标签
	labels := []string{}
	if success {
		labels = append(labels, "success")
	} else {
		labels = append(labels, "failed")
	}
	
	if platform != "" {
		labels = append(labels, "platform")
	}
	
	if err := p.client.AddLabels(ctx, issue, labels); err != nil {
		p.logger.Error("添加标签失败: %v", err)
	}
	
	// 关闭 Issue
	if err := p.client.CloseIssue(ctx, issue); err != nil {
		p.logger.Error("关闭 Issue 失败: %v", err)
		return err
	}
	
	return nil
}
