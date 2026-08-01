package config

import (
	"admin-core/internal/model"

	"gorm.io/gorm"
)

// DBInstance 返回初始化好的 gorm.DB（供 Bootstrap 使用）
func (c *Config) DBInstance() (*gorm.DB, error) {
	if dbCache != nil {
		return dbCache, nil
	}
	return nil, nil
}

// SetDBInstance main.go 启动成功后注入
func SetDBInstance(db *gorm.DB) {
	dbCache = db
}

var dbCache *gorm.DB

// AutoMigrateNew 迁移新系统所需的表
func AutoMigrateNew(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.FirewallRule{},
		&model.SSHBlock{},
		&model.Crontab{},
		&model.CrontabLog{},
		&model.TerminalSession{},
		&model.Bot{},
		&model.BotLog{},
		&model.BotConfig{},
	)
}

// AutoMigrateV2 main.go 使用的别名
func AutoMigrateV2(db *gorm.DB) error {
	return AutoMigrateNew(db)
}
