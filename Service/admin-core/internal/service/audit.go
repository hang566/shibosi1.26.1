package service

import (
	"admin-core/internal/dao"
	"admin-core/internal/model"
	"fmt"
	"time"
)

// AuditService 审计日志服务
type AuditService struct {
	auditDAO *dao.AuditDAO
	logChan  chan *model.AuditLog
}

// NewAuditService 创建AuditService
func NewAuditService(auditDAO *dao.AuditDAO) *AuditService {
	s := &AuditService{
		auditDAO: auditDAO,
		logChan:  make(chan *model.AuditLog, 1000),
	}

	// 启动异步写入协程
	go s.auditDAO.CreateAsync(s.logChan)

	return s
}

// Log 记录审计日志（异步写入，不阻塞主线程）
func (s *AuditService) Log(userID int64, username, action, resource, resourceID, method, path, ip, userAgent, detail string, status int) {
	log := &model.AuditLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Method:     method,
		Path:       path,
		IP:         ip,
		UserAgent:  userAgent,
		Detail:     detail,
		Status:     status,
		CreatedAt:  time.Now(),
	}

	// 非阻塞写入
	select {
	case s.logChan <- log:
	default:
		// channel满时丢弃日志（避免阻塞主流程）
		fmt.Printf("[Audit] 日志通道已满，丢弃日志: %s/%s\n", action, resource)
	}
}

// LogSync 同步记录审计日志（用于重要操作）
func (s *AuditService) LogSync(log *model.AuditLog) error {
	return s.auditDAO.Create(log)
}

// Query 查询审计日志
func (s *AuditService) Query(username, action, method string, startTime, endTime *time.Time, page, pageSize int) ([]model.AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.auditDAO.Query(username, action, method, startTime, endTime, page, pageSize)
}

// GetActionTypes 获取所有操作类型
func (s *AuditService) GetActionTypes() ([]string, error) {
	return s.auditDAO.GetActionTypes()
}

// CleanOldLogs 清理过期日志
func (s *AuditService) CleanOldLogs(retentionDays int) (int64, error) {
	return s.auditDAO.CleanOldLogs(retentionDays)
}

// Close 关闭服务
func (s *AuditService) Close() {
	close(s.logChan)
}