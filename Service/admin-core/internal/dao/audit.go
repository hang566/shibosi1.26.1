package dao

import (
	"admin-core/internal/model"
	"time"

	"gorm.io/gorm"
)

// AuditDAO 审计日志数据访问对象
type AuditDAO struct {
	db *gorm.DB
}

// NewAuditDAO 创建AuditDAO
func NewAuditDAO(db *gorm.DB) *AuditDAO {
	return &AuditDAO{db: db}
}

// Create 创建审计日志
func (d *AuditDAO) Create(log *model.AuditLog) error {
	return d.db.Create(log).Error
}

// CreateAsync 异步创建审计日志（通过channel）
func (d *AuditDAO) CreateAsync(logChan <-chan *model.AuditLog) {
	go func() {
		batch := make([]*model.AuditLog, 0, 100)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case log, ok := <-logChan:
				if !ok {
					// channel关闭，刷入剩余数据
					if len(batch) > 0 {
						d.db.Create(&batch)
					}
					return
				}
				batch = append(batch, log)
				if len(batch) >= 100 {
					d.db.Create(&batch)
					batch = batch[:0]
				}
			case <-ticker.C:
				if len(batch) > 0 {
					d.db.Create(&batch)
					batch = batch[:0]
				}
			}
		}
	}()
}

// Query 多维检索审计日志
func (d *AuditDAO) Query(username, action, method string, startTime, endTime *time.Time, page, pageSize int) ([]model.AuditLog, int64, error) {
	query := d.db.Model(&model.AuditLog{})

	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if startTime != nil {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime != nil {
		query = query.Where("created_at <= ?", endTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AuditLog
	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// CleanOldLogs 清理过期日志
func (d *AuditDAO) CleanOldLogs(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := d.db.Where("created_at < ?", cutoff).Delete(&model.AuditLog{})
	return result.RowsAffected, result.Error
}

// GetActionTypes 获取所有操作类型
func (d *AuditDAO) GetActionTypes() ([]string, error) {
	var actions []string
	err := d.db.Model(&model.AuditLog{}).Distinct("action").Pluck("action", &actions).Error
	return actions, err
}

// CountByAction 统计操作类型数量
func (d *AuditDAO) CountByAction(action string) (int64, error) {
	var count int64
	err := d.db.Model(&model.AuditLog{}).Where("action = ?", action).Count(&count).Error
	return count, err
}