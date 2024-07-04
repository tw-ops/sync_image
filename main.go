package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/v47/github"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	swr "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2/model"
	region "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2/region"
	"golang.org/x/oauth2"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

var resultTpl = `
{{ if .Success }}
**转换完成**
^^^bash
#原镜像
{{ .OriginImageName }}

#转换后镜像
{{ .TargetImageName }}


#下载并重命名镜像
docker pull {{ .TargetImageName }} {{ if .Platform }} --platform {{ .Platform }} {{ end }}

docker tag  {{ .TargetImageName }} {{ index (split .OriginImageName "@") 0 }}

docker images | grep $(echo {{ .OriginImageName }} |awk -F':' '{print $1}')

^^^
{{ else }}
**转换失败**
详见 [构建任务](https://github.com/{{ .GhUser }}/{{ .Repo }}/actions/runs/{{ .RunId }})
{{ end }}
`

func main() {
	ctx := context.Background()

	var (
		ghToken           = kingpin.Flag("github.token", "Github token.").Short('t').String()
		ghUser            = kingpin.Flag("github.user", "Github Owner.").Short('u').String()
		ghRepo            = kingpin.Flag("github.repo", "Github Repo.").Short('p').String()
		registry          = kingpin.Flag("docker.registry", "Docker Registry.").Short('r').Default("").String()
		registryNamespace = kingpin.Flag("docker.namespace", "Docker Registry Namespace.").Short('n').String()
		registryUserName  = kingpin.Flag("docker.user", "Docker Registry User.").Short('a').String()
		registryPassword  = kingpin.Flag("docker.secret", "Docker Registry Password.").Short('s').String()
		runId             = kingpin.Flag("github.run_id", "Github Run Id.").Short('i').String()
	)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	config := &Config{
		GhToken:           *ghToken,
		GhUser:            *ghUser,
		Repo:              *ghRepo,
		Registry:          *registry,
		RegistryNamespace: *registryNamespace,
		RegistryUserName:  *registryUserName,
		RegistryPassword:  *registryPassword,
		RunId:             *runId,
		Rules: map[string]string{
			"^gcr.io":          "",
			"^docker.io":       "docker",
			"^k8s.gcr.io":      "google-containers",
			"^registry.k8s.io": "google-containers",
			"^quay.io":         "quay",
			"^ghcr.io":         "ghcr",
		},
	}

	rulesFile, err := os.ReadFile("rules.yaml")
	if err == nil {
		rules := make(map[string]string)
		err2 := yaml.Unmarshal(rulesFile, &rules)
		if err2 == nil {
			config.Rules = rules
		}
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.GhToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	cli := github.NewClient(tc)

	issues, err := getIssues(cli, ctx, config)
	if err != nil {
		fmt.Println("查询 Issues 报错,", err.Error())
		os.Exit(-1)
	}
	if len(issues) == 0 {
		fmt.Println("暂无需要搬运的镜像")
		os.Exit(0)
	}

	// 可以用协程，但是懒得写
	issue := issues[0]

	fmt.Println("添加 构建进展 Comment")
	commentIssues(issue, cli, ctx, "[构建进展](https://github.com/"+config.GhUser+"/"+config.Repo+"/actions/runs/"+config.RunId+")")
	err, originImageName, targetImageName, platform := mirrorByIssues(issue, config)
	if err != nil {
		commentErr := commentIssues(issue, cli, ctx, err.Error())
		if commentErr != nil {
			fmt.Println("提交 comment 报错", commentErr)
		}
	}

	result := struct {
		Success         bool
		Registry        string
		RegistryUser    string
		OriginImageName string
		TargetImageName string
		Platform        string
		GhUser          string
		Repo            string
		RunId           string
	}{
		Success:         err == nil,
		Registry:        config.Registry,
		RegistryUser:    config.RegistryUserName,
		OriginImageName: originImageName,
		TargetImageName: targetImageName,
		Platform:        platform,
		GhUser:          *ghUser,
		Repo:            *ghRepo,
		RunId:           *runId,
	}

	var buf bytes.Buffer
	funcmap := template.FuncMap{
		"split": strings.Split,
	}
	tmpl, err := template.New("result").Funcs(funcmap).Parse(resultTpl)
	err = tmpl.Execute(&buf, &result)

	fmt.Println("添加 转换结果 Comment")
	res := buf.String()

	commentIssues(issue, cli, ctx, strings.ReplaceAll(res, "^", "`"))

	fmt.Println("添加 转换结果 Label")
	var labels []string
	if len(platform) > 0 {
		labels = append(labels, "platform")
	}
	issuesAddLabels(issue, cli, ctx, result.Success, labels)

	fmt.Println("关闭 Issues")
	issuesClose(issue, cli, ctx)
}

func issuesClose(issues *github.Issue, cli *github.Client, ctx context.Context) {
	names := strings.Split(*issues.RepositoryURL, "/")
	state := "closed"
	cli.Issues.Edit(ctx, names[len(names)-2], names[len(names)-1], issues.GetNumber(), &github.IssueRequest{
		State: &state,
	})
}
func issuesAddLabels(issues *github.Issue, cli *github.Client, ctx context.Context, success bool, labels []string) {
	names := strings.Split(*issues.RepositoryURL, "/")
	label := "success"
	if !success {
		label = "failed"
	}
	if labels == nil {
		labels = []string{label}
	} else {
		labels = append(labels, label)
	}

	cli.Issues.AddLabelsToIssue(ctx, names[len(names)-2], names[len(names)-1], issues.GetNumber(), labels)
}
func commentIssues(issues *github.Issue, cli *github.Client, ctx context.Context, comment string) error {
	names := strings.Split(*issues.RepositoryURL, "/")
	_, _, err := cli.Issues.CreateComment(ctx, names[len(names)-2], names[len(names)-1], issues.GetNumber(), &github.IssueComment{
		Body: &comment,
	})
	return err
}

func mirrorByIssues(issues *github.Issue, config *Config) (err error, originImageName string, targetImageName string, platform string) {
	// 去掉前缀 [PORTER] 整体去除前后空格
	originImageName = strings.TrimSpace(strings.Replace(*issues.Title, "[PORTER]", "", 1))
	names := strings.Split(originImageName, "|")

	originImageName = names[0]
	if len(names) > 1 {
		platform = names[1]
	}

	if !strings.Contains(originImageName, ".") || (!strings.Contains(originImageName, "/") && strings.Index(originImageName, ".") > strings.Index(originImageName, "/")) {
		originImageName = "docker.io/" + originImageName
	} else if !strings.Contains(originImageName, "/") { // 对于只有名称加标签的情况，如"nginx:alpine"，直接添加默认域名
		originImageName = "docker.io/library/" + originImageName
	}

	targetImageName = originImageName

	if strings.ContainsAny(originImageName, "@") {
		targetImageName = strings.Split(originImageName, "@")[0]
	}

	registrys := []string{}

	for k, v := range config.Rules {
		targetImageName = regexp.MustCompile(k).ReplaceAllString(targetImageName, v)
		registrys = append(registrys, k)
	}

	if strings.EqualFold(targetImageName, originImageName) {
		return errors.New("@" + *issues.GetUser().Login + " 暂不支持同步" + originImageName + ",目前仅支持同步 `" + strings.Join(registrys, " ,") + "`镜像"), originImageName, targetImageName, platform
	}

	segments := strings.Split(targetImageName, "/")
	targetImageName = segments[len(segments)-1]

	if len(config.RegistryNamespace) > 0 {
		targetImageName = config.RegistryNamespace + "/" + targetImageName
	}
	if len(config.Registry) > 0 {
		targetImageName = config.Registry + "/" + targetImageName
	}
	fmt.Println("source:", originImageName, " , target:", targetImageName)

	_, err = execCmd("docker", "login", config.Registry, "-u", config.RegistryUserName, "-p", config.RegistryPassword)
	if err != nil {
		return errors.New("@" + config.GhUser + " ,docker login 报错 `" + err.Error() + "`"), originImageName, targetImageName, platform
	}

	err = writeDockerfile("Dockerfile", originImageName)
	if err != nil {
		return errors.New("写入 Dockerfile 报错" + err.Error() + "`"), originImageName, targetImageName, platform
	}

	// docker buildx build --platform linux/amd64,linux/arm64 -t swr.cn-southwest-2.myhuaweicloud.com/wutong/nginx:alpine . --push
	_, err = execCmd("docker", "buildx", "build", "--platform", "linux/amd64,linux/arm64", "-t", targetImageName, ".", "--push")
	if err != nil {
		return errors.New("docker buildx 构建推送报错 `" + err.Error() + "`"), originImageName, targetImageName, platform
	}

	err = ModifyImagePublic(targetImageName)
	if err != nil {
		return errors.New("@" + *issues.GetUser().Login + " ,调整 repo 权限报错 `" + err.Error() + "`"), originImageName, targetImageName, platform
	}

	return nil, originImageName, targetImageName, platform
}

func writeDockerfile(filePath, image string) error {
	// 指令内容
	content := fmt.Sprintf("FROM %s\n", image)

	// 尝试打开文件，如果不存在则创建
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("打开或创建Dockerfile时发生错误: %w", err)
	}
	defer file.Close()

	// 写入FROM指令到文件
	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("写入Dockerfile时发生错误: %w", err)
	}

	return nil
}

