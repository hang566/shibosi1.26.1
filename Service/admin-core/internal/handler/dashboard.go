package handler

import (
	"admin-core/internal/dao"
	"admin-core/internal/model"
	"time"

	"github.com/gin-gonic/gin"
)

// DashboardHandler 仪表盘接口处理器
type DashboardHandler struct {
	userDAO   *dao.UserDAO
	configDAO *dao.ConfigDAO
	auditDAO  *dao.AuditDAO
}

// NewDashboardHandler 创建DashboardHandler
func NewDashboardHandler(userDAO *dao.UserDAO, configDAO *dao.ConfigDAO, auditDAO *dao.AuditDAO) *DashboardHandler {
	return &DashboardHandler{
		userDAO:   userDAO,
		configDAO: configDAO,
		auditDAO:  auditDAO,
	}
}

// GetDashboard 获取仪表盘数据
// GET /api/v1/admin/dashboard
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	stats, _ := h.userDAO.GetStats(c.Request.Context())

	todayNew, _ := h.userDAO.CountTodayNew()
	loginCount, _ := h.auditDAO.CountByAction("login")
	regCount, _ := h.auditDAO.CountByAction("register")

	model.Success(c, gin.H{
		"total_users":    stats["total"],
		"active_users":   stats["active"],
		"disabled_users": stats["disabled"],
		"today_new":      todayNew,
		"total_logins":   loginCount,
		"total_regs":     regCount,
		"server_time":    time.Now().Format("2006-01-02 15:04:05"),
		"uptime":         "运行中",
	})
}

// GetSystemStatus 获取系统状态
// GET /api/v1/admin/system-status
func (h *DashboardHandler) GetSystemStatus(c *gin.Context) {
	// 模拟系统状态数据（实际应通过指标采集获取）
	model.Success(c, gin.H{
		"cpu_usage":     "25%",
		"memory_usage":  "512MB / 2GB",
		"disk_usage":    "10GB / 50GB",
		"goroutines":    42,
		"db_connections": 8,
		"uptime":        "3天 12小时",
		"version":       "2.0.0",
	})
}