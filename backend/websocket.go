package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

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
	TypeConnect  MessageType = "connect"
	TypeJoin     MessageType = "join"
	TypeLeave    MessageType = "leave"
	TypeStart    MessageType = "start"
	TypeHit      MessageType = "hit"
	TypeStand    MessageType = "stand"
	TypeChat     MessageType = "chat"
	TypeUpdate   MessageType = "update"
	TypeError    MessageType = "error"
	TypeRoomInfo MessageType = "roomInfo"
	TypePlayers  MessageType = "players"
	TypeGameEnd  MessageType = "gameEnd"
	TypeAddBot   MessageType = "addBot"
)

// Message WebSocket消息
type Message struct {
	Type  MessageType     `json:"type"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
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
		wsc.close()
	}
}

// Close 关闭连接
func (wsc *WebSocketConn) Close() {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	wsc.close()
}

// close 私有关闭连接（无锁版本，避免死锁）
func (wsc *WebSocketConn) close() {
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
func (rm *RoomManager) JoinRoom(roomID, playerID, nickname, accountName string) (*Room, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("房间不存在")
	}

	player := NewPlayer(playerID, nickname)
	player.AccountName = accountName
	player.Score = accountStore.GetScore(accountName)
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

	// 连接断开后，扫描并清理对应的玩家
	rm.mu.Lock()
	var disconnectedPlayer *Player
	for _, p := range rm.players {
		if p.Conn == wsConn {
			disconnectedPlayer = p
			break
		}
	}
	rm.mu.Unlock()

	if disconnectedPlayer != nil {
		rm.handlePlayerDisconnect(disconnectedPlayer)
	}
}

// handlePlayerDisconnect 处理玩家断开连接（延迟清理以支持重连）
func (rm *RoomManager) handlePlayerDisconnect(player *Player) {
	roomID := player.RoomID
	playerID := player.ID

	go func() {
		time.Sleep(3 * time.Second) // 延迟 3 秒以允许刷新重连
		rm.mu.Lock()
		defer rm.mu.Unlock()

		p, exists := rm.players[playerID]
		if !exists {
			return
		}
		// 如果 p.Conn 为空，或者连接已被标记为关闭，说明在此期间没有连回
		if p.Conn == nil || p.Conn.IsClosed() {
			room, roomExists := rm.rooms[roomID]
			if roomExists {
				room.RemovePlayer(playerID)
				delete(rm.players, playerID)

				if room.PlayerCount() > 0 {
					// 重新发送玩家列表给房间内的其他人
					rm.sendPlayersList(room)
				} else {
					delete(rm.rooms, roomID)
				}
			}
		}
	}()
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
	case TypeAddBot:
		rm.handleAddBot(wsConn, msg)
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
		PlayerID    string `json:"playerId"`
		Nickname    string `json:"nickname"`
		AccountName string `json:"accountName"`
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
		player.AccountName = data.AccountName
		player.Score = accountStore.GetScore(data.AccountName)
		rm.mu.Lock()
		rm.players[data.PlayerID] = player
		rm.mu.Unlock()
	} else {
		player.Nickname = data.Nickname
		player.AccountName = data.AccountName
		player.Score = accountStore.GetScore(data.AccountName)
		player.Conn = wsConn
	}

	wsConn.Send(Message{
		Type: TypeConnect,
		Data: toJSON(map[string]string{
			"playerId":    player.ID,
			"nickname":    player.Nickname,
			"accountName": player.AccountName,
		}),
	})
}

// handleJoin 处理加入房间
func (rm *RoomManager) handleJoin(wsConn *WebSocketConn, msg Message) {
	var data struct {
		RoomID      string `json:"roomId"`
		PlayerID    string `json:"playerId"`
		Nickname    string `json:"nickname"`
		AccountName string `json:"accountName"`
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
		existingPlayer.AccountName = data.AccountName
		existingPlayer.Score = accountStore.GetScore(data.AccountName)

		// 发送房间信息
		wsConn.Send(Message{
			Type: TypeRoomInfo,
			Data: toJSON(map[string]interface{}{
				"roomId":     room.ID,
				"status":     room.Status,
				"isExisting": true,
			}),
		})

		// 广播玩家列表更新
		rm.sendPlayersList(room)
		return
	}

	// 玩家不存在，尝试加入房间
	room, err := rm.JoinRoom(data.RoomID, data.PlayerID, data.Nickname, data.AccountName)
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
	rm.sendPlayersList(room)
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

	rm.sendPlayersList(room)

	// 如果有人机玩家，启动人机自动操作
	rm.botPlay(room)
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

	// 检查游戏是否结束
	if room.CheckGameEnd() {
		rm.handleGameEnd(room)
	} else {
		rm.sendPlayersList(room)
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

	// 检查游戏是否结束
	if room.CheckGameEnd() {
		rm.handleGameEnd(room)
	} else {
		rm.sendPlayersList(room)
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

// handleAddBot 处理添加人机
func (rm *RoomManager) handleAddBot(wsConn *WebSocketConn, msg Message) {
	var data struct {
		RoomID   string `json:"roomId"`
		PlayerID string `json:"playerId"`
	}

	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	room := rm.GetRoom(data.RoomID)
	if room == nil {
		wsConn.Send(Message{Type: TypeError, Error: "房间不存在"})
		return
	}

	if room.Status != GameWaiting {
		wsConn.Send(Message{Type: TypeError, Error: "游戏已开始，无法添加人机"})
		return
	}

	// 统计已有人机数量
	room.Lock.RLock()
	botCount := 0
	for _, p := range room.Players {
		if p.IsBot {
			botCount++
		}
	}
	room.Lock.RUnlock()

	botNum := botCount + 1
	botID := fmt.Sprintf("bot_%s_%d_%d", data.RoomID, botNum, time.Now().UnixNano())
	botName := fmt.Sprintf("机器人%d", botNum)

	bot := NewPlayer(botID, botName)
	bot.IsBot = true

	if !room.AddPlayer(bot) {
		wsConn.Send(Message{Type: TypeError, Error: "无法添加人机（房间已满）"})
		return
	}

	rm.mu.Lock()
	rm.players[botID] = bot
	rm.mu.Unlock()

	// 广播更新的玩家列表
	rm.sendPlayersList(room)
}

// botPlay 人机自动操作（经典策略：<17要牌，>=17停牌）
func (rm *RoomManager) botPlay(room *Room) {
	// 检查是否有人机
	room.Lock.RLock()
	hasBot := false
	for _, p := range room.Players {
		if p.IsBot {
			hasBot = true
			break
		}
	}
	room.Lock.RUnlock()

	if !hasBot {
		return
	}

	go func() {
		time.Sleep(1500 * time.Millisecond) // 等待前端 UI 更新

		for {
			// 查找还能操作的人机
			room.Lock.RLock()
			if room.Status != GamePlaying {
				room.Lock.RUnlock()
				return
			}
			var botID string
			var shouldHit bool
			for _, id := range room.PlayerIDs {
				p := room.Players[id]
				if p != nil && p.IsBot && p.Status == StatusActing {
					botID = p.ID
					shouldHit = p.HandValue < 17
					break
				}
			}
			room.Lock.RUnlock()

			if botID == "" {
				return // 所有人机操作完毕
			}

			// 执行人机决策
			if shouldHit {
				room.PlayerHit(botID)
			} else {
				room.PlayerStand(botID)
			}

			// 检查游戏是否结束
			if room.CheckGameEnd() {
				rm.handleGameEnd(room)
				return
			}

			// 广播更新
			rm.sendPlayersList(room)

			time.Sleep(1000 * time.Millisecond) // 模拟思考时间
		}
	}()
}

// handleGameEnd 处理游戏结束
func (rm *RoomManager) handleGameEnd(room *Room) {
	// 计算结果
	results := make([]map[string]interface{}, 0)
	maxScore := 0

	// 找出最高分（不超过21）
	for _, player := range room.Players {
		if player.Status != StatusBust && player.HandValue > maxScore {
			maxScore = player.HandValue
		}
	}

	// 生成结果：所有达到最高分的玩家都是赢家（平局）
	for _, player := range room.Players {
		isWinner := player.Status != StatusBust && player.HandValue == maxScore && maxScore > 0
		totalScore := accountStore.GetScore(player.AccountName)
		if isWinner && !player.IsBot && player.AccountName != "" {
			totalScore = accountStore.AddScore(player.AccountName, 1)
			player.Score = totalScore
		}
		result := map[string]interface{}{
			"playerId":    player.ID,
			"nickname":    player.Nickname,
			"accountName": player.AccountName,
			"score":       player.HandValue,
			"totalScore":  totalScore,
			"status":      player.GetStatusString(),
			"isWinner":    isWinner,
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

	// 广播公开所有牌的玩家列表
	rm.sendPlayersList(room)
}

// sendPlayersList 给房间内的所有玩家发送量身定制的玩家列表（隐藏其他玩家的手牌）
func (rm *RoomManager) sendPlayersList(room *Room) {
	room.Lock.RLock()
	defer room.Lock.RUnlock()

	for _, player := range room.Players {
		if player.Conn != nil {
			playersData := make([]map[string]interface{}, 0)
			for _, id := range room.PlayerIDs {
				if p, exists := room.Players[id]; exists {
					isOtherDuringGame := (p.ID != player.ID) && (room.Status != GameEnded)
					// 仅在不是当前接收者且游戏未结束时隐藏卡牌和真实状态
					hideCards := isOtherDuringGame
					maskStatus := isOtherDuringGame
					playersData = append(playersData, p.ToMap(hideCards, maskStatus))
				}
			}

			player.Conn.Send(Message{
				Type: TypePlayers,
				Data: toJSON(map[string]interface{}{
					"players": playersData,
				}),
			})
		}
	}
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
