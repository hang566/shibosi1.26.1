package handler

import (
	"admin-core/internal/dao"
	"admin-core/internal/model"

	"github.com/gin-gonic/gin"
)

// ConfigHandler 系统配置接口处理器
type ConfigHandler struct {
	configDAO *dao.ConfigDAO
}

// NewConfigHandler 创建ConfigHandler
func NewConfigHandler(configDAO *dao.ConfigDAO) *ConfigHandler {
	return &ConfigHandler{configDAO: configDAO}
}

// GetConfigs 获取所有配置
// GET /api/v1/admin/configs
func (h *ConfigHandler) GetConfigs(c *gin.Context) {
	configs, err := h.configDAO.List()
	if err != nil {
		model.Fail(c, model.CodeInternal, "获取配置失败")
		return
	}
	model.Success(c, configs)
}

// GetConfig 获取单个配置
// GET /api/v1/admin/configs/:key
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	key := c.Param("key")
	config, err := h.configDAO.Get(key)
	if err != nil {
		model.Fail(c, model.CodeNotFound, "配置不存在")
		return
	}
	model.Success(c, config)
}

// UpdateConfig 更新配置（支持热更新）
// PUT /api/v1/admin/configs/:key
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	username, _ := c.Get("username")
	if err := h.configDAO.Set(key, req.Value, username.(string)); err != nil {
		model.Fail(c, model.CodeInternal, "更新配置失败")
		return
	}

	model.Success(c, gin.H{"message": "配置已更新（热更新生效）"})
}

// BatchUpdateConfigs 批量更新配置
// PUT /api/v1/admin/configs
func (h *ConfigHandler) BatchUpdateConfigs(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	username, _ := c.Get("username")
	if err := h.configDAO.BatchSet(req, username.(string)); err != nil {
		model.Fail(c, model.CodeInternal, "批量更新配置失败")
		return
	}

	model.Success(c, gin.H{"message": "配置已批量更新"})
}