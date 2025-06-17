# 容器镜像同步服务

> 支持多种云服务商的容器镜像同步服务，智能检测目标仓库类型并自动选择最优处理方式

## 免责声明

本人郑重承诺：

1. 本项目不以盈利为目的，过去，现在，未来都不会用于牟利。

2. 本项目不承诺永久可用（比如包括但不限于 DockerHub/华为云SWR 关闭，或者 DockerHub/华为云SWR 修改免费计划限制个人免费镜像数量，Github 主动关闭本项目，Github Action 免费计划修改），但会承诺尽量做到向后兼容（也就是后续有新的扩展 Registry 不会改动原有规则导致之前的不可用）。

3. 本项目不承诺所转存的镜像是安全可靠的，本项目只做转存（从上游 Registry pull 镜像，重新打tag，推送到目标 Registry（本项目是推到华为云），不会进行修改（但是转存后的摘要和上游摘要不相同，这是正常的(因为镜像名字变了)），但是如果上游本身就是恶意镜像，那么转存后仍然是恶意镜像。目前支持的 `gcr.io` , `k8s.gcr.io` , `registry.k8s.io` , `quay.io`, `ghcr.io` 好像都是支持个人上传镜像的，在使用镜像前，请自行确认上游是否可靠，应自行避免供应链攻击。

4. 对于 DockerHub 和 Github 某些策略修改导致的不可预知且不可控因素等导致业务无法拉取镜像而造成损失的，本项目不承担责任。

5. 对于上游恶意镜像或者上游镜像依赖库版本低导致的安全风险本项目无法识别，删除，停用，过滤，要求使用者自行甄别，本项目不承担责任。

**如果不认可上面所述，请不要使用本项目，一旦使用，则视为同意。**

## 镜像转换语法

```bash
# 原镜像名称
gcr.io/namespace/{image}:{tag}

# 转换后镜像
swr.cn-southwest-2.myhuaweicloud.com/wutongbase/{image}:{tag}
```

## 如何拉取新镜像

### 创建 Issue 请求

