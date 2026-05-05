#!/usr/bin/env bash
# 在「你自己的 Mac 终端」运行（Cursor 沙箱内无法 SSH 出站）。
# 用法示例：
#   cd /Users/jj/Documents/Playground
#   export REMOTE_HOST=198.13.35.85
#   export SSH_KEY=~/.ssh/tencent_hk_mac
#   export REMOTE_PATH=/opt/new-api/new-api          # 服务器上二进制目标路径（请按你实际部署修改）
#   export REMOTE_RESTART_CMD='docker restart new-api'   # 或: systemctl restart new-api
#   ./ops/local-tools/deploy/deploy-to-vultr.sh
#
# 若你用 docker compose 源码目录部署，可把 REMOTE_PATH 设为 /tmp/new-api-linux，
# 再在 REMOTE_RESTART_CMD 里写 docker cp + restart 或 compose build 等。

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"

REMOTE_HOST="${REMOTE_HOST:?设置 REMOTE_HOST，例如 198.13.35.85}"
REMOTE_USER="${REMOTE_USER:-root}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/tencent_hk_mac}"
REMOTE_PATH="${REMOTE_PATH:?设置 REMOTE_PATH：服务器上可执行文件路径，如 /opt/new-api/new-api}"
REMOTE_RESTART_CMD="${REMOTE_RESTART_CMD:?设置 REMOTE_RESTART_CMD，例如 docker restart new-api}"

SSH_BASE=(ssh -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -i "$SSH_KEY")
SCP_BASE=(scp -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -i "$SSH_KEY")

BIN_LOCAL="$ROOT/build/new-api-linux-amd64"
mkdir -p "$ROOT/build"

echo "==> 编译 Linux amd64 二进制..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN_LOCAL" .

TMP_REMOTE="/tmp/new-api-linux-$(date +%s)"
echo "==> 上传 -> ${REMOTE_USER}@${REMOTE_HOST}:${TMP_REMOTE}"
"${SCP_BASE[@]}" "$BIN_LOCAL" "${REMOTE_USER}@${REMOTE_HOST}:${TMP_REMOTE}"

echo "==> 安装并重启（远程）..."
"${SSH_BASE[@]}" "${REMOTE_USER}@${REMOTE_HOST}" bash -s <<EOF
set -euo pipefail
install -m 0755 "$TMP_REMOTE" "$REMOTE_PATH"
rm -f "$TMP_REMOTE"
$REMOTE_RESTART_CMD
echo "部署完成: $REMOTE_PATH"
EOF

echo "==> 本地部署脚本结束。"
