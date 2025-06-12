#!/bin/bash
set -e

# 启动脚本用于 Docker-in-Docker 环境
# 检查是否需要启动 Docker daemon

# 启动 Docker daemon（如果需要）
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
fi

# 执行主程序
echo "Starting sync-image with arguments: $@"
exec /app/sync-image "$@"
