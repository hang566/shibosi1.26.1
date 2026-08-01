package dao

import (
	"admin-core/internal/model"
	"context"
	"time"

	"gorm.io/gorm"
)

// UserDAO 用户数据访问对象
type UserDAO struct {
	db *gorm.DB
}

// NewUserDAO 创建UserDAO
func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{db: db}
}

// Create 创建用户
func (d *UserDAO) Create(user *model.User) error {
	return d.db.Create(user).Error
}

// GetByID 根据ID获取用户
func (d *UserDAO) GetByID(id int64) (*model.User, error) {
	var user model.User
	err := d.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (d *UserDAO) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := d.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// List 分页查询用户列表
func (d *UserDAO) List(page, pageSize int, role, status string) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := d.db.Model(&model.User{})

	if role != "" {
		query = query.Where("role = ?", role)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&users).Error
	return users, total, err
}

// Update 更新用户信息
func (d *UserDAO) Update(id int64, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return d.db.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

// UpdatePassword 更新密码
func (d *UserDAO) UpdatePassword(id int64, hashedPassword string) error {
	return d.db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password":   hashedPassword,
		"updated_at": time.Now(),
	}).Error
}

// UpdateLastLogin 更新最后登录时间
func (d *UserDAO) UpdateLastLogin(id int64) error {
	return d.db.Model(&model.User{}).Where("id = ?", id).Update("last_login", time.Now()).Error
}

// Delete 软删除用户（设置状态为禁用）
func (d *UserDAO) Delete(id int64) error {
	return d.db.Model(&model.User{}).Where("id = ?", id).Update("status", 0).Error
}

// HardDelete 硬删除用户
func (d *UserDAO) HardDelete(id int64) error {
	return d.db.Where("id = ?", id).Delete(&model.User{}).Error
}

// CountByRole 统计各角色用户数
func (d *UserDAO) CountByRole(role string) (int64, error) {
	var count int64
	err := d.db.Model(&model.User{}).Where("role = ? AND status = 1", role).Count(&count).Error
	return count, err
}

// CountTodayNew 统计今日新增用户
func (d *UserDAO) CountTodayNew() (int64, error) {
	var count int64
	today := time.Now().Format("2006-01-02")
	err := d.db.Model(&model.User{}).
		Where("DATE(created_at) = ?", today).
		Count(&count).Error
	return count, err
}

// GetStats 获取用户统计信息
func (d *UserDAO) GetStats(ctx context.Context) (map[string]int64, error) {
	stats := make(map[string]int64)

	var total, active, disabled int64
	d.db.Model(&model.User{}).Count(&total)
	d.db.Model(&model.User{}).Where("status = 1").Count(&active)
	d.db.Model(&model.User{}).Where("status = 0").Count(&disabled)

	stats["total"] = total
	stats["active"] = active
	stats["disabled"] = disabled

	return stats, nil
}