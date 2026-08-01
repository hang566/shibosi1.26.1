package middleware

import (
	"encoding/base64"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims 自定义JWT声明
type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager JWT令牌管理器
type JWTManager struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	blacklist       map[string]time.Time
	blacklistMu     sync.RWMutex
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(secret string, accessTTL, refreshTTL int) *JWTManager {
	m := &JWTManager{
		secret:          []byte(secret),
		accessTokenTTL:  time.Duration(accessTTL) * time.Minute,
		refreshTokenTTL: time.Duration(refreshTTL) * time.Minute,
		blacklist:       make(map[string]time.Time),
	}

	// 定期清理过期黑名单
	go m.cleanBlacklist()

	return m
}

// GenerateAccessToken 生成访问令牌
func (m *JWTManager) GenerateAccessToken(userID int64, username, role string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "shibosi-admin-core",
			Subject:   fmt.Sprintf("%d", userID),
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefreshToken 生成刷新令牌
func (m *JWTManager) GenerateRefreshToken(userID int64, username, role string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "shibosi-admin-core",
			Subject:   fmt.Sprintf("refresh:%d", userID),
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseToken 解析并验证令牌
func (m *JWTManager) ParseToken(tokenString string) (*JWTClaims, error) {
	// 检查黑名单
	if m.IsBlacklisted(tokenString) {
		return nil, fmt.Errorf("令牌已失效")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("签名算法不匹配: %v", token.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("无效的令牌")
	}

	return claims, nil
}

// AddToBlacklist 将令牌加入黑名单
func (m *JWTManager) AddToBlacklist(tokenString string, exp time.Time) {
	m.blacklistMu.Lock()
	defer m.blacklistMu.Unlock()
	m.blacklist[tokenString] = exp
}

// IsBlacklisted 检查令牌是否在黑名单中
func (m *JWTManager) IsBlacklisted(tokenString string) bool {
	m.blacklistMu.RLock()
	defer m.blacklistMu.RUnlock()
	_, exists := m.blacklist[tokenString]
	return exists
}

// cleanBlacklist 定期清理过期黑名单条目
func (m *JWTManager) cleanBlacklist() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.blacklistMu.Lock()
		now := time.Now()
		for token, exp := range m.blacklist {
			if now.After(exp) {
				delete(m.blacklist, token)
			}
		}
		m.blacklistMu.Unlock()
	}
}

// generateTokenID 生成唯一的Token ID
func generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// GinJWTAuth Gin JWT鉴权中间件
func GinJWTAuth(jwtManager *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"code": 401, "msg": "未提供认证令牌", "timestamp": time.Now().UnixMilli(),
			})
			return
		}

		claims, err := jwtManager.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"code": 401, "msg": "令牌无效或已过期", "timestamp": time.Now().UnixMilli(),
			})
			return
		}

		// 将用户信息注入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("token", tokenString)
		c.Set("token_exp", claims.ExpiresAt.Time)

		c.Next()
	}
}

// GinRequireRole 角色校验中间件
func GinRequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(403, gin.H{
				"code": 403, "msg": "无权访问", "timestamp": time.Now().UnixMilli(),
			})
			return
		}

		roleStr := userRole.(string)
		for _, r := range roles {
			if r == roleStr {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(403, gin.H{
			"code": 403, "msg": "权限不足", "timestamp": time.Now().UnixMilli(),
		})
	}
}

// extractToken 从请求头提取Token (支持 Bearer 和 X-API-Key)
func extractToken(c *gin.Context) string {
	// 优先从 Authorization Header 获取
	auth := c.GetHeader("Authorization")
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	// 备选: 从 X-API-Key 获取
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// 备选: 从 Query 参数获取
	token := c.Query("token")
	if token != "" {
		return token
	}

	return ""
}