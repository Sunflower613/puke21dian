package main

import (
	"fmt"
	"sync"
	"time"
)

// GameStatus 游戏状态
type GameStatus int

const (
	GameWaiting GameStatus = iota // 等待玩家
	GamePlaying                   // 游戏中
	GameEnded                     // 游戏结束
)

// Room 房间
type Room struct {
	ID          string                   `json:"id"`
	Players     map[string]*Player       `json:"players"`
	Status      GameStatus               `json:"status"`
	Deck        *Deck                    `json:"-"`
	CurrentTurn int                      `json:"currentTurn"`
	CreatedAt   time.Time                `json:"createdAt"`
	Lock        sync.RWMutex            `json:"-"`
}

// NewRoom 创建新房间
func NewRoom(id string) *Room {
	return &Room{
		ID:        id,
		Players:   make(map[string]*Player),
		Status:    GameWaiting,
		Deck:      nil,
		CreatedAt: time.Now(),
	}
}

// AddPlayer 添加玩家到房间
func (r *Room) AddPlayer(player *Player) bool {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	if _, exists := r.Players[player.ID]; exists {
		return false
	}

	if len(r.Players) >= 6 { // 最多6个玩家
		return false
	}

	player.RoomID = r.ID
	r.Players[player.ID] = player
	return true
}

// RemovePlayer 从房间移除玩家
func (r *Room) RemovePlayer(playerID string) {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	delete(r.Players, playerID)

	// 如果房间空了，可以标记为待删除
	if len(r.Players) == 0 {
		r.Status = GameEnded
	}
}

// GetPlayer 获取玩家
func (r *Room) GetPlayer(playerID string) *Player {
	r.Lock.RLock()
	defer r.Lock.RUnlock()

	return r.Players[playerID]
}

// StartGame 开始游戏
func (r *Room) StartGame() error {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	if r.Status == GamePlaying {
		return fmt.Errorf("游戏已在进行中")
	}

	if len(r.Players) < 1 {
		return fmt.Errorf("至少需要1个玩家")
	}

	// 重置所有玩家
	for _, player := range r.Players {
		player.Reset()
		player.Status = StatusActing
	}

	// 创建新牌组并洗牌
	r.Deck = NewDeck()

	// 发初始牌（每人2张）
	for _, player := range r.Players {
		player.AddCard(r.Deck.Deal())
		player.AddCard(r.Deck.Deal())

		// 检查是否直接21点
		if IsBlackjack(player.Cards) {
			player.Status = StatusStood
		}
	}

	r.Status = GamePlaying
	r.CurrentTurn = 0

	return nil
}

// PlayerHit 玩家要牌
func (r *Room) PlayerHit(playerID string) error {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	if r.Status != GamePlaying {
		return fmt.Errorf("游戏未进行中")
	}

	player, exists := r.Players[playerID]
	if !exists {
		return fmt.Errorf("玩家不存在")
	}

	if !player.CanAct() {
		return fmt.Errorf("当前不能操作")
	}

	card := r.Deck.Deal()
	player.AddCard(card)

	return nil
}

// PlayerStand 玩家停牌
func (r *Room) PlayerStand(playerID string) error {
	r.Lock.Lock()
	defer r.Lock.Unlock()

	if r.Status != GamePlaying {
		return fmt.Errorf("游戏未进行中")
	}

	player, exists := r.Players[playerID]
	if !exists {
		return fmt.Errorf("玩家不存在")
	}

	if player.Status != StatusActing {
		return fmt.Errorf("当前不能停牌")
	}

	player.Stand()
	return nil
}

// CheckGameEnd 检查游戏是否结束
func (r *Room) CheckGameEnd() bool {
	r.Lock.RLock()
	defer r.Lock.RUnlock()

	if r.Status != GamePlaying {
		return true
	}

	// 检查所有玩家是否都结束操作（停牌或爆牌）
	allDone := true
	for _, player := range r.Players {
		if player.Status == StatusActing {
			allDone = false
			break
		}
	}

	if allDone {
		r.Status = GameEnded
		return true
	}

	return false
}

// Broadcast 向房间内所有玩家广播消息
func (r *Room) Broadcast(message Message) {
	r.Lock.RLock()
	defer r.Lock.RUnlock()

	for _, player := range r.Players {
		if player.Conn != nil {
			player.Conn.Send(message)
		}
	}
}

// GetPlayersList 获取玩家列表
func (r *Room) GetPlayersList(excludeID string) []map[string]interface{} {
	r.Lock.RLock()
	defer r.Lock.RUnlock()

	players := make([]map[string]interface{}, 0)
	for _, player := range r.Players {
		// 为其他玩家隐藏牌
		hideCards := (player.ID != excludeID)
		players = append(players, player.ToMap(hideCards))
	}

	return players
}

// PlayerCount 获取玩家数量
func (r *Room) PlayerCount() int {
	r.Lock.RLock()
	defer r.Lock.RUnlock()

	return len(r.Players)
}
