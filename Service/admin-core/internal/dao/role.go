package dao

import (
	"admin-core/internal/model"

	"gorm.io/gorm"
)

// RoleDAO 角色数据访问对象
type RoleDAO struct {
	db *gorm.DB
}

// NewRoleDAO 创建RoleDAO
func NewRoleDAO(db *gorm.DB) *RoleDAO {
	return &RoleDAO{db: db}
}

// Create 创建角色
func (d *RoleDAO) Create(role *model.Role) error {
	return d.db.Create(role).Error
}

// GetByID 根据ID获取角色
func (d *RoleDAO) GetByID(id int64) (*model.Role, error) {
	var role model.Role
	err := d.db.First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetByCode 根据Code获取角色
func (d *RoleDAO) GetByCode(code string) (*model.Role, error) {
	var role model.Role
	err := d.db.Where("code = ?", code).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// List 获取所有角色
func (d *RoleDAO) List() ([]model.Role, error) {
	var roles []model.Role
	err := d.db.Order("id ASC").Find(&roles).Error
	return roles, err
}

// Update 更新角色
func (d *RoleDAO) Update(id int64, updates map[string]interface{}) error {
	return d.db.Model(&model.Role{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除角色
func (d *RoleDAO) Delete(id int64) error {
	return d.db.Where("id = ?", id).Delete(&model.Role{}).Error
}

// GetRolePermissions 获取角色的权限列表
func (d *RoleDAO) GetRolePermissions(roleID int64) ([]model.Permission, error) {
	var permissions []model.Permission
	err := d.db.Table("permissions").
		Joins("INNER JOIN role_permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Find(&permissions).Error
	return permissions, err
}

// SetRolePermissions 设置角色权限（全量替换）
func (d *RoleDAO) SetRolePermissions(roleID int64, permissionIDs []int64) error {
	tx := d.db.Begin()

	// 删除旧权限
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 添加新权限
	for _, pid := range permissionIDs {
		rp := &model.RolePermission{
			RoleID:       roleID,
			PermissionID: pid,
		}
		if err := tx.Create(rp).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// GetAllPermissions 获取所有权限
func (d *RoleDAO) GetAllPermissions() ([]model.Permission, error) {
	var permissions []model.Permission
	err := d.db.Order("sort ASC, id ASC").Find(&permissions).Error
	return permissions, err
}

// GetUserRoles 获取用户的角色列表
func (d *RoleDAO) GetUserRoles(userID int64) ([]model.Role, error) {
	var roles []model.Role
	err := d.db.Table("roles").
		Joins("INNER JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

// AssignUserRole 为用户分配角色
func (d *RoleDAO) AssignUserRole(userID, roleID int64) error {
	ur := &model.UserRole{
		UserID: userID,
		RoleID: roleID,
	}
	return d.db.Create(ur).Error
}

// RemoveUserRole 移除用户角色
func (d *RoleDAO) RemoveUserRole(userID, roleID int64) error {
	return d.db.Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&model.UserRole{}).Error
}

// SetUserRoles 设置用户角色（全量替换）
func (d *RoleDAO) SetUserRoles(userID int64, roleIDs []int64) error {
	tx := d.db.Begin()

	if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, rid := range roleIDs {
		ur := &model.UserRole{UserID: userID, RoleID: rid}
		if err := tx.Create(ur).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}