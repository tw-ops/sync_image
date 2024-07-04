---
name: 搬运镜像使用这个模板，别瞎改
about: docker镜像搬运工
title: "[PORTER]"
labels: porter
assignees: ''

---

**为了防止被滥用，目前仅仅支持一次同步一个镜像**

**Issues 必须带 `porter` label，** 简单来说就是通过模板创建就没问题，别抖机灵自己瞎弄。

**标题必须为 `[PORTER]镜像名:tag` 的格式，** 例如
- `[PORTER]nginx:alpine`
- `[PORTER]gcr.io/google-containers/federation-controller-manager-arm64:v1.3.1`

**特别的**，默认同步 `arm64` 和 `amd64` 双架构的镜像，如果`上游同步的镜像为单架构镜像`，则同步的多架构镜像`实际还是单架构`

issues的内容无所谓，可以为空

可以参考 [已搬运镜像集锦](https://github.com/tw-ops/sync_image/issues?q=is%3Aissue+label%3Aporter+)

**注意:**

**>>>>>>>>本项目目前仅支持 docker.io、 gcr.io、k8s.gcr.io、docker.io、registry.k8s.io、quay.io、ghcr.io 镜像<<<<<<<<**