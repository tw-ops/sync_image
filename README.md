Google Container Registry Mirror(Google Container Registry镜像加速)
-------

Disclaimer/免责声明
-------
本人郑重承诺
1. 本项目不以盈利为目的，过去，现在，未来都不会用于牟利。
2. 本项目不承诺永久可用（比如包括但不限于 DockerHub 关闭，或者 DockerHub 修改免费计划限制个人免费镜像数量，Github 主动关闭本项目，Github Action 免费计划修改），但会承诺尽量做到向后兼容（也就是后续有新的扩展 Registry 不会改动原有规则导致之前的不可用）。
3. 本项目不承诺所转存的镜像是安全可靠的，本项目只做转存（从上游 Registry pull 镜像，重新打tag，推送到目标 Registry（本项目是推到华为云），不会进行修改（但是转存后的摘要和上游摘要不相同，这是正常的(因为镜像名字变了)），但是如果上游本身就是恶意镜像，那么转存后仍然是恶意镜像。目前支持的 `gcr.io` , `k8s.gcr.io` , `registry.k8s.io` , `quay.io`, `ghcr.io` 好像都是支持个人上传镜像的，在使用镜像前，请自行确认上游是否可靠，应自行避免供应链攻击。
4. 对于 DockerHub 和 Github 某些策略修改导致的 不可预知且不可控因素等导致业务无法拉取镜像而造成损失的，本项目不承担责任。
5. 对于上游恶意镜像或者上游镜像依赖库版本低导致的安全风险 本项目无法识别，删除，停用，过滤，要求使用者自行甄别，本项目不承担责任。

如果不认可上面所述，请不要使用本项目，一旦使用，则视为同意。

Syntax/语法
-------

```bash
# origin / 原镜像名称
gcr.io/namespace/{image}:{tag}
 
# target / 转换后镜像
swr.cn-southwest-2.myhuaweicloud.com/wutongbase/{image}:{tag}
```

Uses/如何拉取新镜像
-------
[创建issues(直接套用模板即可，别自己瞎改labels)](https://github.com/tw-ops/sync_image/issues/new?assignees=&labels=porter&template=porter.md&title=%5BPORTER%5D) ,将自动触发 github actions 进行拉取转推到docker hub

**注意：**

**为了防止被滥用，目前仅仅支持一次同步一个镜像**

**Issues 必须带 `porter` label，** 简单来说就是通过模板创建就没问题，别抖机灵自己瞎弄。

**标题必须为 `[PORTER]镜像名:tag` 的格式，** 例如
- `[PORTER]k8s.gcr.io/federation-controller-manager-arm64:v1.3.1-beta.1`
- `[PORTER]gcr.io/google-containers/federation-controller-manager-arm64:v1.3.1-beta.1`

**特别的**，默认同步 `arm64` 和 `amd64` 双架构的镜像，如果`上游同步的镜像为单架构镜像`，则同步的多架构镜像`实际还是单架构`

issues的内容无所谓，可以为空

可以参考 [已搬运镜像集锦](https://github.com/tw-ops/sync_image/issues?q=is%3Aissue+label%3Aporter+)

**注意:**

本项目目前仅支持 `docker.io`,  `gcr.io` , `k8s.gcr.io` , `registry.k8s.io` , `quay.io`, `ghcr.io` 镜像，其余镜像源可以提 Issues 反馈或者自己 Fork 一份，修改 `rules.yaml`


Fork/分叉代码自行维护
-------

- 必须: <https://github.com/tw-ops/sync_image/fork> 点击连接在自己账号下分叉出 `sync_image` 项目
- 可选: 修改 [./rules.yaml](./rules.yaml) 增加暂未支持的镜像库
- 在 [./settings/actions](../../settings/actions) 的 `Workflow permissions` 选项中，授予读写权限
- 在 [./settings/secrets/actions](../../settings/secrets/actions) 创建自己的参数
- 随便新建个issues，然后在右侧创建个名为 `porter` 和 `question` 的 label，后续通过模板创建时会自动带上

`DOCKER_REGISTRY`: 如果推到 docker hub 为空即可
`DOCKER_NAMESPACE`: 如果推到 docker hub ，则是自己的 docker hub 账号(不带@email部分)，例如我的 tw-ops
`DOCKER_USER`: 如果推到 docker hub,则是 docker hub 账号(不带@email部分)，例如我的 tw-ops
`DOCKER_PASSWORD`: 如果推到 docker hub，则是 docker hub 密码
`HW_AK`: 华为云AK，由于华为云默认镜像为私有，需要调整为公共镜像
`HW_SK`: 华为云SK

### 拉取并转换单个镜像
```shell
# chmod +x pull-k8s-images.sh
cat pull-k8s-images.sh
# 代码如下 ↓↓↓
```

```shell
#!/bin/sh

k8s_img=$1
mirror_img=$(echo ${k8s_img}|
        sed 's/k8s\.gcr\.io/tw-ops\/google-containers/g;s/gcr\.io/tw-ops/g;s/\//\./g;s/ /\n/g;s/tw-ops\./tw-ops\//g' |
        uniq)

sudo docker pull ${mirror_img}
sudo docker tag ${mirror_img} ${k8s_img}
```