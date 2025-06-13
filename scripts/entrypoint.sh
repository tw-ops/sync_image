#!/bin/bash
set -e

# 启动脚本用于 Docker-in-Docker 环境
# 检查是否需要启动 Docker daemon

# 检查 Docker socket 权限（简化版，适用于 root 用户）
check_docker_permissions() {
    if [ -S /var/run/docker.sock ]; then
        echo "Docker socket found at /var/run/docker.sock"

        # 检查是否可以访问 Docker socket
        if docker info >/dev/null 2>&1; then
            echo "Docker socket accessible"
            return 0
        else
            echo "Docker socket exists but not accessible"
            return 1
        fi
    else
        echo "Docker socket not found, may need to start Docker daemon"
        return 1
    fi
}

# 启动 Docker daemon（如果需要）
if ! check_docker_permissions; then
    if [ ! -S /var/run/docker.sock ]; then
        echo "Starting Docker daemon..."
        dockerd-entrypoint.sh &

        # 等待 Docker daemon 启动
        echo "Waiting for Docker daemon to start..."
        while ! docker info >/dev/null 2>&1; do
            echo "Docker daemon not ready, waiting..."
            sleep 2
        done
        echo "Docker daemon started successfully"
    else
        echo "Warning: Docker socket exists but not accessible (running as root should resolve this)"
    fi
fi

# 执行主程序
echo "Starting sync-image with arguments: $@"
exec /app/sync-image "$@"
