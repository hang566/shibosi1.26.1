package middleware

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 结构化日志中间件
type Logger struct {
	file   *os.File
	logger *log.Logger
}

// NewLogger 创建日志中间件
func NewLogger(logPath string, maxSizeMB int) (*Logger, error) {
	l := &Logger{}

	if logPath != "" {
		dir := filepath.Dir(logPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}

		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		l.file = f

		// 多输出: 文件 + 控制台
		multiWriter := io.MultiWriter(f, os.Stdout)
		l.logger = log.New(multiWriter, "[shibosi] ", log.LstdFlags|log.Lshortfile)
	} else {
		l.logger = log.New(os.Stdout, "[shibosi] ", log.LstdFlags|log.Lshortfile)
	}

	return l, nil
}

// Close 关闭日志文件
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// Info 记录信息日志
func (l *Logger) Info(format string, v ...interface{}) {
	l.logger.Printf("[INFO] "+format, v...)
}

// Warn 记录警告日志
func (l *Logger) Warn(format string, v ...interface{}) {
	l.logger.Printf("[WARN] "+format, v...)
}

// Error 记录错误日志
func (l *Logger) Error(format string, v ...interface{}) {
	l.logger.Printf("[ERROR] "+format, v...)
}

// GinLogger Gin请求日志中间件
func (l *Logger) GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		l.logger.Printf("%s | %3d | %13v | %15s | %-7s | %s",
			time.Now().Format("2006-01-02 15:04:05.000"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)
	}
}

// GinCORS CORS中间件（可配置来源）
func GinCORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		allowOrigin := ""
		sendCredentials := false

		for _, allowed := range allowedOrigins {
			if allowed == "*" {
				if origin != "" {
					allowOrigin = origin
					sendCredentials = true
				} else {
					allowOrigin = "*"
				}
				break
			}
			if allowed == origin {
				allowOrigin = origin
				sendCredentials = true
				break
			}
		}

		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Requested-With")
			c.Header("Access-Control-Expose-Headers", "Content-Length, X-Request-Id")
			c.Header("Access-Control-Max-Age", "3600")
			if sendCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Vary", "Origin")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}

		c.Next()
	}
}

// GinRecovery 全局异常恢复中间件
func GinRecovery(l *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录堆栈信息
				l.Error("PANIC recovered: %v | path: %s | method: %s | ip: %s",
					err,
					c.Request.URL.Path,
					c.Request.Method,
					c.ClientIP(),
				)

				c.AbortWithStatusJSON(500, gin.H{
					"code":      500,
					"msg":       "服务器内部错误，请稍后重试",
					"timestamp": time.Now().UnixMilli(),
				})
			}
		}()

		c.Next()
	}
}