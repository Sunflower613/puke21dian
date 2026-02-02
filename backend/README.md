# 21点游戏服务器 - Go后端

## 项目概述

这是一个完整的多人在线21点（Blackjack）游戏服务器，使用Go语言开发，支持WebSocket实时通信。

## 功能特性

### 游戏功能
- ✅ 多人在线游戏（最多6人/房间）
- ✅ 实时WebSocket通信
- ✅ 21点游戏逻辑（发牌、计算分数、判断胜负）
- ✅ 房间管理（创建/加入/退出）
- ✅ 玩家管理（昵称、状态追踪）
- ✅ 实时聊天功能
- ✅ 游戏结果统计

### 技术特性
- 🚀 高性能Go后端
- 📡 WebSocket实时通信
- 🎴 完整的扑克牌逻辑
- 🔒 线程安全的并发处理
- 📦 静态编译，无依赖

## 项目结构

```
backend/
├── main.go          # 主程序入口和HTTP路由
├── card.go          # 扑克牌和牌组逻辑
├── player.go        # 玩家管理
├── room.go          # 房间管理
├── websocket.go     # WebSocket连接和消息处理
├── go.mod           # Go模块依赖
├── build.sh         # Linux构建脚本
├── build.bat        # Windows构建脚本
└── README.md        # 本文档

../
├── 21dian.html      # 首页
├── 21game.html      # 游戏页面
├── 21game.js        # 游戏客户端JavaScript
├── pukeshow.html    # 扑克展示
├── common.css       # 通用样式
└── puke_sprites.css # 扑克精灵图样式
```

## 快速开始

### 开发环境要求

- Go 1.21 或更高版本
- 现代浏览器（Chrome, Firefox, Safari, Edge）

### 本地开发

1. 安装依赖：
```bash
cd backend
go mod download
```

2. 启动开发服务器：
```bash
go run .
```

3. 访问游戏：
```
http://localhost:8080/21dian.html
```

### 编译生产版本

#### Windows
```batch
cd backend
build.bat
```

#### Linux/Mac
```bash
cd backend
chmod +x build.sh
./build.sh
```

### 部署到Linux服务器

1. 上传编译后的文件到服务器：
```bash
scp build/blackjack-linux-amd64 user@server:/opt/blackjack/
scp -r ../ user@server:/opt/blackjack/public/
```

2. 启动服务：
```bash
cd /opt/blackjack
./blackjack-linux-amd64
```

3. 配置防火墙（如果需要）：
```bash
sudo ufw allow 8080/tcp
```

## API文档

### HTTP API

#### 创建房间
```
POST /api/room/create
Response:
{
  "roomId": "12345"
}
```

#### 获取房间信息
```
GET /api/room/{roomId}
Response:
{
  "roomId": "12345",
  "playerCount": 3,
  "status": 1
}
```

#### 离开房间
```
DELETE /api/room/{roomId}?playerId={playerId}
Response:
{
  "message": "已离开房间"
}
```

### WebSocket API

连接地址：`ws://server:port/ws`

#### 消息类型

**connect** - 连接服务器
```json
{
  "type": "connect",
  "data": {
    "playerId": "player123",
    "nickname": "小明"
  }
}
```

**join** - 加入房间
```json
{
  "type": "join",
  "data": {
    "roomId": "12345",
    "playerId": "player123",
    "nickname": "小明"
  }
}
```

**start** - 开始游戏
```json
{
  "type": "start",
  "data": {
    "roomId": "12345",
    "playerId": "player123"
  }
}
```

**hit** - 要牌
```json
{
  "type": "hit",
  "data": {
    "roomId": "12345",
    "playerId": "player123"
  }
}
```

**stand** - 停牌
```json
{
  "type": "stand",
  "data": {
    "roomId": "12345",
    "playerId": "player123"
  }
}
```

**chat** - 发送聊天消息
```json
{
  "type": "chat",
  "data": {
    "roomId": "12345",
    "playerId": "player123",
    "message": "你好！"
  }
}
```

#### 服务器推送消息

**players** - 玩家列表更新
```json
{
  "type": "players",
  "data": {
    "players": [
      {
        "id": "player1",
        "nickname": "小明",
        "cards": ["pk-spadeA", "pk-heart3"],
        "cardCount": 2,
        "handValue": 14,
        "status": "操作中",
        "statusColor": "yellow"
      }
    ]
  }
}
```

**update** - 玩家状态更新
```json
{
  "type": "update",
  "data": {
    "id": "player1",
    "nickname": "小明",
    "cards": ["pk-spadeA", "pk-heart3", "pk-club4"],
    "cardCount": 3,
    "handValue": 18,
    "status": "已停牌",
    "statusColor": "green"
  }
}
```

**chat** - 聊天消息
```json
{
  "type": "chat",
  "data": {
    "playerId": "player1",
    "nickname": "小明",
    "message": "你好！",
    "time": "now"
  }
}
```

**gameEnd** - 游戏结束
```json
{
  "type": "gameEnd",
  "data": {
    "roomId": "12345",
    "results": [
      {
        "playerId": "player1",
        "nickname": "小明",
        "score": 20,
        "status": "已停牌",
        "isWinner": true
      }
    ]
  }
}
```

## 配置说明

### 环境变量

- `PORT` - 服务器端口（默认：8080）

### 使用示例

```bash
# 使用自定义端口启动
PORT=3000 ./blackjack-linux-amd64
```

## 游戏规则

1. **目标**：手牌点数尽可能接近21点，但不能超过
2. **牌面值**：
   - A：1点或11点（自动计算最优值）
   - 2-10：按牌面值
   - J、Q、K：10点
3. **爆牌**：超过21点即为爆牌，直接判负
4. **胜负判定**：
   - 未爆牌的玩家中点数最大者获胜
   - 21点（Blackjack）特殊奖励
   - 所有玩家都爆牌则无赢家

## 性能优化

- 使用WebSocket长连接，减少HTTP开销
- 并发安全的房间管理
- 高效的消息广播机制
- 内存复用，减少GC压力

## 安全建议

1. **生产环境部署**
   - 启用HTTPS（WSS）
   - 配置CORS策略
   - 限制房间数量和玩家数量
   - 添加认证机制

2. **反向代理配置（Nginx示例）**
```nginx
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

## 故障排查

### 问题：无法连接服务器
- 检查服务器是否运行
- 检查防火墙设置
- 检查WebSocket连接地址

### 问题：游戏无法开始
- 确认已加入房间
- 检查服务器日志
- 查看浏览器控制台错误

### 问题：编译失败
- 确认Go版本 >= 1.21
- 运行 `go mod tidy` 清理依赖
- 检查网络连接

## 贡献指南

欢迎提交问题和改进建议！

## 许可证

MIT License

## 联系方式

如有问题，请提交Issue。
