#!/usr/bin/env bash
# 评测镜像构建脚本：bash build_benzhi_docker.sh <镜像名> <平台>
# 例：bash build_benzhi_docker.sh my-project linux/amd64
set -euo pipefail

IMAGE_NAME="${1:-my-project}"
PLATFORM="${2:-linux/amd64}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

docker buildx build --platform "$PLATFORM" -f "$SCRIPT_DIR/benzhi.Dockerfile" -t "$IMAGE_NAME" "$SCRIPT_DIR"
