#!/usr/bin/env bash
# 构建并部署到服务器
# 用法: ./build.sh [deploy]
#   ./build.sh        — 仅构建
#   ./build.sh deploy — 构建 + 部署到服务器
set -euo pipefail

REMOTE="duoglas@dm.kuoo.uk"
SSH_PORT=2255
IMAGE="cpa-hardened:latest"
TMPFILE="/tmp/cpa-hardened.tar.gz"

cd "$(dirname "$0")"

echo "==> 构建 Docker 镜像 (linux/amd64)..."
docker buildx build \
    --build-arg GOPROXY=https://goproxy.cn,direct \
    --build-arg TARGETARCH=amd64 \
    --build-arg VERSION=hardened-$(git rev-parse --short HEAD) \
    --build-arg COMMIT=$(git rev-parse --short HEAD) \
    --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
    -t ${IMAGE} --load .

echo "==> 构建成功: ${IMAGE}"
docker inspect ${IMAGE} --format 'Platform: {{.Os}}/{{.Architecture}}'

if [ "${1:-}" = "deploy" ]; then
    echo "==> 导出镜像..."
    docker save ${IMAGE} | gzip > ${TMPFILE}
    echo "==> 上传到服务器..."
    scp -P ${SSH_PORT} ${TMPFILE} ${REMOTE}:/tmp/
    echo "==> 加载镜像并重启服务..."
    ssh -p ${SSH_PORT} ${REMOTE} "sudo docker load < ${TMPFILE} && cd /opt/cliproxyapi && sudo docker compose down && sudo docker compose up -d && rm ${TMPFILE}"
    rm -f ${TMPFILE}
    echo "==> 验证..."
    sleep 3
    ssh -p ${SSH_PORT} ${REMOTE} "sudo docker ps --format '{{.Names}}\t{{.Status}}' | grep cli-proxy-api"
    echo "==> 部署完成!"
fi
