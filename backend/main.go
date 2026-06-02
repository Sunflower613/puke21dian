package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var roomManager *RoomManager
var accountStore *AccountStore

func init() {
	rand.Seed(time.Now().UnixNano())
	roomManager = NewRoomManager()
	accountStore = NewAccountStore("accounts.json")
}

func main() {
	// 自动检测静态文件目录（优先根据可执行文件相对路径向上查找，避免 CWD 目录不正确导致的文件错乱）
	staticDir := "../../" // 默认退路
	if cwd, err := os.Getwd(); err == nil {
		candidates := []string{
			cwd,
			filepath.Join(cwd, ".."),
			filepath.Join(cwd, "../.."),
		}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(c, "index.html")); err == nil {
				staticDir = c
				break
			}
		}
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates := []string{
			exeDir,
			filepath.Join(exeDir, ".."),
			filepath.Join(exeDir, "../.."),
		}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(c, "index.html")); err == nil {
				staticDir = c
				break
			}
		}
	}

	// 转换为绝对路径，确保静态资源加载稳定
	if absDir, err := filepath.Abs(staticDir); err == nil {
		staticDir = absDir
	}

	fs := http.FileServer(http.Dir(staticDir))

	// 创建房间API
	http.HandleFunc("/api/auth/login", handleLogin)
	http.HandleFunc("/api/leaderboard", handleLeaderboard)
	http.HandleFunc("/api/room/create", handleCreateRoom)
	http.HandleFunc("/api/room/", handleRoomAPI)

	// WebSocket处理
	http.HandleFunc("/ws", roomManager.HandleWebSocket)

	// 静态文件服务（处理SPA路由）
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 如果是WebSocket连接，交给WS处理
		if r.Header.Get("Upgrade") == "websocket" {
			roomManager.HandleWebSocket(w, r)
			return
		}

		// 禁用静态文件浏览器缓存，防止开发调试时静态资源被强缓存
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		path := r.URL.Path
		if path == "/room" || path == "/room/" {
			// Ensure staticDir has correct slash prefix/suffix
			filePath := staticDir + "/room.html"
			if _, err := os.Stat(filePath); err == nil {
				http.ServeFile(w, r, filePath)
				return
			}
		}

		// 其他请求提供静态文件
		fs.ServeHTTP(w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🎰 21点游戏服务器启动\n")
	fmt.Printf("🌐 HTTP服务地址: http://localhost:%s/\n", port)
	fmt.Printf("🔌 WebSocket地址: ws://localhost:%s/ws\n", port)
	fmt.Printf("📁 静态文件目录: %s\n\n", staticDir)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

// handleLogin 处理简单名称密码登录；新名称会自动注册
func handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的数据格式"})
		return
	}

	account, created, err := accountStore.Login(data.Username, data.Password)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"username": account.Username,
		"score":    account.Score,
		"created":  created,
	})
}

// handleLeaderboard 返回积分排行榜
func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"players": accountStore.Leaderboard(20),
	})
}

// handleCreateRoom 处理创建房间
func handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	room := roomManager.CreateRoom()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"roomId": room.ID,
	})
}

// handleRoomAPI 处理房间API
func handleRoomAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 提取房间ID
	roomID := r.URL.Path[len("/api/room/"):]
	if roomID == "" {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "房间ID不能为空",
		})
		return
	}

	room := roomManager.GetRoom(roomID)
	if room == nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "房间不存在",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取房间信息
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roomId":      room.ID,
			"playerCount": room.PlayerCount(),
			"status":      room.Status,
		})

	case http.MethodDelete:
		// 离开房间 (支持 Auth Header: X-Player-Id)
		playerID := r.Header.Get("X-Player-Id")
		if playerID == "" {
			playerID = r.URL.Query().Get("playerId")
		}

		if playerID == "" {
			json.NewEncoder(w).Encode(map[string]string{
				"error": "玩家ID不能为空",
			})
			return
		}

		roomManager.LeaveRoom(roomID, playerID)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "已离开房间",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
