package middleware

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TokenBucket 令牌桶限流器
type TokenBucket struct {
	rate       float64 // 每秒生成令牌数 (QPS)
	burst      int     // 最大突发令牌数
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(rate float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许通过
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}
	tb.lastUpdate = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// RateLimiter 基于IP的限流器
type RateLimiter struct {
	buckets  map[string]*TokenBucket
	rate     float64
	burst    int
	mu       sync.RWMutex
	stopChan chan struct{}
}

// NewRateLimiter 创建限流器
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*TokenBucket),
		rate:     rate,
		burst:    burst,
		stopChan: make(chan struct{}),
	}

	// 定期清理过期的桶
	go rl.cleanup()

	return rl
}

// getClientIP 获取客户端真实IP（支持代理）
func getClientIP(c *gin.Context) string {
	// X-Forwarded-For 优先
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// X-Real-IP
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// 直连IP
	ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	return ip
}

// Allow 检查指定IP是否允许通过
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// 双重检查
		bucket, exists = rl.buckets[ip]
		if !exists {
			bucket = NewTokenBucket(rl.rate, rl.burst)
			rl.buckets[ip] = bucket
		}
		rl.mu.Unlock()
	}

	return bucket.Allow()
}

// cleanup 定期清理长时间未使用的桶
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for ip, bucket := range rl.buckets {
				bucket.mu.Lock()
				if time.Since(bucket.lastUpdate) > 10*time.Minute {
					delete(rl.buckets, ip)
				}
				bucket.mu.Unlock()
			}
			rl.mu.Unlock()
		case <-rl.stopChan:
			return
		}
	}
}

// Stop 停止清理协程
func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
}

// GinRateLimit Gin限流中间件
// rate: 每秒允许的请求数 (QPS)
// burst: 突发请求数
func GinRateLimit(rate float64, burst int) gin.HandlerFunc {
	limiter := NewRateLimiter(rate, burst)

	return func(c *gin.Context) {
		ip := getClientIP(c)

		if !limiter.Allow(ip) {
			c.AbortWithStatusJSON(429, gin.H{
				"code":      429,
				"msg":       "请求过于频繁，请稍后再试",
				"timestamp": time.Now().UnixMilli(),
			})
			return
		}

		c.Next()
	}
}

// GinAdminIPWhitelist 管理接口IP白名单中间件
func GinAdminIPWhitelist(allowedIPs []string) gin.HandlerFunc {
	if len(allowedIPs) == 0 {
		// 如果未配置白名单，则允许所有内网IP
		allowedIPs = []string{"127.0.0.1", "::1", "10.", "172.16.", "192.168."}
	}

	return func(c *gin.Context) {
		ip := getClientIP(c)

		for _, allowed := range allowedIPs {
			// 支持CIDR匹配
			if strings.Contains(allowed, "/") {
				_, cidr, err := net.ParseCIDR(allowed)
				if err == nil && cidr.Contains(net.ParseIP(ip)) {
					c.Next()
					return
				}
			}

			// 支持前缀匹配 (如 192.168.)
			if strings.HasSuffix(allowed, ".") {
				if strings.HasPrefix(ip, allowed) {
					c.Next()
					return
				}
			}

			// 完全匹配
			if ip == allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(403, gin.H{
			"code":      403,
			"msg":       "IP不在白名单中，无法访问管理接口",
			"timestamp": time.Now().UnixMilli(),
		})
	}
}