package main

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	Results   []SearchResult
	ExpiresAt time.Time
}

// SearchCache 搜索结果缓存
type SearchCache struct {
	mu      sync.RWMutex
	items   map[string]CacheItem
	ttl     time.Duration
}

// NewSearchCache 创建新的搜索缓存
func NewSearchCache(ttl time.Duration) *SearchCache {
	cache := &SearchCache{
		items: make(map[string]CacheItem),
		ttl:   ttl,
	}
	go cache.cleanupLoop()
	return cache
}

// Get 获取缓存结果
func (c *SearchCache) Get(key string) ([]SearchResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item.Results, true
}

// Set 设置缓存结果
func (c *SearchCache) Set(key string, results []SearchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = CacheItem{
		Results:   results,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete 删除缓存项
func (c *SearchCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear 清空所有缓存
func (c *SearchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]CacheItem)
}

// cleanupLoop 定期清理过期缓存
func (c *SearchCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.ExpiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// Stats 返回缓存统计信息
func (c *SearchCache) Stats() (total, expired int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	for _, item := range c.items {
		total++
		if now.After(item.ExpiresAt) {
			expired++
		}
	}
	return
}
