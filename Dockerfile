# 多阶段构建 Dockerfile for GitHub Actions
# 第一阶段：构建阶段
FROM golang:1.20-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata make

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用程序（使用 Makefile）
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN make build VERSION=${VERSION} COMMIT=${COMMIT} BUILD_TIME=${BUILD_TIME}

# 第二阶段：运行阶段
FROM docker:24-dind

# 安装必要的工具
RUN apk --no-cache add ca-certificates bash curl

# 安装 buildx
RUN docker buildx install

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/build/sync-image ./sync-image

# 复制配置文件
COPY --from=builder /app/configs ./configs

# 复制脚本文件
COPY --from=builder /app/scripts ./scripts

# 设置脚本执行权限
RUN chmod +x scripts/pull-k8s-image.sh

# 创建工作目录
RUN mkdir -p /workspace

# 设置环境变量
ENV DOCKER_BUILDKIT=1
ENV DOCKER_CLI_EXPERIMENTAL=enabled

# 复制启动脚本
COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# 设置入口点
ENTRYPOINT ["/entrypoint.sh"]

# 默认命令参数
CMD ["--help"]
