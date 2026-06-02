package main

import "time"

// PlayerStatus 玩家状态
type PlayerStatus int

const (
	StatusWaiting PlayerStatus = iota // 等待中
	StatusActing                      // 操作中
	StatusStood                       // 已停牌
	StatusBust                        // 已爆牌
)

// Player 玩家
type Player struct {
	ID          string         `json:"id"`
	Nickname    string         `json:"nickname"`
	AccountName string         `json:"accountName"`
	Score       int            `json:"score"`
	Cards       []Card         `json:"cards"`
	Status      PlayerStatus   `json:"status"`
	HandValue   int            `json:"handValue"`
	RoomID      string         `json:"roomId"`
	IsBot       bool           `json:"isBot"`
	Conn        *WebSocketConn `json:"-"` // WebSocket连接
	LastActive  time.Time      `json:"lastActive"`
}

// NewPlayer 创建新玩家
func NewPlayer(id, nickname string) *Player {
	return &Player{
		ID:          id,
		Nickname:    nickname,
		AccountName: "",
		Score:       0,
		Cards:       make([]Card, 0),
		Status:      StatusWaiting,
		HandValue:   0,
		LastActive:  time.Now(),
	}
}

// Reset 重置玩家状态（新一局）
func (p *Player) Reset() {
	p.Cards = make([]Card, 0)
	p.Status = StatusWaiting
	p.HandValue = 0
}

// AddCard 添加一张牌
func (p *Player) AddCard(card Card) {
	p.Cards = append(p.Cards, card)
	p.HandValue = CalculateHandValue(p.Cards)

	// 检查是否爆牌
	if p.HandValue > 21 {
		p.Status = StatusBust
	}
}

// Stand 停牌
func (p *Player) Stand() {
	p.Status = StatusStood
}

// GetStatusString 获取状态字符串
func (p *Player) GetStatusString() string {
	switch p.Status {
	case StatusWaiting:
		return "等待中"
	case StatusActing:
		return "操作中"
	case StatusStood:
		return "已停牌"
	case StatusBust:
		return "已爆牌"
	default:
		return "未知"
	}
}

// GetStatusColor 获取状态颜色
func (p *Player) GetStatusColor() string {
	switch p.Status {
	case StatusWaiting:
		return "gray"
	case StatusActing:
		return "yellow"
	case StatusStood:
		return "green"
	case StatusBust:
		return "red"
	default:
		return "gray"
	}
}

// CanAct 检查是否可以操作（哪怕21点也需手动停牌）
func (p *Player) CanAct() bool {
	return p.Status == StatusActing && p.HandValue <= 21
}

// ToMap 转换为Map（用于JSON序列化）
// hideCards: 隐藏手牌（只露第一张）
// maskStatus: 隐藏真实状态（爆牌显示为停牌，隐藏分数）
func (p *Player) ToMap(hideCards bool, maskStatus bool) map[string]interface{} {
	cards := make([]string, 0, len(p.Cards))
	for _, card := range p.Cards {
		cards = append(cards, card.String())
	}

	// 如果需要隐藏牌，只显示第一张
	if hideCards && len(cards) > 1 {
		cards = cards[:1]
		for i := 1; i < len(p.Cards); i++ {
			cards = append(cards, "pk-hide")
		}
	}

	status := p.GetStatusString()
	statusColor := p.GetStatusColor()
	handValue := p.HandValue

	// 对其他玩家隐藏真实状态：爆牌伪装为停牌，停牌/爆牌隐藏分数
	if maskStatus {
		if p.Status == StatusBust {
			status = "已停牌"
			statusColor = "green"
		}
		if p.Status == StatusStood || p.Status == StatusBust {
			handValue = 0
		}
	}

	return map[string]interface{}{
		"id":          p.ID,
		"nickname":    p.Nickname,
		"accountName": p.AccountName,
		"score":       accountStore.GetScore(p.AccountName),
		"cards":       cards,
		"cardCount":   len(p.Cards),
		"handValue":   handValue,
		"status":      status,
		"statusColor": statusColor,
		"isBot":       p.IsBot,
	}
}
