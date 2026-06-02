package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Account struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash"`
	Score        int       `json:"score"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PublicAccount struct {
	Username string `json:"username"`
	Score    int    `json:"score"`
}

type AccountStore struct {
	path     string
	accounts map[string]*Account
	mu       sync.RWMutex
}

func NewAccountStore(path string) *AccountStore {
	store := &AccountStore{
		path:     path,
		accounts: make(map[string]*Account),
	}
	store.load()
	return store
}

func (s *AccountStore) Login(username, password string) (*PublicAccount, bool, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, false, fmt.Errorf("名称和密码不能为空")
	}
	if len(username) > 16 {
		return nil, false, fmt.Errorf("名称最多 16 个字符")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	hash := hashPassword(username, password)
	account, exists := s.accounts[username]
	if exists {
		if account.PasswordHash != hash {
			return nil, false, fmt.Errorf("密码错误")
		}
		account.UpdatedAt = now
		if err := s.saveLocked(); err != nil {
			return nil, false, err
		}
		return &PublicAccount{Username: account.Username, Score: account.Score}, false, nil
	}

	account = &Account{
		Username:     username,
		PasswordHash: hash,
		Score:        0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.accounts[username] = account
	if err := s.saveLocked(); err != nil {
		return nil, false, err
	}

	return &PublicAccount{Username: account.Username, Score: account.Score}, true, nil
}

func (s *AccountStore) AddScore(username string, delta int) int {
	username = strings.TrimSpace(username)
	if username == "" || delta == 0 {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	account, exists := s.accounts[username]
	if !exists {
		return 0
	}

	account.Score += delta
	account.UpdatedAt = time.Now()
	if err := s.saveLocked(); err != nil {
		fmt.Printf("保存积分失败: %v\n", err)
	}
	return account.Score
}

func (s *AccountStore) GetScore(username string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if account, exists := s.accounts[username]; exists {
		return account.Score
	}
	return 0
}

func (s *AccountStore) Leaderboard(limit int) []PublicAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]PublicAccount, 0, len(s.accounts))
	for _, account := range s.accounts {
		list = append(list, PublicAccount{
			Username: account.Username,
			Score:    account.Score,
		})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Score == list[j].Score {
			return list[i].Username < list[j].Username
		}
		return list[i].Score > list[j].Score
	})

	if limit > 0 && len(list) > limit {
		return list[:limit]
	}
	return list
}

func (s *AccountStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &s.accounts); err != nil {
		fmt.Printf("读取账号数据失败: %v\n", err)
		s.accounts = make(map[string]*Account)
	}
}

func (s *AccountStore) saveLocked() error {
	data, err := json.MarshalIndent(s.accounts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func hashPassword(username, password string) string {
	sum := sha256.Sum256([]byte(username + ":" + password))
	return hex.EncodeToString(sum[:])
}
