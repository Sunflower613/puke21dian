#!/bin/bash
# 本地执行：打包核心文件并部署到远程服务器

set -euo pipefail

REMOTE_HOST="${REMOTE_HOST:-111.229.107.228}"
REMOTE_PORT="${REMOTE_PORT:-22}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_DIR="/www/wwwroot/secondhand.com/games"
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SSH_OPTS=(-o StrictHostKeyChecking=accept-new -p "$REMOTE_PORT")

echo "==> 1. 编译 Linux amd64 产物"
cd "$PROJECT_ROOT/backend"
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/blackjack-linux-amd64 .

echo "==> 2. 检查远程端口占用"
ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$REMOTE_HOST" '
for p in 8080 8081 8082 8083 8090 8888 8889 9000; do
  if ss -tln 2>/dev/null | awk "{print \$4}" | grep -q ":${p}$"; then
    echo "占用: $p"
  else
    echo "空闲: $p"
  fi
done
'

echo "==> 3. 创建远程目录"
ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$REMOTE_HOST" "mkdir -p $REMOTE_DIR/backend/build $REMOTE_DIR/backend/deploy"

echo "==> 4. 同步核心文件"
rsync -avz -e "ssh ${SSH_OPTS[*]}" \
  "$PROJECT_ROOT/index.html" \
  "$PROJECT_ROOT/21game.html" \
  "$PROJECT_ROOT/21game.js" \
  "$PROJECT_ROOT/21dian.html" \
  "$PROJECT_ROOT/common.css" \
  "$PROJECT_ROOT/puke_sprites.css" \
  "$PROJECT_ROOT/pukeshow.html" \
  "$PROJECT_ROOT/backend/build/blackjack-linux-amd64" \
  "$PROJECT_ROOT/backend/deploy/remote-start.sh" \
  "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/"

ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$REMOTE_HOST" "
  mv -f $REMOTE_DIR/blackjack-linux-amd64 $REMOTE_DIR/backend/build/
  mv -f $REMOTE_DIR/remote-start.sh $REMOTE_DIR/backend/deploy/
  chmod +x $REMOTE_DIR/backend/build/blackjack-linux-amd64 $REMOTE_DIR/backend/deploy/remote-start.sh
"

echo "==> 5. 远程启动并测试"
ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$REMOTE_HOST" "bash $REMOTE_DIR/backend/deploy/remote-start.sh"

echo "==> 部署完成"
