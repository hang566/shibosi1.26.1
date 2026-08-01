package middleware

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Permission 权限定义
type Permission struct {
	Code   string
	Method string
	Path   string
}

// RBACManager 基于角色的访问控制管理器
type RBACManager struct {
	// role -> []Permission
	rolePermissions map[string][]Permission
	// 权限缓存（用于快速匹配）
	permissionCache map[string]bool
	mu              sync.RWMutex
}

// NewRBACManager 创建RBAC管理器
func NewRBACManager() *RBACManager {
	return &RBACManager{
		rolePermissions: make(map[string][]Permission),
		permissionCache: make(map[string]bool),
	}
}

// SetRolePermissions 设置角色权限（支持热更新）
func (r *RBACManager) SetRolePermissions(role string, permissions []Permission) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rolePermissions[role] = permissions
}

// AddPermission 为角色添加权限
func (r *RBACManager) AddPermission(role string, perm Permission) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rolePermissions[role] = append(r.rolePermissions[role], perm)
}

// RemovePermission 移除角色权限
func (r *RBACManager) RemovePermission(role string, code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	perms := r.rolePermissions[role]
	for i, p := range perms {
		if p.Code == code {
			r.rolePermissions[role] = append(perms[:i], perms[i+1:]...)
			break
		}
	}
}

// HasPermission 检查角色是否拥有指定权限
func (r *RBACManager) HasPermission(role, method, path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	perms, ok := r.rolePermissions[role]
	if !ok {
		return false
	}

	// 缓存键
	cacheKey := fmt.Sprintf("%s:%s:%s", role, method, path)
	if allowed, exists := r.permissionCache[cacheKey]; exists {
		return allowed
	}

	for _, p := range perms {
		if matchPermission(p, method, path) {
			r.permissionCache[cacheKey] = true
			return true
		}
	}

	r.permissionCache[cacheKey] = false
	return false
}

// ClearCache 清除权限缓存（权限变更时调用）
func (r *RBACManager) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.permissionCache = make(map[string]bool)
}

// GetAllRoles 获取所有角色列表
func (r *RBACManager) GetAllRoles() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roles := make([]string, 0, len(r.rolePermissions))
	for role := range r.rolePermissions {
		roles = append(roles, role)
	}
	return roles
}

// GetRolePermissions 获取角色的所有权限
func (r *RBACManager) GetRolePermissions(role string) []Permission {
	r.mu.RLock()
	defer r.mu.RUnlock()

	perms, ok := r.rolePermissions[role]
	if !ok {
		return []Permission{}
	}
	return perms
}

// matchPermission 匹配权限（支持通配符）
// 例如: GET /api/users/* 匹配 GET /api/users/123
func matchPermission(perm Permission, method, path string) bool {
	// 方法匹配
	if perm.Method != "" && !strings.EqualFold(perm.Method, method) {
		return false
	}

	// 路径匹配（支持通配符 ** 和 *）
	permPath := strings.TrimRight(perm.Path, "/")
	reqPath := strings.TrimRight(path, "/")

	// 完全匹配
	if permPath == reqPath {
		return true
	}

	// 通配符匹配
	if strings.HasSuffix(permPath, "/**") {
		prefix := strings.TrimSuffix(permPath, "/**")
		return strings.HasPrefix(reqPath, prefix)
	}

	if strings.Contains(permPath, "*") {
		parts := strings.Split(permPath, "/")
		reqParts := strings.Split(reqPath, "/")

		if len(parts) != len(reqParts) {
			return false
		}

		for i, part := range parts {
			if part != "*" && part != reqParts[i] {
				return false
			}
		}
		return true
	}

	return false
}

// GinRBAC Gin RBAC中间件
func GinRBAC(rbacManager *RBACManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(403, gin.H{
				"code": 403, "msg": "未获取到用户角色", "timestamp": time.Now().UnixMilli(),
			})
			return
		}

		roleStr := role.(string)

		// 管理员拥有所有权限
		if roleStr == "admin" {
			c.Next()
			return
		}

		if !rbacManager.HasPermission(roleStr, c.Request.Method, c.Request.URL.Path) {
			c.AbortWithStatusJSON(403, gin.H{
				"code": 403, "msg": "权限不足，无法访问该接口", "timestamp": time.Now().UnixMilli(),
			})
			return
		}

		c.Next()
	}
}