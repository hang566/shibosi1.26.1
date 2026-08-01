package service

import (
	"admin-core/internal/dao"
	"admin-core/internal/model"
	"fmt"
)

// UserService 用户管理服务
type UserService struct {
	userDAO *dao.UserDAO
	roleDAO *dao.RoleDAO
}

// NewUserService 创建UserService
func NewUserService(userDAO *dao.UserDAO, roleDAO *dao.RoleDAO) *UserService {
	return &UserService{
		userDAO: userDAO,
		roleDAO: roleDAO,
	}
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(id int64) (*model.User, error) {
	user, err := s.userDAO.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return user, nil
}

// ListUsers 分页查询用户
func (s *UserService) ListUsers(page, pageSize int, role, status string) ([]model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.userDAO.List(page, pageSize, role, status)
}

// UpdateUser 更新用户信息
func (s *UserService) UpdateUser(id int64, updates map[string]interface{}) error {
	// 不允许通过此接口修改角色
	delete(updates, "role")
	delete(updates, "password")
	delete(updates, "status")

	return s.userDAO.Update(id, updates)
}

// UpdateUserRole 更新用户角色
func (s *UserService) UpdateUserRole(userID int64, role string) error {
	validRoles := map[string]bool{"admin": true, "user": true, "guest": true, "editor": true}
	if !validRoles[role] {
		return fmt.Errorf("无效的角色: %s", role)
	}
	return s.userDAO.Update(userID, map[string]interface{}{"role": role})
}

// ToggleUserStatus 启用/禁用用户
func (s *UserService) ToggleUserStatus(userID int64, status int) error {
	if status != 0 && status != 1 {
		return fmt.Errorf("无效的状态值")
	}
	return s.userDAO.Update(userID, map[string]interface{}{"status": status})
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(userID int64) error {
	user, err := s.userDAO.GetByID(userID)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}
	if user.Role == "admin" {
		return fmt.Errorf("不能删除管理员账户")
	}
	return s.userDAO.Delete(userID)
}

// ResetUserPassword 重置用户密码（管理员操作）
func (s *UserService) ResetUserPassword(userID int64, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("密码至少6个字符")
	}
	// 注意：这里需要调用authService的密码哈希
	return nil
}

// GetUserStats 获取用户统计
func (s *UserService) GetUserStats() (map[string]interface{}, error) {
	stats, err := s.userDAO.GetStats(nil)
	if err != nil {
		return nil, err
	}

	todayNew, _ := s.userDAO.CountTodayNew()

	return map[string]interface{}{
		"total":      stats["total"],
		"active":     stats["active"],
		"disabled":   stats["disabled"],
		"today_new":  todayNew,
	}, nil
}

// AssignUserRoles 为用户分配角色
func (s *UserService) AssignUserRoles(userID int64, roleIDs []int64) error {
	return s.roleDAO.SetUserRoles(userID, roleIDs)
}