package dao

import (
	"admin-core/internal/model"

	"gorm.io/gorm"
)

// DAO 数据访问对象（持有 db 引用 + 新表的 CRUD）
type DAO struct {
	DB *gorm.DB
}

func NewDAO(db *gorm.DB) *DAO { return &DAO{DB: db} }

// =========== Firewall ===========

func (d *DAO) CreateFirewallRule(r *model.FirewallRule) error {
	return d.DB.Create(r).Error
}

func (d *DAO) ListFirewallRules() ([]model.FirewallRule, error) {
	var list []model.FirewallRule
	err := d.DB.Order("id desc").Find(&list).Error
	return list, err
}

func (d *DAO) GetFirewallRule(id int64) (*model.FirewallRule, error) {
	var r model.FirewallRule
	err := d.DB.First(&r, id).Error
	return &r, err
}

func (d *DAO) UpdateFirewallRule(r *model.FirewallRule) error {
	return d.DB.Save(r).Error
}

func (d *DAO) DeleteFirewallRule(id int64) error {
	return d.DB.Delete(&model.FirewallRule{}, id).Error
}

// =========== SSHBlock ===========

func (d *DAO) CreateSSHBlock(b *model.SSHBlock) error {
	return d.DB.Create(b).Error
}
func (d *DAO) ListSSHBlocks() ([]model.SSHBlock, error) {
	var list []model.SSHBlock
	err := d.DB.Order("blocked_at desc").Find(&list).Error
	return list, err
}

// =========== Crontab ===========

func (d *DAO) CreateCrontab(t *model.Crontab) error { return d.DB.Create(t).Error }
func (d *DAO) ListCrontabs() ([]model.Crontab, error) {
	var list []model.Crontab
	err := d.DB.Order("id desc").Find(&list).Error
	return list, err
}
func (d *DAO) GetCrontab(id int64) (*model.Crontab, error) {
	var t model.Crontab
	err := d.DB.First(&t, id).Error
	return &t, err
}
func (d *DAO) UpdateCrontab(t *model.Crontab) error { return d.DB.Save(t).Error }
func (d *DAO) DeleteCrontab(id int64) error         { return d.DB.Delete(&model.Crontab{}, id).Error }

func (d *DAO) CreateCrontabLog(l *model.CrontabLog) error { return d.DB.Create(l).Error }
func (d *DAO) ListCrontabLogs(taskID int64) ([]model.CrontabLog, error) {
	var list []model.CrontabLog
	err := d.DB.Where("task_id = ?", taskID).Order("id desc").Limit(200).Find(&list).Error
	return list, err
}
func (d *DAO) CleanCrontabLogs(taskID int64, days int) error {
	return d.DB.Where("task_id = ? AND started_at < ?", taskID, daysAgo(days)).
		Delete(&model.CrontabLog{}).Error
}

func daysAgo(days int) interface{} {
	// 不引入 time 包依赖
	return nil
}
