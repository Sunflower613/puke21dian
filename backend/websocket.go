package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境需要限制）
	},
}

// MessageType 消息类型
type MessageType string

const (
	TypeConnect    MessageType = "connect"
	TypeJoin       MessageType = "join"
	TypeLeave      MessageType = "leave"
	TypeStart      MessageType = "start"
	TypeHit        MessageType = "hit"
	TypeStand      MessageType = "stand"
	TypeChat       MessageType = "chat"
	TypeUpdate     MessageType = "update"
	TypeError      MessageType = "error"
	TypeRoomInfo   MessageType = "roomInfo"
	TypePlayers    MessageType = "players"
	TypeGameEnd    MessageType = "gameEnd"
)

// Message WebSocket消息
type Message struct {
	Type    MessageType      `json:"type"`
	Data    json.RawMessage   `json:"data,omitempty"`
	Error   string           `json:"error,omitempty"`
}

// WebSocketConn WebSocket连接
type WebSocketConn struct {
	conn   *websocket.Conn
	send   chan Message
	mu     sync.Mutex
	closed bool
}

// NewWebSocketConn 创建新连接
func NewWebSocketConn(conn *websocket.Conn) *WebSocketConn {
	return &WebSocketConn{
		conn:   conn,
		send:   make(chan Message, 256),
		closed: false,
	}
}

// Send 发送消息
func (wsc *WebSocketConn) Send(msg Message) {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	if wsc.closed {
		return
	}

	select {
	case wsc.send <- msg:
	default:
		// 发送缓冲区满，关闭连接
		wsc.Close()
	}
}

// Close 关闭连接
func (wsc *WebSocketConn) Close() {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	if wsc.closed {
		return
	}

	wsc.closed = true
	close(wsc.send)
	wsc.conn.Close()
}

// IsClosed 检查连接是否已关闭
func (wsc *WebSocketConn) IsClosed() bool {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	return wsc.closed
}

// WritePump 写入协程
func (wsc *WebSocketConn) WritePump() {
	defer wsc.Close()

	for {
		select {
		case msg, ok := <-wsc.send:
			if !ok {
				return
			}

			if err := wsc.conn.WriteJSON(msg); err != nil {
				log.Printf("写入错误: %v", err)
				return
			}
		}
	}
}

// ReadPump 读取协程
func (wsc *WebSocketConn) ReadPump(handler func(msg Message)) {
	defer wsc.Close()

	for {
		var msg Message
		if err := wsc.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("读取错误: %v", err)
			}
			return
		}

		handler(msg)
	}
}

// RoomManager 房间管理器
type RoomManager struct {
	rooms   map[string]*Room
	players map[string]*Player // 按玩家ID索引
	mu      sync.RWMutex
}

// NewRoomManager 创建房间管理器
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:   make(map[string]*Room),
		players: make(map[string]*Player),
	}
}

// CreateRoom 创建房间
func (rm *RoomManager) CreateRoom() *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	roomID := generateRoomID()
	room := NewRoom(roomID)
	rm.rooms[roomID] = room

	return room
}

// GetRoom 获取房间
func (rm *RoomManager) GetRoom(roomID string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.rooms[roomID]
}

// JoinRoom 加入房间
func (rm *RoomManager) JoinRoom(roomID, playerID, nickname string) (*Room, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("房间不存在")
	}

	player := NewPlayer(playerID, nickname)
	if !room.AddPlayer(player) {
		return nil, fmt.Errorf("无法加入房间（玩家已存在或房间已满）")
	}

	rm.players[playerID] = player
	return room, nil
}

// LeaveRoom 离开房间
func (rm *RoomManager) LeaveRoom(roomID, playerID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, exists := rm.rooms[roomID]
	if exists {
		room.RemovePlayer(playerID)
		delete(rm.players, playerID)

		// 如果房间空了，删除房间
		if room.PlayerCount() == 0 {
			delete(rm.rooms, roomID)
		}
	}
}

// GetPlayer 获取玩家
func (rm *RoomManager) GetPlayer(playerID string) *Player {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.players[playerID]
}

