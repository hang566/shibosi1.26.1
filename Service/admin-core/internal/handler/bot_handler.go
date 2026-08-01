package handler

import (
	"admin-core/internal/model"
	"admin-core/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

// BotHandler 算法机器人API处理器
type BotHandler struct {
	BotSvc *service.BotService
}

func NewBotHandler(botSvc *service.BotService) *BotHandler {
	return &BotHandler{BotSvc: botSvc}
}

// ===== Bot CRUD =====

// ListBots 获取机器人列表
func (h *BotHandler) ListBots(c *gin.Context) {
	list, err := h.BotSvc.List()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, list)
}

// GetBot 获取单个机器人
func (h *BotHandler) GetBot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	bot, err := h.BotSvc.Get(id)
	if err != nil {
		model.Fail(c, 404, err.Error())
		return
	}
	model.Success(c, bot)
}

// CreateBot 创建机器人
func (h *BotHandler) CreateBot(c *gin.Context) {
	var bot model.Bot
	if err := c.ShouldBindJSON(&bot); err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	if err := h.BotSvc.Create(&bot); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, bot)
}

// UpdateBot 更新机器人
func (h *BotHandler) UpdateBot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var bot model.Bot
	if err := c.ShouldBindJSON(&bot); err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	bot.ID = id
	if err := h.BotSvc.Update(&bot); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, bot)
}

// DeleteBot 删除机器人
func (h *BotHandler) DeleteBot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.BotSvc.Delete(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}

// ===== Bot Control =====

// StartBot 启动机器人
func (h *BotHandler) StartBot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.BotSvc.Start(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"message": "机器人已启动"})
}

// StopBot 停止机器人
func (h *BotHandler) StopBot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.BotSvc.Stop(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"message": "机器人已停止"})
}

// TriggerBot 手动触发机器人
func (h *BotHandler) TriggerBot(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.BotSvc.Trigger(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"message": "机器人已触发"})
}

// StartAllBots 启动所有机器人
func (h *BotHandler) StartAllBots(c *gin.Context) {
	h.BotSvc.StartAll()
	model.Success(c, gin.H{"message": "已启动所有机器人"})
}

// StopAllBots 停止所有机器人
func (h *BotHandler) StopAllBots(c *gin.Context) {
	h.BotSvc.StopAll()
	model.Success(c, gin.H{"message": "已停止所有机器人"})
}

// AutoConfig 自动配置优化
func (h *BotHandler) AutoConfig(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	c.ShouldBindJSON(&req)
	if req.Mode == "" {
		req.Mode = "full"
	}
	result := h.BotSvc.AutoConfig(req.Mode)
	model.Success(c, result)
}

// ===== Bot Stats =====

// GetBotStats 获取机器人统计
func (h *BotHandler) GetBotStats(c *gin.Context) {
	stats, err := h.BotSvc.GetStats()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, stats)
}

// ===== Bot Logs =====

// ListBotLogs 获取机器人日志
func (h *BotHandler) ListBotLogs(c *gin.Context) {
	botID, _ := strconv.ParseInt(c.Query("bot_id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	logs, err := h.BotSvc.ListLogs(botID, limit)
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, logs)
}

// CleanBotLogs 清理机器人日志
func (h *BotHandler) CleanBotLogs(c *gin.Context) {
	var req struct {
		BotID int64  `json:"bot_id"`
		Days  int    `json:"days"`
	}
	c.ShouldBindJSON(&req)
	if req.Days <= 0 {
		req.Days = 30
	}
	if err := h.BotSvc.CleanLogs(req.BotID, req.Days); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, gin.H{"message": "日志已清理"})
}

// ===== Bot Configs =====

// ListBotConfigs 获取配置列表
func (h *BotHandler) ListBotConfigs(c *gin.Context) {
	configs, err := h.BotSvc.ListConfigs()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, configs)
}

// SaveBotConfig 保存配置
func (h *BotHandler) SaveBotConfig(c *gin.Context) {
	var cfg model.BotConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		model.Fail(c, 400, err.Error())
		return
	}
	if err := h.BotSvc.SaveConfig(&cfg); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, cfg)
}

// DeleteBotConfig 删除配置
func (h *BotHandler) DeleteBotConfig(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.BotSvc.DeleteConfig(id); err != nil {
		model.Fail(c, 500, err.Error())
		return
	}
	model.Success(c, nil)
}
