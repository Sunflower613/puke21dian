#!/bin/bash

# 21点游戏服务器启动脚本

# 默认端口
PORT=${PORT:-8080}

# 启动服务器
PORT=$PORT ./blackjack-linux-amd64
