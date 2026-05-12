package service

import (
	"log"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

type PromptService struct {
	db    *sqlx.DB
	cache map[string]promptCacheItem
	mu    sync.RWMutex
}

type promptCacheItem struct {
	content       string
	systemContent string
	variables     string
	expiresAt     time.Time
}

type PromptBundle struct {
	Content       string
	SystemContent string
	Variables     string
}

func NewPromptService(db *sqlx.DB) *PromptService {
	return &PromptService{
		db:    db,
		cache: make(map[string]promptCacheItem),
	}
}

// GetPrompt 获取提示词主内容
func (s *PromptService) GetPrompt(key string) string {
	c, _ := s.GetPromptFull(key)
	return c
}

// GetPromptFull 获取提示词主内容和系统指令
func (s *PromptService) GetPromptFull(key string) (string, string) {
	bundle := s.GetPromptBundle(key)
	return bundle.Content, bundle.SystemContent
}

// GetPromptBundle 获取提示词主内容、系统指令和变量定义
func (s *PromptService) GetPromptBundle(key string) PromptBundle {
	// 1. 尝试从缓存获取
	s.mu.RLock()
	item, ok := s.cache[key]
	s.mu.RUnlock()
	if ok && time.Now().Before(item.expiresAt) {
		return PromptBundle{
			Content:       item.content,
			SystemContent: item.systemContent,
			Variables:     item.variables,
		}
	}

	// 2. 尝试从数据库获取
	var row struct {
		Content       string `db:"content"`
		SystemContent string `db:"system_content"`
		Variables     string `db:"variables"`
	}
	err := s.db.Get(&row, "SELECT content, system_content, variables FROM prompt_template WHERE prompt_key = ? AND status = 1", key)
	if err == nil && (row.Content != "" || row.SystemContent != "" || row.Variables != "") {
		s.setCacheFull(key, row.Content, row.SystemContent, row.Variables)
		return PromptBundle{
			Content:       row.Content,
			SystemContent: row.SystemContent,
			Variables:     row.Variables,
		}
	}

	// 3. 记录日志
	log.Printf("[PromptService] Warning: No prompt found in DB for key: %s.", key)
	return PromptBundle{}
}

func (s *PromptService) getCache(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.cache[key]
	if !ok || time.Now().After(item.expiresAt) {
		return "", false
	}
	return item.content, true
}

func (s *PromptService) setCacheFull(key, content, systemContent, variables string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = promptCacheItem{
		content:       content,
		systemContent: systemContent,
		variables:     variables,
		expiresAt:     time.Now().Add(10 * time.Minute),
	}
}

// InvalidateCache 在后台更新提示词后，主动失效缓存
func (s *PromptService) InvalidateCache(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, key)
}