func execCmd(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := out.String()

	if err != nil {
		return output, fmt.Errorf("执行命令时发生错误: '%w', 命令: '%s %s', 输出: '%s'", err, command, strings.Join(args, " "), output)
	}

	return output, nil
}

type Config struct {
	GhToken           string            `yaml:"gh_token"`
	GhUser            string            `yaml:"gh_user"`
	Repo              string            `yaml:"repo"`
	Registry          string            `yaml:"registry"`
	RegistryNamespace string            `yaml:"registry_namespace"`
	RegistryUserName  string            `yaml:"registry_user_name"`
	RegistryPassword  string            `yaml:"registry_password"`
	Rules             map[string]string `yaml:"rules"`
	RunId             string            `yaml:"run_id"`
}

func getIssues(cli *github.Client, ctx context.Context, config *Config) ([]*github.Issue, error) {
	issues, _, err := cli.Issues.ListByRepo(ctx, config.GhUser, config.Repo, &github.IssueListByRepoOptions{
		//State: "closed",
		State:     "open",
		Labels:    []string{"porter"},
		Sort:      "created",
		Direction: "desc",
		// 防止被滥用，每次最多只能拉20条，虽然可以递归，但是没必要。
		//ListOptions: github.ListOptions{Page: 1, PerPage: 20},
		// 考虑了下，每次还是只允许转一个吧
		ListOptions: github.ListOptions{Page: 1, PerPage: 1},
	})
	return issues, err
}

func ModifyImagePublic(targetImageName string) error {
	auth, _ := basic.NewCredentialsBuilder().
		WithAk(os.Getenv("AK")).
		WithSk(os.Getenv("SK")).
		SafeBuild()

	client := swr.NewSwrClient(
		swr.SwrClientBuilder().
			WithRegion(region.ValueOf("cn-southwest-2")).
			WithCredential(auth).
			Build())

	parts := strings.Split(targetImageName, "/")
	namespace := parts[1]
	repo := strings.Split(parts[2], ":")[0]

	request := &model.UpdateRepoRequest{}
	request.Namespace = namespace
	request.Repository = repo
	request.Body = &model.UpdateRepoRequestBody{
		IsPublic: true,
	}
	_, err := client.UpdateRepo(request)
	if err != nil {
		return err
	}
	return nil
}