[点击创建 Issue](https://github.com/tw-ops/sync_image/issues/new?assignees=&labels=porter&template=porter.md&title=%5BPORTER%5D)（直接套用模板即可，别自己瞎改labels），将自动触发 GitHub Actions 进行拉取转推到华为云SWR。

### 重要注意事项

> ⚠️ **为了防止被滥用，目前仅仅支持一次同步一个镜像**

- **Issues 必须带 `porter` label** - 简单来说就是通过模板创建就没问题，别抖机灵自己瞎弄
- **标题必须为 `[PORTER]镜像名:tag` 的格式**，例如：
  - `[PORTER]k8s.gcr.io/xxxxxxx:latest`
- **智能架构同步**：
  - 🔍 **自动检测**：系统会自动检测上游镜像支持的架构
  - 🏗️ **智能构建**：根据上游镜像实际支持的架构进行同步
  - 📋 **架构说明**：在结果中显示详细的架构信息和说明
  - ⚡ **性能优化**：单架构镜像使用更快的构建方式
- **Issues 内容无所谓，可以为空**

### 参考示例

可以参考 [已搬运镜像集锦](https://github.com/tw-ops/sync_image/issues?q=is%3Aissue+label%3Aporter+)

### 支持的镜像仓库

目前支持以下镜像仓库：
- `docker.io`
- `gcr.io`
- `k8s.gcr.io`
- `registry.k8s.io`
- `quay.io`
- `ghcr.io`

其余镜像源可以提 Issues 反馈或者自己 Fork 一份，修改 `configs/rules.yaml`

### 架构同步说明

#### 🔍 智能架构检测
- **自动检测上游镜像架构**：系统会自动检测上游镜像支持的架构（如 linux/amd64、linux/arm64）
- **智能构建策略**：根据检测结果选择最优的构建方式
- **架构信息展示**：在 Issue 结果中显示详细的架构对比信息

#### 📋 同步行为
- **多架构镜像** → **保持多架构**：如果上游镜像支持多个架构，同步后也是多架构
- **单架构镜像** → **保持单架构**：如果上游镜像只有单个架构，同步后也是单架构
- **部分支持** → **智能过滤**：如果上游镜像不支持某些架构，会自动跳过并说明

#### 💡 示例说明
```
上游镜像：nginx:latest (支持 linux/amd64, linux/arm64)
同步结果：多架构镜像 (linux/amd64, linux/arm64)

上游镜像：some-image:tag (仅支持 linux/amd64)
同步结果：单架构镜像 (linux/amd64)，自动跳过 linux/arm64
```


## Fork 分叉代码自行维护

### 基本步骤

1. **必须**：[点击 Fork 项目](https://github.com/tw-ops/sync_image/fork) 在自己账号下分叉出 `sync_image` 项目
2. **可选**：修改 [./configs/rules.yaml](./configs/rules.yaml) 增加暂未支持的镜像库
3. 在 [./settings/actions](../../settings/actions) 的 `Workflow permissions` 选项中，授予读写权限
4. 在 [./settings/secrets/actions](../../settings/secrets/actions) 创建自己的参数
5. 随便新建个 Issues，然后在右侧创建个名为 `porter` 和 `question` 的 label，后续通过模板创建时会自动带上

### 环境变量配置

#### 基础配置

| 变量名        | 说明                    | 示例                          |
| ------------- | ----------------------- | ----------------------------- |
| `PLATFORMS`   | 支持的平台架构          | `linux/amd64,linux/arm64`    |

#### 华为云 SWR 配置（可选，用于自动设置镜像公开权限）

| 变量名                   | 说明                                      | 示例               |
| ------------------------ | ----------------------------------------- | ------------------ |
| `HUAWEI_SWR_ACCESS_KEY`  | 华为云 Access Key（可选）                 | -                  |
| `HUAWEI_SWR_SECRET_KEY`  | 华为云 Secret Key（可选）                 | -                  |
| `HUAWEI_SWR_REGION`      | 华为云区域，默认 `cn-southwest-2`         | `cn-southwest-2`   |

**说明：** 华为云配置是可选的。如果配置了，系统会在推送到华为云SWR后自动设置镜像为公开访问。如果未配置，镜像将保持默认的私有状态。

#### 通用仓库配置（推荐）

适用于Docker Hub、私有仓库等所有其他仓库：

| 变量名              | 说明                                    | 示例                                    |
| ------------------ | --------------------------------------- | --------------------------------------- |
| `GENERIC_REGISTRY` | 仓库地址                                | `docker.io`                             |
| `GENERIC_NAMESPACE`| 命名空间（可选）                        | `my-namespace`                          |
| `GENERIC_USERNAME` | 用户名                                  | `docker_username`                       |
| `GENERIC_PASSWORD` | 密码或访问令牌                          | `docker_token`                          |

**使用示例：**

```bash
# Docker Hub
export GENERIC_REGISTRY="docker.io"
export GENERIC_USERNAME="your_docker_username"
export GENERIC_PASSWORD="your_docker_password_or_token"

# 私有仓库
export GENERIC_REGISTRY="your-private-registry.com"
export GENERIC_USERNAME="your_username"
export GENERIC_PASSWORD="your_password"
```

### 架构说明

本项目采用**统一通用处理器 + 后处理机制架构**：

- **单一处理器**：所有仓库（包括华为云SWR、Docker Hub、私有仓库等）都使用同一个通用处理器
- **后处理机制**：推送完成后，系统会自动执行适用的后处理操作
- **华为云后处理**：当推送到华为云SWR时，会自动调用华为云SDK设置镜像为公开访问
- **可扩展设计**：后处理机制支持添加其他云服务商的特殊处理逻辑
- **配置灵活**：所有配置都是可选的，支持匿名访问公共仓库
## 本地脚本使用

### 拉取并转换单个镜像

```bash
# 给脚本执行权限
chmod +x scripts/pull-k8s-image.sh

# 使用脚本拉取镜像
./scripts/pull-k8s-image.sh <镜像名>
```

<details>
<summary>查看脚本内容</summary>

```bash
#!/bin/sh

k8s_img=$1
mirror_img=$(echo ${k8s_img}|
        sed 's/quay\.io/tw-ops/\/quay/g;s/ghcr\.io/tw-ops/\/ghcr/g;s/registry\.k8s\.io/tw-ops/\/google-containers/g;s/k8s\.gcr\.io/tw-ops/\/google-containers/g;s/gcr\.io/tw-ops//g;s/\//\./g;s/ /\n/g;s/tw-ops/\./tw-ops/\//g' |
        uniq)

if [ -x "$(command -v docker)" ]; then
  sudo docker pull ${mirror_img}
  sudo docker tag ${mirror_img} ${k8s_img}
  exit 0
fi

if [ -x "$(command -v ctr)" ]; then
  sudo ctr -n k8s.io image pull docker.io/${mirror_img}
  sudo ctr -n k8s.io image tag docker.io/${mirror_img} ${k8s_img}
  exit 0
fi

echo "command not found:docker or ctr"
```

</details>

## 高级配置

项目支持多种配置方式，优先级从高到低：

1. **命令行参数**
2. **环境变量**
3. **配置文件**

可以复制 `configs/config.example.yaml` 为 `configs/config.yaml` 并根据需要修改配置。

### 架构配置

默认情况下，系统会尝试构建 `linux/amd64,linux/arm64` 两个架构，但实际构建的架构取决于上游镜像的支持情况：

- **自动适应**：系统会自动检测上游镜像支持的架构
- **智能过滤**：只构建上游镜像实际支持的架构
- **性能优化**：单架构镜像使用更快的构建方式

如需自定义架构，可以在配置文件中修改 `platforms` 参数：
```yaml
platforms: "linux/amd64,linux/arm64,linux/s390x"
```

## 本地构建和使用

```bash
# 克隆项目
git clone https://github.com/tw-ops/sync_image.git
cd sync_image

# 构建项目
make build
# 或者
go build -o sync-image cmd/sync/main.go

# 查看帮助
./build/sync-image --help

# 查看版本信息
./build/sync-image --version
```

## Docker 使用

### 本地使用

```bash
# 构建 Docker 镜像
docker build -t sync-image:latest .

# 运行容器
docker run --rm sync-image:latest --help

# 构建 CI 版本
docker build -f Dockerfile.ci -t sync-image:ci .
```

### GitHub Container Registry

项目自动构建并发布 Docker 镜像到 GitHub Container Registry：

```bash
# 拉取最新镜像
docker pull ghcr.io/tw-ops/sync_image:latest

# 运行镜像同步（需要 Docker socket）
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e GITHUB_TOKEN=your_token \
  ghcr.io/tw-ops/sync_image:latest \
  --config=configs/rules.yaml \
  --github.user=your_user \
  --github.repo=your_repo
```

## 常见问题

### Q: 为什么我的镜像只同步了部分架构？
A: 系统会自动检测上游镜像支持的架构。如果上游镜像只支持 `linux/amd64`，那么同步后的镜像也只有 `linux/amd64`。这是正常行为，可以在 Issue 结果中查看详细的架构信息。

### Q: 如何知道上游镜像支持哪些架构？
A: 在 Issue 结果中会显示完整的架构信息，包括：
- 🏗️ **上游镜像架构**：上游镜像实际支持的架构
- 📋 **请求构建架构**：系统尝试构建的架构
- ✅ **实际构建架构**：最终成功构建的架构

### Q: 可以强制构建不支持的架构吗？
A: 不可以。系统会智能过滤掉上游镜像不支持的架构，这样可以避免构建失败，节省时间和资源。

### Q: 单架构和多架构镜像的构建有什么区别？
A:
- **单架构镜像**：使用 Docker SDK 构建，速度更快
- **多架构镜像**：使用 buildx 构建，支持多架构特性
- 系统会自动选择最优的构建方式

---

<div align="center">

**如果这个项目对你有帮助，请给个 ⭐ Star 支持一下！**

</div>