package dao

import (
	"admin-core/internal/model"
	"time"

	"gorm.io/gorm"
)

// =========== Bot ===========

func (d *DAO) CreateBot(b *model.Bot) error {
	return d.DB.Create(b).Error
}

func (d *DAO) ListBots() ([]model.Bot, error) {
	var list []model.Bot
	err := d.DB.Order("id desc").Find(&list).Error
	return list, err
}

func (d *DAO) ListBotsByType(botType string) ([]model.Bot, error) {
	var list []model.Bot
	err := d.DB.Where("type = ?", botType).Order("id desc").Find(&list).Error
	return list, err
}

func (d *DAO) GetBot(id int64) (*model.Bot, error) {
	var b model.Bot
	err := d.DB.First(&b, id).Error
	return &b, err
}

func (d *DAO) UpdateBot(b *model.Bot) error {
	return d.DB.Save(b).Error
}

func (d *DAO) DeleteBot(id int64) error {
	return d.DB.Delete(&model.Bot{}, id).Error
}

func (d *DAO) GetBotStats() (map[string]interface{}, error) {
	var total int64
	var running int64
	var stopped int64
	var errorCount int64

	d.DB.Model(&model.Bot{}).Count(&total)
	d.DB.Model(&model.Bot{}).Where("status = ?", "running").Count(&running)
	d.DB.Model(&model.Bot{}).Where("status = ?", "stopped").Count(&stopped)
	d.DB.Model(&model.Bot{}).Where("status = ?", "error").Count(&errorCount)

	// 按类型统计
	var typeStats []struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	d.DB.Model(&model.Bot{}).Select("type, count(*) as count").Group("type").Scan(&typeStats)

	return map[string]interface{}{
		"total":         total,
		"running":       running,
		"stopped":       stopped,
		"error":         errorCount,
		"by_type":       typeStats,
	}, nil
}

// =========== BotLog ===========

func (d *DAO) CreateBotLog(l *model.BotLog) error {
	return d.DB.Create(l).Error
}

func (d *DAO) ListBotLogs(botID int64, limit int) ([]model.BotLog, error) {
	var list []model.BotLog
	query := d.DB.Model(&model.BotLog{})
	if botID > 0 {
		query = query.Where("bot_id = ?", botID)
	}
	if limit <= 0 {
		limit = 200
	}
	err := query.Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

func (d *DAO) CleanBotLogs(botID int64, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	query := d.DB.Where("created_at < ?", cutoff)
	if botID > 0 {
		query = query.Where("bot_id = ?", botID)
	}
	return query.Delete(&model.BotLog{}).Error
}

func (d *DAO) GetBotLogStats() (map[string]interface{}, error) {
	var total int64
	var todayCount int64
	var errorCount int64

	d.DB.Model(&model.BotLog{}).Count(&total)
	today := time.Now().Format("2006-01-02")
	d.DB.Model(&model.BotLog{}).Where("DATE(created_at) = ?", today).Count(&todayCount)
	d.DB.Model(&model.BotLog{}).Where("level IN ?", []string{"ERROR", "FATAL"}).Count(&errorCount)

	return map[string]interface{}{
		"total":      total,
		"today":      todayCount,
		"errors":     errorCount,
	}, nil
}

// =========== BotConfig ===========

func (d *DAO) CreateBotConfig(c *model.BotConfig) error {
	return d.DB.Create(c).Error
}

func (d *DAO) ListBotConfigs() ([]model.BotConfig, error) {
	var list []model.BotConfig
	err := d.DB.Order("category asc, key asc").Find(&list).Error
	return list, err
}

func (d *DAO) GetBotConfig(key string) (*model.BotConfig, error) {
	var c model.BotConfig
	err := d.DB.Where("key = ?", key).First(&c).Error
	return &c, err
}

func (d *DAO) UpdateBotConfig(c *model.BotConfig) error {
	return d.DB.Save(c).Error
}

func (d *DAO) DeleteBotConfig(id int64) error {
	return d.DB.Delete(&model.BotConfig{}, id).Error
}

// GetDB 返回数据库实例
func (d *DAO) GetDB() *gorm.DB {
	return d.DB
}
