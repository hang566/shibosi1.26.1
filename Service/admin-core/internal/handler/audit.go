package handler

import (
	"admin-core/internal/model"
	"admin-core/internal/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditHandler 审计日志接口处理器
type AuditHandler struct {
	auditService *service.AuditService
}

// NewAuditHandler 创建AuditHandler
func NewAuditHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// QueryLogs 查询审计日志
// GET /api/v1/admin/audit-logs
func (h *AuditHandler) QueryLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	username := c.Query("username")
	action := c.Query("action")
	method := c.Query("method")

	var startTime, endTime *time.Time
	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse("2006-01-02 15:04:05", st)
		if err == nil {
			startTime = &t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse("2006-01-02 15:04:05", et)
		if err == nil {
			endTime = &t
		}
	}

	logs, total, err := h.auditService.Query(username, action, method, startTime, endTime, page, pageSize)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.SuccessPage(c, logs, total, page, pageSize)
}

// GetActionTypes 获取操作类型列表
// GET /api/v1/admin/audit-logs/action-types
func (h *AuditHandler) GetActionTypes(c *gin.Context) {
	actions, err := h.auditService.GetActionTypes()
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}
	model.Success(c, actions)
}

// CleanLogs 清理过期日志
// DELETE /api/v1/admin/audit-logs/clean
func (h *AuditHandler) CleanLogs(c *gin.Context) {
	var req struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.RetentionDays = 90 // 默认保留90天
	}

	count, err := h.auditService.CleanOldLogs(req.RetentionDays)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{
		"deleted_count": count,
		"message":       "清理完成",
	})
}