package main

import (
	"math/rand"
	"time"
)

// Suit 扑克花色
type Suit int

const (
	Club Suit = iota // 梅花
	Diamond          // 方块
	Heart            // 红桃
	Spade            // 黑桃
)

// Rank 牌面点数
type Rank int

const (
	Ace Rank = 1 + iota
	Two
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
)

// Card 扑克牌
type Card struct {
	Suit Suit
	Rank Rank
}

// Value 获取牌的数值（21点规则）
func (c *Card) Value() int {
	switch c.Rank {
	case Jack, Queen, King:
		return 10
	default:
		return int(c.Rank)
	}
}

// String 获取牌的字符串表示（用于 CSS 类名）
func (c *Card) String() string {
	suitStr := ""
	rankStr := ""

	switch c.Suit {
	case Club:
		suitStr = "club"
	case Diamond:
		suitStr = "diamond"
	case Heart:
		suitStr = "heart"
	case Spade:
		suitStr = "spade"
	}

	switch c.Rank {
	case Ace:
		rankStr = "A"
	case Jack:
		rankStr = "J"
	case Queen:
		rankStr = "Q"
	case King:
		rankStr = "K"
	default:
		rankStr = string('0' + byte(c.Rank))
	}

	return "pk-" + suitStr + rankStr
}

// Deck 一副牌（52张）
type Deck struct {
	cards []Card
}

// NewDeck 创建一副新牌并洗牌
func NewDeck() *Deck {
	d := &Deck{cards: make([]Card, 0, 52)}

	for suit := Club; suit <= Spade; suit++ {
		for rank := Ace; rank <= King; rank++ {
			d.cards = append(d.cards, Card{Suit: suit, Rank: rank})
		}
	}

	d.Shuffle()
	return d
}

// Shuffle 洗牌
func (d *Deck) Shuffle() {
	rand.Seed(time.Now().UnixNano())
	for i := len(d.cards) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	}
}

// Deal 发一张牌
func (d *Deck) Deal() Card {
	if len(d.cards) == 0 {
		return Card{}
	}
	card := d.cards[len(d.cards)-1]
	d.cards = d.cards[:len(d.cards)-1]
	return card
}

// Remaining 返回剩余牌数
func (d *Deck) Remaining() int {
	return len(d.cards)
}

// CalculateHandValue 计算手牌的总点数
func CalculateHandValue(cards []Card) int {
	total := 0
	aces := 0

	for _, card := range cards {
		total += card.Value()
		if card.Rank == Ace {
			aces++
		}
	}

	// 如果总点数超过21且有A，将A从11分变成1分
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}

	return total
}

// IsBust 检查是否爆牌（超过21点）
func IsBust(cards []Card) bool {
	return CalculateHandValue(cards) > 21
}

// IsBlackjack 检查是否为21点（首两张牌为21点）
func IsBlackjack(cards []Card) bool {
	return len(cards) == 2 && CalculateHandValue(cards) == 21
}
