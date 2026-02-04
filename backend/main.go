package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var roomManager *RoomManager

func init() {
	rand.Seed(time.Now().UnixNano())
	roomManager = NewRoomManager()
}

func main() {
	// 设置静态文件服务
	// 自动检测静态文件目录（支持从backend或backend/build运行）
	staticDir := "../"
	if _, err := os.Stat("../../21dian.html"); err == nil {
		staticDir = "../../"
	}
	fs := http.FileServer(http.Dir(staticDir))

	// 创建房间API
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

		// 其他请求提供静态文件
		fs.ServeHTTP(w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🎰 21点游戏服务器启动\n")
	fmt.Printf("🌐 HTTP服务地址: http://localhost:%s/21dian.html\n", port)
	fmt.Printf("🔌 WebSocket地址: ws://localhost:%s/ws\n", port)
	fmt.Printf("📁 静态文件目录: %s\n\n", staticDir)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
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
		// 离开房间
		playerID := r.URL.Query().Get("playerId")
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