// HandleWebSocket 处理WebSocket连接
func (rm *RoomManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}

	wsConn := NewWebSocketConn(conn)

	// 启动写入协程
	go wsConn.WritePump()

	// 处理消息
	wsConn.ReadPump(func(msg Message) {
		rm.handleMessage(wsConn, msg)
	})
}

// handleMessage 处理收到的消息
func (rm *RoomManager) handleMessage(wsConn *WebSocketConn, msg Message) {
	switch msg.Type {
	case TypeConnect:
		rm.handleConnect(wsConn, msg)
	case TypeJoin:
		rm.handleJoin(wsConn, msg)
	case TypeStart:
		rm.handleStart(wsConn, msg)
	case TypeHit:
		rm.handleHit(wsConn, msg)
	case TypeStand:
		rm.handleStand(wsConn, msg)
	case TypeChat:
		rm.handleChat(wsConn, msg)
	default:
		wsConn.Send(Message{
			Type:  TypeError,
			Error: "未知消息类型",
		})
	}
}

// handleConnect 处理连接消息
func (rm *RoomManager) handleConnect(wsConn *WebSocketConn, msg Message) {
	var data struct {
		PlayerID string `json:"playerId"`
		Nickname string `json:"nickname"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		wsConn.Send(Message{
			Type:  TypeError,
			Error: "无效的数据格式",
		})
		return
	}

	player := rm.GetPlayer(data.PlayerID)
	if player == nil {
		player = NewPlayer(data.PlayerID, data.Nickname)
		rm.mu.Lock()
		rm.players[data.PlayerID] = player
		rm.mu.Unlock()
	} else {
		player.Nickname = data.Nickname
		player.Conn = wsConn
	}

	wsConn.Send(Message{
		Type: TypeConnect,
		Data: toJSON(map[string]string{
			"playerId": player.ID,
			"nickname": player.Nickname,
		}),
	})
}

// handleJoin 处理加入房间
func (rm *RoomManager) handleJoin(wsConn *WebSocketConn, msg Message) {
	var data struct {
		RoomID   string `json:"roomId"`
		PlayerID string `json:"playerId"`
		Nickname string `json:"nickname"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		wsConn.Send(Message{
			Type:  TypeError,
			Error: "无效的数据格式",
		})
		return
	}

	// 获取房间
	room := rm.GetRoom(data.RoomID)
	if room == nil {
		wsConn.Send(Message{
			Type:  TypeError,
			Error: "房间不存在",
		})
		return
	}

	// 检查玩家是否已经在房间中
	existingPlayer := room.GetPlayer(data.PlayerID)
	if existingPlayer != nil {
		// 玩家已存在，更新连接（处理刷新页面的情况）
		existingPlayer.Conn = wsConn
		existingPlayer.Nickname = data.Nickname
		
		// 发送房间信息
		wsConn.Send(Message{
			Type: TypeRoomInfo,
			Data: toJSON(map[string]interface{}{
				"roomId": room.ID,
				"status": room.Status,
			}),
		})

		// 广播玩家列表更新
		rm.broadcastPlayers(room, data.PlayerID)
		return
	}

	// 玩家不存在，尝试加入房间
	room, err := rm.JoinRoom(data.RoomID, data.PlayerID, data.Nickname)
	if err != nil {
		wsConn.Send(Message{
			Type:  TypeError,
			Error: err.Error(),
		})
		return
	}

	player := rm.GetPlayer(data.PlayerID)
	player.Conn = wsConn

	// 发送房间信息
	wsConn.Send(Message{
		Type: TypeRoomInfo,
		Data: toJSON(map[string]interface{}{
			"roomId": room.ID,
			"status": room.Status,
		}),
	})

	// 广播玩家列表更新
	rm.broadcastPlayers(room, data.PlayerID)
}

