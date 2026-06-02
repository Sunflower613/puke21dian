#!/bin/bash
# 远程服务器启动脚本：自动选择可用端口并启动 21 点游戏服务

set -euo pipefail

APP_DIR="/www/wwwroot/secondhand.com/games"
BIN="$APP_DIR/backend/build/blackjack-linux-amd64"
PID_FILE="$APP_DIR/backend/build/server.pid"
LOG_FILE="$APP_DIR/backend/build/server.log"

find_free_port() {
  local port
  for port in 8080 8081 8082 8083 8090 8889 9000; do
    if ! ss -tln 2>/dev/null | awk '{print $4}' | grep -q ":${port}$"; then
      echo "$port"
      return 0
    fi
  done
  return 1
}

if [ ! -x "$BIN" ]; then
  echo "错误: 找不到可执行文件 $BIN"
  exit 1
fi

if [ -f "$PID_FILE" ]; then
  OLD_PID=$(cat "$PID_FILE" 2>/dev/null || true)
  if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
    echo "服务已在运行 (PID=$OLD_PID)"
    exit 0
  fi
fi

PORT="${PORT:-$(find_free_port || true)}"
if [ -z "$PORT" ]; then
  echo "错误: 未找到可用端口"
  exit 1
fi

mkdir -p "$(dirname "$LOG_FILE")"
cd "$APP_DIR/backend/build"
nohup env PORT="$PORT" ./blackjack-linux-amd64 >>"$LOG_FILE" 2>&1 &
echo $! > "$PID_FILE"
sleep 1

if kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
  echo "启动成功"
  echo "PORT=$PORT"
  echo "PID=$(cat "$PID_FILE")"
  echo "URL=http://127.0.0.1:$PORT/"
  curl -s -o /dev/null -w "health_http=%{http_code}\n" "http://127.0.0.1:$PORT/" || true
  curl -s -o /dev/null -w "health_api=%{http_code}\n" -X POST "http://127.0.0.1:$PORT/api/room/create" || true
else
  echo "启动失败，最近日志:"
  tail -20 "$LOG_FILE" 2>/dev/null || true
  exit 1
fi
