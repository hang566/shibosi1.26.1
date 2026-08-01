package handler

import (
	"admin-core/internal/model"
	"admin-core/internal/service"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
)

// FavoriteHandler 收藏与历史接口处理器
type FavoriteHandler struct {
	favoriteService *service.FavoriteService
}

// NewFavoriteHandler 创建FavoriteHandler
func NewFavoriteHandler(favoriteService *service.FavoriteService) *FavoriteHandler {
	return &FavoriteHandler{
		favoriteService: favoriteService,
	}
}

// ============ 收藏接口 ============

// CreateFavorite 创建收藏
// POST /api/v1/user/favorites
func (h *FavoriteHandler) CreateFavorite(c *gin.Context) {
	userID := getUserID(c)

	var req model.CreateFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	fav, err := h.favoriteService.CreateFavorite(userID, &req)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	model.Success(c, gin.H{
		"success": true,
		"item":    fav,
	})
}

// GetFavorite 获取单个收藏
// GET /api/v1/user/favorites/:id
func (h *FavoriteHandler) GetFavorite(c *gin.Context) {
	userID := getUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的收藏ID")
		return
	}

	fav, err := h.favoriteService.GetFavorite(id, userID)
	if err != nil {
		model.Fail(c, model.CodeNotFound, err.Error())
		return
	}

	model.Success(c, fav)
}

// ListFavorites 获取收藏列表
// GET /api/v1/user/favorites
func (h *FavoriteHandler) ListFavorites(c *gin.Context) {
	userID := getUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	favType := c.Query("type")
	keyword := c.Query("keyword")

	favs, total, err := h.favoriteService.ListFavorites(userID, page, pageSize, favType, keyword)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.SuccessPage(c, favs, total, page, pageSize)
}

// UpdateFavorite 更新收藏
// PUT /api/v1/user/favorites/:id
func (h *FavoriteHandler) UpdateFavorite(c *gin.Context) {
	userID := getUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的收藏ID")
		return
	}

	var req model.UpdateFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.favoriteService.UpdateFavorite(id, userID, &req); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true, "message": "更新成功"})
}

// DeleteFavorite 删除收藏
// DELETE /api/v1/user/favorites/:id
func (h *FavoriteHandler) DeleteFavorite(c *gin.Context) {
	userID := getUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的收藏ID")
		return
	}

	if err := h.favoriteService.DeleteFavorite(id, userID); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true, "message": "删除成功"})
}

// DeleteFavorites 批量删除收藏
// POST /api/v1/user/favorites/batch-delete
func (h *FavoriteHandler) DeleteFavorites(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		IDs []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.favoriteService.DeleteFavorites(req.IDs, userID); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true, "message": "批量删除成功"})
}

// GetFavoriteCount 获取收藏数量
// GET /api/v1/user/favorites/count
func (h *FavoriteHandler) GetFavoriteCount(c *gin.Context) {
	userID := getUserID(c)

	count, err := h.favoriteService.GetFavoriteCount(userID)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{"count": count})
}

// CheckFavorite 检查是否已收藏
// GET /api/v1/user/favorites/check
func (h *FavoriteHandler) CheckFavorite(c *gin.Context) {
	userID := getUserID(c)
	url := c.Query("url")

	exists, favID, err := h.favoriteService.CheckFavorite(userID, url)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{
		"exists": exists,
		"fav_id": favID,
	})
}

// ============ 历史记录接口 ============

// CreateHistory 创建历史记录
// POST /api/v1/user/history
func (h *FavoriteHandler) CreateHistory(c *gin.Context) {
	userID := getUserID(c)

	var req model.CreateHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	history, err := h.favoriteService.CreateHistory(userID, &req)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{
		"success": true,
		"item":    history,
	})
}

