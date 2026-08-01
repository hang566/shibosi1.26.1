package handler

import (
	"admin-core/internal/dao"
	"admin-core/internal/model"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RoleHandler 角色权限接口处理器
type RoleHandler struct {
	roleDAO *dao.RoleDAO
}

// NewRoleHandler 创建RoleHandler
func NewRoleHandler(roleDAO *dao.RoleDAO) *RoleHandler {
	return &RoleHandler{roleDAO: roleDAO}
}

// ListRoles 获取角色列表
// GET /api/v1/admin/roles
func (h *RoleHandler) ListRoles(c *gin.Context) {
	roles, err := h.roleDAO.List()
	if err != nil {
		model.Fail(c, model.CodeInternal, "获取角色列表失败")
		return
	}
	model.Success(c, roles)
}

// CreateRole 创建角色
// POST /api/v1/admin/roles
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.roleDAO.Create(&role); err != nil {
		model.Fail(c, model.CodeInternal, "创建角色失败: "+err.Error())
		return
	}

	model.Success(c, role)
}

// UpdateRole 更新角色
// PUT /api/v1/admin/roles/:id
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的角色ID")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.roleDAO.Update(id, updates); err != nil {
		model.Fail(c, model.CodeInternal, "更新角色失败")
		return
	}

	model.Success(c, gin.H{"message": "更新成功"})
}

// DeleteRole 删除角色
// DELETE /api/v1/admin/roles/:id
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的角色ID")
		return
	}

	if err := h.roleDAO.Delete(id); err != nil {
		model.Fail(c, model.CodeInternal, "删除角色失败")
		return
	}

	model.Success(c, gin.H{"message": "删除成功"})
}

// GetRolePermissions 获取角色权限
// GET /api/v1/admin/roles/:id/permissions
func (h *RoleHandler) GetRolePermissions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的角色ID")
		return
	}

	permissions, err := h.roleDAO.GetRolePermissions(id)
	if err != nil {
		model.Fail(c, model.CodeInternal, "获取权限列表失败")
		return
	}

	model.Success(c, permissions)
}

// SetRolePermissions 设置角色权限
// PUT /api/v1/admin/roles/:id/permissions
func (h *RoleHandler) SetRolePermissions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的角色ID")
		return
	}

	var req struct {
		PermissionIDs []int64 `json:"permission_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.roleDAO.SetRolePermissions(id, req.PermissionIDs); err != nil {
		model.Fail(c, model.CodeInternal, "设置权限失败")
		return
	}

	model.Success(c, gin.H{"message": "权限设置成功"})
}

// GetAllPermissions 获取所有权限
// GET /api/v1/admin/permissions
func (h *RoleHandler) GetAllPermissions(c *gin.Context) {
	permissions, err := h.roleDAO.GetAllPermissions()
	if err != nil {
		model.Fail(c, model.CodeInternal, "获取权限列表失败")
		return
	}
	model.Success(c, permissions)
}