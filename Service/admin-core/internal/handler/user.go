package handler

import (
	"admin-core/internal/model"
	"admin-core/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

// UserHandler 用户管理接口处理器
type UserHandler struct {
	userService  *service.UserService
	authService  *service.AuthService
	auditService *service.AuditService
}

// NewUserHandler 创建UserHandler
func NewUserHandler(userService *service.UserService, authService *service.AuthService, auditService *service.AuditService) *UserHandler {
	return &UserHandler{
		userService:  userService,
		authService:  authService,
		auditService: auditService,
	}
}

// ListUsers 获取用户列表
// GET /api/v1/admin/users
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	role := c.Query("role")
	status := c.Query("status")

	users, total, err := h.userService.ListUsers(page, pageSize, role, status)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.SuccessPage(c, users, total, page, pageSize)
}

// GetUser 获取单个用户
// GET /api/v1/admin/users/:id
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的用户ID")
		return
	}

	user, err := h.userService.GetUserByID(id)
	if err != nil {
		model.Fail(c, model.CodeNotFound, err.Error())
		return
	}

	model.Success(c, user)
}

// UpdateUser 更新用户信息
// PUT /api/v1/admin/users/:id
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的用户ID")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userService.UpdateUser(id, updates); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	username, _ := c.Get("username")
	h.auditService.Log(id, username.(string), "update_user", "user", c.Param("id"), "PUT", c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), "更新用户信息", 1)

	model.Success(c, gin.H{"message": "更新成功"})
}

// UpdateUserRole 更新用户角色
// PUT /api/v1/admin/users/:id/role
func (h *UserHandler) UpdateUserRole(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的用户ID")
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userService.UpdateUserRole(id, req.Role); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	username, _ := c.Get("username")
	h.auditService.Log(id, username.(string), "update_role", "user", c.Param("id"), "PUT", c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), "更新用户角色为: "+req.Role, 1)

	model.Success(c, gin.H{"message": "角色更新成功"})
}

// ToggleStatus 启用/禁用用户
// PUT /api/v1/admin/users/:id/status
func (h *UserHandler) ToggleStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的用户ID")
		return
	}

	var req struct {
		Status int `json:"status" binding:"required,oneof=0 1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userService.ToggleUserStatus(id, req.Status); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	username, _ := c.Get("username")
	action := "enable"
	if req.Status == 0 {
		action = "disable"
	}
	h.auditService.Log(id, username.(string), action+"_user", "user", c.Param("id"), "PUT", c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), "启用/禁用用户", 1)

	model.Success(c, gin.H{"message": "操作成功"})
}

// ResetPassword 重置用户密码
// POST /api/v1/admin/users/:id/reset-password
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的用户ID")
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.authService.ResetPassword(id, req.NewPassword); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	username, _ := c.Get("username")
	h.auditService.Log(id, username.(string), "reset_password", "user", c.Param("id"), "POST", c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), "管理员重置密码", 1)

	model.Success(c, gin.H{"message": "密码重置成功"})
}

// CreateUser 创建用户（管理员）
// POST /api/v1/admin/users
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=3"`
		Password string `json:"password" binding:"required,min=6"`
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	regReq := &model.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	}
	user, err := h.authService.Register(c.Request.Context(), regReq)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	// 更新额外字段
	updates := map[string]interface{}{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Role != "user" {
		updates["role"] = req.Role
	}
	if len(updates) > 0 {
		h.userService.UpdateUser(user.ID, updates)
	}

	username, _ := c.Get("username")
	h.auditService.Log(user.ID, username.(string), "create_user", "user", strconv.FormatInt(user.ID, 10), "POST", c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), "创建用户: "+req.Username, 1)

	model.Success(c, gin.H{"message": "用户创建成功", "id": user.ID})
}

// DeleteUser 删除用户
// DELETE /api/v1/admin/users/:id
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的用户ID")
		return
	}

	if err := h.userService.DeleteUser(id); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	username, _ := c.Get("username")
	h.auditService.Log(id, username.(string), "delete_user", "user", c.Param("id"), "DELETE", c.Request.URL.Path, c.ClientIP(), c.GetHeader("User-Agent"), "删除用户", 1)

	model.Success(c, gin.H{"message": "删除成功"})
}

// GetUserStats 获取用户统计
// GET /api/v1/admin/users/stats
func (h *UserHandler) GetUserStats(c *gin.Context) {
	stats, err := h.userService.GetUserStats()
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}
	model.Success(c, stats)
}