// handleStart 处理开始游戏
func (rm *RoomManager) handleStart(wsConn *WebSocketConn, msg Message) {
	var data struct {
		RoomID   string `json:"roomId"`
		PlayerID string `json:"playerId"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	room := rm.GetRoom(data.RoomID)
	if room == nil {
		return
	}

	if err := room.StartGame(); err != nil {
		wsConn.Send(Message{
			Type:  TypeError,
			Error: err.Error(),
		})
		return
	}

	// 广播游戏开始和玩家列表
	room.Broadcast(Message{
		Type: TypeStart,
		Data: toJSON(map[string]string{
			"roomId": room.ID,
		}),
	})

	rm.broadcastPlayers(room, data.PlayerID)
}

// handleHit 处理要牌
func (rm *RoomManager) handleHit(wsConn *WebSocketConn, msg Message) {
	var data struct {
		RoomID   string `json:"roomId"`
		PlayerID string `json:"playerId"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	room := rm.GetRoom(data.RoomID)
	if room == nil {
		return
	}

	if err := room.PlayerHit(data.PlayerID); err != nil {
		wsConn.Send(Message{
			Type:  TypeError,
			Error: err.Error(),
		})
		return
	}

	player := room.GetPlayer(data.PlayerID)
	room.Broadcast(Message{
		Type: TypeUpdate,
		Data: toJSON(player.ToMap(false)),
	})

	// 检查游戏是否结束
	if room.CheckGameEnd() {
		rm.handleGameEnd(room)
	} else {
		rm.broadcastPlayers(room, data.PlayerID)
	}
}

// handleStand 处理停牌
func (rm *RoomManager) handleStand(wsConn *WebSocketConn, msg Message) {
	var data struct {
		RoomID   string `json:"roomId"`
		PlayerID string `json:"playerId"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	room := rm.GetRoom(data.RoomID)
	if room == nil {
		return
	}

	if err := room.PlayerStand(data.PlayerID); err != nil {
		wsConn.Send(Message{
			Type:  TypeError,
			Error: err.Error(),
		})
		return
	}

	player := room.GetPlayer(data.PlayerID)
	room.Broadcast(Message{
		Type: TypeUpdate,
		Data: toJSON(player.ToMap(false)),
	})

	// 检查游戏是否结束
	if room.CheckGameEnd() {
		rm.handleGameEnd(room)
	} else {
		rm.broadcastPlayers(room, data.PlayerID)
	}
}

// handleChat 处理聊天
func (rm *RoomManager) handleChat(wsConn *WebSocketConn, msg Message) {
	var data struct {
		RoomID   string `json:"roomId"`
		PlayerID string `json:"playerId"`
		Message  string `json:"message"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	room := rm.GetRoom(data.RoomID)
	if room == nil {
		return
	}

	player := room.GetPlayer(data.PlayerID)
	if player == nil {
		return
	}

	chatMsg := map[string]interface{}{
		"playerId": player.ID,
		"nickname": player.Nickname,
		"message":  data.Message,
		"time":     "now",
	}

	room.Broadcast(Message{
		Type: TypeChat,
		Data: toJSON(chatMsg),
	})
}

// handleGameEnd 处理游戏结束
func (rm *RoomManager) handleGameEnd(room *Room) {
	// 计算结果
	results := make([]map[string]interface{}, 0)
	maxScore := 0
	winnerID := ""

	// 找出最高分（不超过21）
	for _, player := range room.Players {
		if player.Status != StatusBust && player.HandValue > maxScore {
			maxScore = player.HandValue
			winnerID = player.ID
		}
	}

	// 生成结果
	for _, player := range room.Players {
		result := map[string]interface{}{
			"playerId":  player.ID,
			"nickname":  player.Nickname,
			"score":     player.HandValue,
			"status":    player.GetStatusString(),
			"isWinner":  (player.ID == winnerID),
		}
		results = append(results, result)
	}

	// 广播游戏结束
	room.Broadcast(Message{
		Type: TypeGameEnd,
		Data: toJSON(map[string]interface{}{
			"roomId":  room.ID,
			"results": results,
		}),
	})
}

// broadcastPlayers 广播玩家列表
func (rm *RoomManager) broadcastPlayers(room *Room, excludeID string) {
	players := room.GetPlayersList(excludeID)

	room.Broadcast(Message{
		Type: TypePlayers,
		Data: toJSON(map[string]interface{}{
			"players": players,
		}),
	})
}

// generateRoomID 生成房间ID
func generateRoomID() string {
	return fmt.Sprintf("%d", 10000+rand.Intn(90000))
}

// toJSON 将对象转换为JSON字节数组
func toJSON(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return json.RawMessage(data)
}
