package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 全局 SSH 防爆破实例
var (
	globalSSHBruteForce *SSHBruteForce
	globalSSHOnce       sync.Once
)

// SetGlobalSSHBruteForce 设置全局实例
func SetGlobalSSHBruteForce(b *SSHBruteForce) {
	globalSSHOnce.Do(func() { globalSSHBruteForce = b })
}

// GetGlobalSSHBruteForce 获取全局实例
func GetGlobalSSHBruteForce() *SSHBruteForce {
	return globalSSHBruteForce
}

// WAFMiddleware WAF 级防护：SQL 注入/XSS/路径穿越
func WAFMiddleware() gin.HandlerFunc {
	sqlPatterns := []string{
		"drop table", "drop database", "union select", "union all select",
		"or 1=1", "or '1'='1", "or 1=1--", "insert into", "delete from",
		"information_schema", "sleep(", "xp_cmdshell", "sp_executesql",
		"' or '", "\" or \"", "waitfor delay", "--",
	}
	xssPatterns := []string{
		"<script", "javascript:", "onerror=", "onload=",
		"<iframe", "eval(", "alert(", "document.cookie",
	}

	return func(c *gin.Context) {
		qs := c.Request.URL.RawQuery
		body := ""
		if c.Request.Body != nil {
			buf := make([]byte, 4096)
			n, _ := c.Request.Body.Read(buf)
			body = string(buf[:n])
			c.Request.Body = &fakeReader{data: []byte(body)}
		}
		raw := qs + "\n" + body

		rawLow := strings.ToLower(raw)
		for _, p := range sqlPatterns {
			if strings.Contains(rawLow, p) {
				wafBlock(c, "sql_injection", p)
				return
			}
		}
		for _, p := range xssPatterns {
			if strings.Contains(rawLow, p) {
				wafBlock(c, "xss", p)
				return
			}
		}
		pathLow := strings.ToLower(c.Request.URL.Path)
		if strings.Contains(pathLow, "../") || strings.Contains(pathLow, "..\\") || strings.Contains(pathLow, "/etc/passwd") {
			wafBlock(c, "path_traversal", "path traversal")
			return
		}

		c.Next()
	}
}

func wafBlock(c *gin.Context, attack, detail string) {
	ip := c.ClientIP()
	fmt.Printf("[WAF] %s %s from %s: %s\n", time.Now().Format(time.RFC3339), attack, ip, detail)
	c.Header("X-WAF-Blocked", attack)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"code":   403,
		"msg":    "请求被安全策略拦截",
		"attack": attack,
	})
}

type fakeReader struct {
	data []byte
	pos  int
}

func (f *fakeReader) Read(p []byte) (int, error) {
	if f.pos >= len(f.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, f.data[f.pos:])
	f.pos += n
	return n, nil
}
func (f *fakeReader) Close() error { return nil }

// SSHBruteForce SSH 防爆破（IP 黑名单）
type SSHBruteForce struct {
	mu       sync.RWMutex
	attempts map[string]*attempt
	blocked  map[string]time.Time
	maxTry   int
	window   time.Duration
	blockDur time.Duration
}

type attempt struct {
	count    int
	lastTime time.Time
}

func NewSSHBruteForce(maxTry int, window, blockDur time.Duration) *SSHBruteForce {
	return &SSHBruteForce{
		attempts: make(map[string]*attempt),
		blocked:  make(map[string]time.Time),
		maxTry:   maxTry,
		window:   window,
		blockDur: blockDur,
	}
}

// Middleware 作为全局中间件，拦截已被封禁 IP
func (b *SSHBruteForce) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		b.mu.Lock()
		now := time.Now()
		for k, v := range b.blocked {
			if now.After(v) {
				delete(b.blocked, k)
			}
		}
		if expire, ok := b.blocked[ip]; ok {
			b.mu.Unlock()
			remaining := time.Until(expire).Seconds()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  fmt.Sprintf("IP %s 已被临时封禁，%.0f 秒后恢复", ip, remaining),
			})
			return
		}
		b.mu.Unlock()
		c.Next()
	}
}

// RecordFailed 记录登录失败；返回 true 表示已触发封禁
func (b *SSHBruteForce) RecordFailed(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.attempts[ip]
	if !ok || time.Since(a.lastTime) > b.window {
		b.attempts[ip] = &attempt{count: 1, lastTime: time.Now()}
		return false
	}
	a.count++
	a.lastTime = time.Now()
	if a.count >= b.maxTry {
		b.blocked[ip] = time.Now().Add(b.blockDur)
		go writeHostsDeny(ip)
		delete(b.attempts, ip)
		return true
	}
	return false
}

// RecordSuccess 登录成功清除记录
func (b *SSHBruteForce) RecordSuccess(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.attempts, ip)
}

// IsBlocked 查询是否封禁
func (b *SSHBruteForce) IsBlocked(ip string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	expire, ok := b.blocked[ip]
	return ok && time.Now().Before(expire)
}

// GetBlockedList 获取封禁列表
func (b *SSHBruteForce) GetBlockedList() []BlockedIP {
	b.mu.RLock()
	defer b.mu.RUnlock()
	list := make([]BlockedIP, 0, len(b.blocked))
	for ip, expire := range b.blocked {
		list = append(list, BlockedIP{
			IP:        ip,
			Expires:   expire.Format("2006-01-02 15:04:05"),
			Remaining: int(time.Until(expire).Seconds()),
		})
	}
	return list
}

// BlockedIP 对外暴露的封禁条目
type BlockedIP struct {
	IP        string `json:"ip"`
	Expires   string `json:"expires"`
	Remaining int    `json:"remaining"`
}

// Unblock 解封
func (b *SSHBruteForce) Unblock(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.blocked[ip]; ok {
		delete(b.blocked, ip)
		return true
	}
	return false
}

func writeHostsDeny(ip string) {
	f, err := os.OpenFile("/etc/hosts.deny", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(fmt.Sprintf("sshd: %s # shibosi deny at %s\n", ip, time.Now().Format(time.RFC3339)))
}

// ToJSON 辅助
func ToJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// IsPrivateIP 判断是否为内网 IP
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