// GetHistory 获取单个历史记录
// GET /api/v1/user/history/:id
func (h *FavoriteHandler) GetHistory(c *gin.Context) {
	userID := getUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的记录ID")
		return
	}

	history, err := h.favoriteService.GetHistory(id, userID)
	if err != nil {
		model.Fail(c, model.CodeNotFound, err.Error())
		return
	}

	model.Success(c, history)
}

// ListHistories 获取历史记录列表
// GET /api/v1/user/history
func (h *FavoriteHandler) ListHistories(c *gin.Context) {
	userID := getUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	historyType := c.Query("type")
	keyword := c.Query("keyword")

	histories, total, err := h.favoriteService.ListHistories(userID, page, pageSize, historyType, keyword)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.SuccessPage(c, histories, total, page, pageSize)
}

// DeleteHistory 删除历史记录
// DELETE /api/v1/user/history/:id
func (h *FavoriteHandler) DeleteHistory(c *gin.Context) {
	userID := getUserID(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		model.Fail(c, model.CodeBadRequest, "无效的记录ID")
		return
	}

	if err := h.favoriteService.DeleteHistory(id, userID); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true, "message": "删除成功"})
}

// DeleteHistories 批量删除历史记录
// POST /api/v1/user/history/batch-delete
func (h *FavoriteHandler) DeleteHistories(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		IDs []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.favoriteService.DeleteHistories(req.IDs, userID); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true, "message": "批量删除成功"})
}

// ClearHistory 清空历史记录
// DELETE /api/v1/user/history
func (h *FavoriteHandler) ClearHistory(c *gin.Context) {
	userID := getUserID(c)

	if err := h.favoriteService.ClearHistory(userID); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true, "message": "清空成功"})
}

// GetHistoryCount 获取历史记录数量
// GET /api/v1/user/history/count
func (h *FavoriteHandler) GetHistoryCount(c *gin.Context) {
	userID := getUserID(c)

	count, err := h.favoriteService.GetHistoryCount(userID)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{"count": count})
}

// ============ 通用用户数据接口 ============

// UserDataHandler 用户通用数据接口
type UserDataHandler struct {
	userDataService *service.UserDataService
}

// NewUserDataHandler 创建 UserDataHandler
func NewUserDataHandler(userDataService *service.UserDataService) *UserDataHandler {
	return &UserDataHandler{userDataService: userDataService}
}

// GetData 获取单个数据
// GET /api/v1/user/data/:key
func (h *UserDataHandler) GetData(c *gin.Context) {
	userID := getUserID(c)
	key := c.Param("key")
	if key == "" {
		model.Fail(c, model.CodeBadRequest, "key 不能为空")
		return
	}

	value, err := h.userDataService.Get(userID, key)
	if err != nil {
		model.Fail(c, model.CodeNotFound, err.Error())
		return
	}

	model.Success(c, gin.H{"key": key, "value": value})
}

// ListData 列出所有数据
// GET /api/v1/user/data
func (h *UserDataHandler) ListData(c *gin.Context) {
	userID := getUserID(c)
	data, err := h.userDataService.List(userID)
	if err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	result := make(map[string]string)
	for _, d := range data {
		result[d.Key] = d.Value
	}

	model.Success(c, result)
}

// SetData 设置单个数据
// POST /api/v1/user/data
func (h *UserDataHandler) SetData(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		Key   string `json:"key" binding:"required,max=64"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userDataService.Set(userID, req.Key, req.Value); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true})
}

// BatchSetData 批量设置数据
// POST /api/v1/user/data/batch
func (h *UserDataHandler) BatchSetData(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		Data map[string]string `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userDataService.BatchSet(userID, req.Data); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true})
}

// DeleteData 删除单个数据
// DELETE /api/v1/user/data/:key
func (h *UserDataHandler) DeleteData(c *gin.Context) {
	userID := getUserID(c)
	key := c.Param("key")

	if err := h.userDataService.Delete(userID, key); err != nil {
		model.Fail(c, model.CodeBadRequest, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true})
}

