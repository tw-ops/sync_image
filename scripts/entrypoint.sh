#!/bin/bash
set -e

# 启动脚本用于 Docker-in-Docker 环境
# 检查是否需要启动 Docker daemon

# 检查 Docker socket 权限
check_docker_permissions() {
    if [ -S /var/run/docker.sock ]; then
        echo "Docker socket found at /var/run/docker.sock"

        # 检查当前用户是否可以访问 Docker socket
        if docker info >/dev/null 2>&1; then
            echo "Docker socket accessible"
            return 0
        else
            echo "Docker socket exists but not accessible"

            # 尝试获取 Docker socket 的组ID
            DOCKER_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo "999")
            echo "Docker socket group ID: $DOCKER_GID"

            # 检查当前用户是否在 docker 组中
            if groups | grep -q "\b$DOCKER_GID\b\|docker"; then
                echo "User is in docker group, but still cannot access socket"
            else
                echo "User is not in docker group (GID: $DOCKER_GID)"
            fi

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
        echo "Warning: Docker socket exists but is not accessible"
        echo "This may cause permission errors during execution"
    fi
fi

# 执行主程序
echo "Starting sync-image with arguments: $@"
exec /app/sync-image "$@"
