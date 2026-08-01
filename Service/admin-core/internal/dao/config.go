package dao

import (
	"admin-core/internal/model"
	"time"

	"gorm.io/gorm"
)

// ConfigDAO 系统配置数据访问对象
type ConfigDAO struct {
	db *gorm.DB
}

// NewConfigDAO 创建ConfigDAO
func NewConfigDAO(db *gorm.DB) *ConfigDAO {
	return &ConfigDAO{db: db}
}

// Get 获取配置值
func (d *ConfigDAO) Get(key string) (*model.SystemConfig, error) {
	var config model.SystemConfig
	err := d.db.Where("`key` = ?", key).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetValue 获取配置字符串值
func (d *ConfigDAO) GetValue(key string, defaultVal string) string {
	var config model.SystemConfig
	err := d.db.Where("`key` = ?", key).First(&config).Error
	if err != nil {
		return defaultVal
	}
	return config.Value
}

// Set 设置配置值
func (d *ConfigDAO) Set(key, value, updatedBy string) error {
	config := &model.SystemConfig{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
		UpdatedBy: updatedBy,
	}

	result := d.db.Where("`key` = ?", key).Updates(map[string]interface{}{
		"value":      value,
		"updated_at": time.Now(),
		"updated_by": updatedBy,
	})

	if result.RowsAffected == 0 {
		config.Editable = true
		config.Type = "string"
		return d.db.Create(config).Error
	}

	return result.Error
}

// List 获取所有配置
func (d *ConfigDAO) List() ([]model.SystemConfig, error) {
	var configs []model.SystemConfig
	err := d.db.Order("`key` ASC").Find(&configs).Error
	return configs, err
}

// Delete 删除配置
func (d *ConfigDAO) Delete(key string) error {
	return d.db.Where("`key` = ?", key).Delete(&model.SystemConfig{}).Error
}

// BatchSet 批量设置配置
func (d *ConfigDAO) BatchSet(kvMap map[string]string, updatedBy string) error {
	tx := d.db.Begin()

	for key, value := range kvMap {
		result := tx.Where("`key` = ?", key).Updates(map[string]interface{}{
			"value":      value,
			"updated_at": time.Now(),
			"updated_by": updatedBy,
		})

		if result.RowsAffected == 0 {
			config := &model.SystemConfig{
				Key:       key,
				Value:     value,
				Type:      "string",
				Editable:  true,
				UpdatedAt: time.Now(),
				UpdatedBy: updatedBy,
			}
			if err := tx.Create(config).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit().Error
}