// ClearData 清空所有数据
// DELETE /api/v1/user/data
func (h *UserDataHandler) ClearData(c *gin.Context) {
	userID := getUserID(c)

	if err := h.userDataService.DeleteAll(userID); err != nil {
		model.Fail(c, model.CodeInternal, err.Error())
		return
	}

	model.Success(c, gin.H{"success": true})
}

// GetPoints 获取用户积分数据
// GET /api/v1/user/points
func (h *UserDataHandler) GetPoints(c *gin.Context) {
	userID := getUserID(c)

	pointsStr, err := h.userDataService.Get(userID, "points")
	totalPoints := 0
	if err == nil {
		totalPoints, _ = strconv.Atoi(pointsStr)
	}

	logsStr, err := h.userDataService.Get(userID, "points_history")
	var logs []interface{}
	if err == nil && logsStr != "" {
		_ = json.Unmarshal([]byte(logsStr), &logs)
	}

	model.Success(c, gin.H{
		"total_points": totalPoints,
		"logs":         logs,
	})
}

// ImportPoints 导入积分数据到云端
// POST /api/v1/user/points/import
func (h *UserDataHandler) ImportPoints(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		TotalPoints int           `json:"total_points"`
		Logs        []interface{} `json:"logs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		model.Fail(c, model.CodeBadRequest, "参数校验失败: "+err.Error())
		return
	}

	// 获取云端现有积分
	existingPointsStr, _ := h.userDataService.Get(userID, "points")
	existingPoints, _ := strconv.Atoi(existingPointsStr)

	// 取较大值
	newBalance := req.TotalPoints
	balanceAdded := 0
	if newBalance < existingPoints {
		newBalance = existingPoints
	} else {
		balanceAdded = newBalance - existingPoints
	}

	// 读取云端现有历史
	existingLogsStr, _ := h.userDataService.Get(userID, "points_history")
	var existingLogs []interface{}
	if existingLogsStr != "" {
		_ = json.Unmarshal([]byte(existingLogsStr), &existingLogs)
	}

	// 合并去重
	mergedLogs := mergePointLogs(existingLogs, req.Logs)
	logsInserted := len(mergedLogs) - len(existingLogs)
	if logsInserted < 0 {
		logsInserted = 0
	}

	// 保存
	_ = h.userDataService.Set(userID, "points", strconv.Itoa(newBalance))
	logsJSON, _ := json.Marshal(mergedLogs)
	_ = h.userDataService.Set(userID, "points_history", string(logsJSON))

	model.Success(c, gin.H{
		"success":       true,
		"balance_added": balanceAdded,
		"logs_inserted": logsInserted,
		"total_points":  newBalance,
	})
}

// mergePointLogs 合并积分日志，按时间戳+操作+积分去重
func mergePointLogs(existing, incoming []interface{}) []interface{} {
	seen := make(map[string]bool)
	var result []interface{}

	// 先加入已有日志
	for _, item := range existing {
		if m, ok := item.(map[string]interface{}); ok {
			key := buildLogKey(m)
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		}
	}

	// 再加入新日志（去重）
	for _, item := range incoming {
		if m, ok := item.(map[string]interface{}); ok {
			key := buildLogKey(m)
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		}
	}

	return result
}

func buildLogKey(m map[string]interface{}) string {
	ts := ""
	if t, ok := m["created_at"]; ok {
		ts = toString(t)
	}
	action := ""
	if a, ok := m["action"]; ok {
		action = toString(a)
	}
	points := ""
	if p, ok := m["points"]; ok {
		points = toString(p)
	}
	return ts + "|" + action + "|" + points
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return ""
	}
}

// getUserID 从JWT上下文中获取用户ID
func getUserID(c *gin.Context) int64 {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	switch v := userID.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		id, _ := strconv.ParseInt(v, 10, 64)
		return id
	default:
		return 0
	}
}
