package middleware

import (
	"bytes"
	"html"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SQL注入检测正则（匹配常见的SQL注入模式）
var sqlInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\bselect\b.*\bfrom\b)|(\binsert\b.*\binto\b)|(\bupdate\b.*\bset\b)|(\bdelete\b.*\bfrom\b)`),
	regexp.MustCompile(`(?i)(\bdrop\b.*\btable\b)|(\balter\b.*\btable\b)|(\btruncate\b.*\btable\b)`),
	regexp.MustCompile(`(?i)(\bunion\b.*\bselect\b)|(\bexec\b\(.*\))|(\bexecute\b\(.*\))`),
	regexp.MustCompile(`(?i)(--|\#|\/\*|\*\/|;)`),
	regexp.MustCompile(`(?i)(\bwaitfor\b\s+delay\b)|(\bbenchmark\b\(.*\))`),
	regexp.MustCompile(`(?i)(\bxp_cmdshell\b)|(\bwget\b)|(\bcurl\b)`),
}

// XSS检测正则
var xssPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<script[^>]*>[\s\S]*?</script>`),
	regexp.MustCompile(`(?i)<iframe[^>]*>[\s\S]*?</iframe>`),
	regexp.MustCompile(`(?i)<object[^>]*>[\s\S]*?</object>`),
	regexp.MustCompile(`(?i)<embed[^>]*>`),
	regexp.MustCompile(`(?i)javascript\s*:`),
	regexp.MustCompile(`(?i)(on\w+\s*=)`),
	regexp.MustCompile(`(?i)(expression\s*\()`),
	regexp.MustCompile(`(?i)(eval\s*\()`),
}

// SafeParam 安全的参数值白名单
var safeParamPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\s,.@:/\\]+$`)

// containsSQLInjection 检测字符串是否包含SQL注入
func containsSQLInjection(s string) bool {
	for _, pattern := range sqlInjectionPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}

// containsXSS 检测字符串是否包含XSS攻击
func containsXSS(s string) bool {
	for _, pattern := range xssPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}

// sanitizeString 对字符串进行XSS清洗
func sanitizeString(s string) string {
	// HTML实体编码
	s = html.EscapeString(s)
	// 移除危险字符
	s = strings.ReplaceAll(s, "javascript:", "")
	s = strings.ReplaceAll(s, "onerror", "")
	s = strings.ReplaceAll(s, "onload", "")
	return s
}

// GinSQLInjectionFilter SQL注入过滤中间件
// 检测请求参数中的SQL注入模式，拦截恶意请求
func GinSQLInjectionFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查Query参数
		for _, values := range c.Request.URL.Query() {
			for _, v := range values {
				if containsSQLInjection(v) {
					c.AbortWithStatusJSON(400, gin.H{
						"code":      400,
						"msg":       "检测到非法请求参数",
						"timestamp": time.Now().UnixMilli(),
					})
					return
				}
			}
		}

		// 检查请求体（仅当Content-Type为application/json时）
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				bodyStr := string(bodyBytes)
				if containsSQLInjection(bodyStr) {
					c.AbortWithStatusJSON(400, gin.H{
						"code":      400,
						"msg":       "请求体包含非法字符",
						"timestamp": time.Now().UnixMilli(),
					})
					return
				}
				// 恢复Body供后续读取
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		c.Next()
	}
}

// GinXSSFilter XSS过滤中间件
// 检测并清洗请求中的XSS攻击向量
func GinXSSFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查Query参数
		query := c.Request.URL.Query()
		modified := false
		for key, values := range query {
			for i, v := range values {
				if containsXSS(v) {
					query[key][i] = sanitizeString(v)
					modified = true
				}
			}
		}
		if modified {
			c.Request.URL.RawQuery = query.Encode()
		}

		// 检查请求体
		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "application/json") {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				bodyStr := string(bodyBytes)
				if containsXSS(bodyStr) {
					bodyStr = sanitizeString(bodyStr)
				}
				c.Request.Body = io.NopCloser(bytes.NewBufferString(bodyStr))
			}
		}

		c.Next()
	}
}

// GinSecurityHeaders 安全响应头中间件
func GinSecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}