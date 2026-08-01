package handler

import (
	"admin-core/internal/model"
	"admin-core/internal/service"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证接口处理器
type AuthHandler struct {
	authService  *service.AuthService
	auditService *service.AuditService
}

// NewAuthHandler 创建AuthHandler
func NewAuthHandler(authService *service.AuthService, auditService *service.AuditService) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		auditService: auditService,
	}
}

// Register 用户注册
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	user, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	h.auditService.Log(user.ID, user.Username, "register", "user", "", "POST", "/api/v1/auth/register", c.ClientIP(), c.GetHeader("User-Agent"), "用户注册", 1)

	model.Success(c, gin.H{"user": user})
}

// Login 用户登录
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		// 记录失败登录
		h.auditService.Log(0, req.Username, "login_failed", "auth", "", "POST", "/api/v1/auth/login", c.ClientIP(), c.GetHeader("User-Agent"), err.Error(), 0)
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	h.auditService.Log(resp.User.ID, resp.User.Username, "login", "auth", "", "POST", "/api/v1/auth/login", c.ClientIP(), c.GetHeader("User-Agent"), "用户登录成功", 1)

	model.Success(c, resp)
}

// RefreshToken 刷新令牌
// POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req model.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	resp, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		model.Fail(c, model.CodeUnauthorized, err.Error())
		return
	}

	model.Success(c, resp)
}

// Logout 用户登出
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, _ := c.Get("user_id")
	token, _ := c.Get("token")

	if err := h.authService.Logout(c.Request.Context(), userID.(int64), token.(string)); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	username, _ := c.Get("username")
	h.auditService.Log(userID.(int64), username.(string), "logout", "auth", "", "POST", "/api/v1/auth/logout", c.ClientIP(), c.GetHeader("User-Agent"), "用户登出", 1)

	model.Success(c, gin.H{"message": "登出成功"})
}

// GetProfile 获取当前用户信息
// GET /api/v1/auth/profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	role, _ := c.Get("role")

	model.Success(c, gin.H{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"time":     time.Now().Format("2006-01-02 15:04:05"),
	})
}

// ChangePassword 修改密码
// POST /api/v1/auth/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.authService.ChangePassword(userID.(int64), req.OldPassword, req.NewPassword); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	h.auditService.Log(userID.(int64), username.(string), "change_password", "user", "", "POST", "/api/v1/auth/change-password", c.ClientIP(), c.GetHeader("User-Agent"), "修改密码", 1)

	model.Success(c, gin.H{"message": "密码修改成功"})
}