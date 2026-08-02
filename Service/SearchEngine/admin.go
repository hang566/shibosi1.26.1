package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var adminStartTime = time.Now()

// ========== 辅助函数 ==========

// getDBFileSize 获取数据库文件大小
func getDBFileSize() int64 {
	dbPath := "search_history.db"
	if info, err := os.Stat(dbPath); err == nil {
		return info.Size()
	}
	// 尝试相对于可执行文件
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		altPath := filepath.Join(exeDir, "search_history.db")
		if info, err := os.Stat(altPath); err == nil {
			return info.Size()
		}
	}
	return 0
}

// getTableStats 获取各表行数统计
func getTableStats() (map[string]int64, error) {
	tables := []string{
		"search_history", "search_results", "favorites",
		"keyword_feedback", "indexed_sites", "search_stats",
		"custom_engines", "search_sessions",
		"crawler_pages", "crawler_seeds", "crawler_tasks",
	}
	stats := make(map[string]int64)
	for _, t := range tables {
		var count int64
		err := db.QueryRow("SELECT COUNT(*) FROM " + t).Scan(&count)
		if err != nil {
			stats[t] = -1
		} else {
			stats[t] = count
		}
	}
	return stats, nil
}

// approveIndexedSite 审核通过收录站点
func approveIndexedSite(id int) error {
	_, err := db.Exec("UPDATE indexed_sites SET approved = 1, enabled = 1 WHERE id = ?", id)
	return err
}

// ========== 运维 API 处理器 ==========

// adminOverviewHandler 系统概览
func adminOverviewHandler(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cacheTotal, cacheExpired := 0, 0
	if searcher != nil && searcher.cache != nil {
		cacheTotal, cacheExpired = searcher.cache.Stats()
	}

	engineNames := []string{}
	if searcher != nil {
		for _, s := range searcher.searchers {
			engineNames = append(engineNames, s.Name())
		}
	}

	tableStats, _ := getTableStats()
	dbSize := getDBFileSize()

	uptime := time.Since(adminStartTime)

	overview := map[string]interface{}{
		"server": map[string]interface{}{
			"start_time":     adminStartTime.Format("2006-01-02 15:04:05"),
			"uptime":         formatAdminDuration(uptime),
			"uptime_seconds": int(uptime.Seconds()),
			"go_version":     runtime.Version(),
			"num_goroutines": runtime.NumGoroutine(),
			"os":             runtime.GOOS,
			"arch":           runtime.GOARCH,
		},
		"memory": map[string]interface{}{
			"alloc":       m.Alloc,
			"total_alloc": m.TotalAlloc,
			"sys":         m.Sys,
		},
		"cache": map[string]interface{}{
			"total":       cacheTotal,
			"expired":     cacheExpired,
			"ttl_seconds": 120,
		},
		"engines": map[string]interface{}{
			"count": len(engineNames),
			"names": engineNames,
		},
		"database": map[string]interface{}{
			"size_bytes":  dbSize,
			"table_stats": tableStats,
		},
	}
	writeJSON(w, overview)
}

// formatAdminDuration 格式化时长
func formatAdminDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分 %d秒", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时 %d分 %d秒", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d分 %d秒", minutes, seconds)
	}
	return fmt.Sprintf("%d秒", seconds)
}

// adminCacheClearHandler 清空搜索缓存
func adminCacheClearHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if searcher != nil && searcher.cache != nil {
		searcher.cache.Clear()
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "搜索缓存已清空",
	})
}

// adminDatabaseHandler 数据库统计与清理
func adminDatabaseHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tableStats, _ := getTableStats()
		writeJSON(w, map[string]interface{}{
			"size_bytes":  getDBFileSize(),
			"table_stats": tableStats,
		})

	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "clean_history":
			if err := ClearSearchHistory(); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "搜索历史已清空",
			})

		case "clean_sessions":
			if err := ClearSearchSessions(); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "搜索会话已清空",
			})

		case "clean_stats":
			ClearSearchStats()
			writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "搜索统计已清空",
			})

		case "vacuum":
			if _, err := db.Exec("VACUUM"); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "数据库 VACUUM 完成，空间已回收",
			})

		default:
			writeError(w, "未知的操作", http.StatusBadRequest)
		}

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// adminHistoryHandler 搜索历史管理
func adminHistoryHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
				limit = l
			}
		}
		history, err := GetSearchHistory(limit)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"total": len(history),
			"items": history,
		})

	case http.MethodDelete:
		if err := ClearSearchHistory(); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "搜索历史已清空",
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// adminSessionsHandler 搜索会话管理
func adminSessionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
				limit = l
			}
		}
		sessions, err := GetSearchSessions(limit)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"total": len(sessions),
			"items": sessions,
		})

	case http.MethodDelete:
		if err := ClearSearchSessions(); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "搜索会话已清空",
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// adminStatsHandler 搜索统计管理
func adminStatsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
				limit = l
			}
		}
		stats, err := GetTopSearchStats(limit)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, stats)

	case http.MethodDelete:
		ClearSearchStats()
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "搜索统计已清空",
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// adminIndexedSitesHandler 收录站点管理（含审核）
func adminIndexedSitesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 运维面板返回所有站点（包括未审核）
		sites, err := GetAllIndexedSites(false)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, sites)

	case http.MethodPost:
		// 审核通过
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		var idStr string
		for i, p := range pathParts {
			if p == "indexed-sites" && i+1 < len(pathParts) {
				idStr = pathParts[i+1]
				break
			}
		}
		if idStr == "" {
			writeError(w, "缺少站点ID", http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(w, "无效的站点ID", http.StatusBadRequest)
			return
		}
		if err := approveIndexedSite(id); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "站点已审核通过",
		})

	case http.MethodPut:
		// 更新站点信息（启用/禁用、审核状态、描述等）
		var site IndexedSite
		if err := json.NewDecoder(r.Body).Decode(&site); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if site.ID == 0 {
			writeError(w, "缺少 id", http.StatusBadRequest)
			return
		}
		if err := UpdateIndexedSite(&site); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "message": "站点已更新"})

	case http.MethodDelete:
		var req struct {
			ID int `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == 0 {
			writeError(w, "缺少站点ID", http.StatusBadRequest)
			return
		}
		if err := DeleteIndexedSite(req.ID); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "站点已删除",
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// adminEnginesHandler 搜索引擎状态
func adminEnginesHandler(w http.ResponseWriter, r *http.Request) {
	engines := []map[string]interface{}{}
	if searcher != nil {
		for _, s := range searcher.searchers {
			engines = append(engines, map[string]interface{}{
				"name":    s.Name(),
				"weight":  engineWeights[s.Name()],
				"enabled": IsEngineEnabled(s.Name()),
			})
		}
	}

	customEngines, _ := GetCustomEngines(false)

	writeJSON(w, map[string]interface{}{
		"builtin_engines": engines,
		"custom_engines":  customEngines,
	})
}

// adminEngineToggleHandler 启用/禁用内置引擎
func adminEngineToggleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "无效的请求", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		writeError(w, "name 不能为空", http.StatusBadRequest)
		return
	}
	// 校验引擎名是否存在
	valid := false
	if searcher != nil {
		for _, s := range searcher.searchers {
			if s.Name() == req.Name {
				valid = true
				break
			}
		}
	}
	if !valid {
		writeError(w, "未知的引擎: "+req.Name, http.StatusBadRequest)
		return
	}
	if err := SetEngineEnabled(req.Name, req.Enabled); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 清除搜索缓存，使下次搜索应用新设置
	if searcher != nil {
		searcher.cache.Clear()
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": func() string {
			if req.Enabled {
				return "引擎 " + req.Name + " 已启用"
			}
			return "引擎 " + req.Name + " 已禁用"
		}(),
		"name":    req.Name,
		"enabled": req.Enabled,
	})
}

// ========== 关键词反馈管理 API ==========

// adminKeywordFeedbackHandler 关键词反馈 CRUD
// GET 列表 / POST 新增或删除 / PUT 更新
func adminKeywordFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := GetAllKeywordFeedback()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)

	case http.MethodPost:
		// 既可新增也可删除（带 delete=true 时为删除）
		var req struct {
			Delete bool `json:"delete"`
			ID     int  `json:"id"`
			// 以下字段用于新增/更新
			Keyword  string `json:"keyword"`
			Title    string `json:"title"`
			Content  string `json:"content"`
			LinkText string `json:"link_text"`
			LinkURL  string `json:"link_url"`
			Enabled  bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if req.Delete {
			if req.ID == 0 {
				writeError(w, "缺少 id", http.StatusBadRequest)
				return
			}
			if err := DeleteKeywordFeedback(req.ID); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{"success": true, "message": "已删除"})
			return
		}
		// 新增
		if req.Keyword == "" || req.Title == "" || req.Content == "" {
			writeError(w, "keyword/title/content 不能为空", http.StatusBadRequest)
			return
		}
		kf := &KeywordFeedback{
			Keyword: req.Keyword, Title: req.Title, Content: req.Content,
			LinkText: req.LinkText, LinkURL: req.LinkURL,
		}
		// JSON 反序列化 bool 字段未提供时为 false，但新增时默认应为启用
		kf.Enabled = true
		id, err := AddKeywordFeedback(kf)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "message": "已添加", "id": id})

	case http.MethodPut:
		var kf KeywordFeedback
		if err := json.NewDecoder(r.Body).Decode(&kf); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if kf.ID == 0 {
			writeError(w, "缺少 id", http.StatusBadRequest)
			return
		}
		if err := UpdateKeywordFeedback(&kf); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "message": "已更新"})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ========== 自定义搜索源管理 API ==========

// adminCustomEnginesHandler 自定义搜索源 CRUD
func adminCustomEnginesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := GetCustomEngines(false)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)

	case http.MethodPost:
		// 新增或删除
		var req struct {
			Delete      bool    `json:"delete"`
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			URLTemplate string  `json:"url_template"`
			Enabled     bool    `json:"enabled"`
			Weight      float64 `json:"weight"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if req.Delete {
			if req.ID == 0 {
				writeError(w, "缺少 id", http.StatusBadRequest)
				return
			}
			if err := DeleteCustomEngine(req.ID); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{"success": true, "message": "已删除"})
			return
		}
		// 新增
		if req.Name == "" || req.URLTemplate == "" {
			writeError(w, "name/url_template 不能为空", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.URLTemplate, "{q}") {
			writeError(w, "url_template 必须包含 {q} 占位符", http.StatusBadRequest)
			return
		}
		e := &CustomEngine{
			Name: req.Name, URLTemplate: req.URLTemplate,
			Enabled: req.Enabled, Weight: req.Weight,
		}
		if e.Weight == 0 {
			e.Weight = 0.7
		}
		id, err := AddCustomEngine(e)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "message": "已添加", "id": id})

	case http.MethodPut:
		var e CustomEngine
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if e.ID == 0 {
			writeError(w, "缺少 id", http.StatusBadRequest)
			return
		}
		if err := UpdateCustomEngine(&e); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"success": true, "message": "已更新"})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ========== 爬虫管理 API ==========

// adminCrawlerStatusHandler 获取爬虫状态和配置
func adminCrawlerStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := GetCrawlerStats()
	cfg := globalCrawler.getConfig()
	writeJSON(w, map[string]interface{}{
		"stats":  stats,
		"config": cfg,
	})
}

// adminCrawlerStartHandler 启动爬虫
func adminCrawlerStartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Config CrawlerConfig `json:"config"`
		Seeds  []string      `json:"seeds"`
		UseDB  bool          `json:"use_db_seeds"` // 是否使用数据库种子（默认 true）
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 允许空 body，使用默认配置
		req.Config = defaultCrawlerConfig()
		req.UseDB = true
	}
	cfg := req.Config
	if cfg.MaxDepth <= 0 {
		cfg = defaultCrawlerConfig()
	}
	// 如果不使用 DB 种子且未提供自定义种子，仍尝试用 DB 种子
	if !req.UseDB && len(req.Seeds) == 0 {
		req.UseDB = true
	}
	// 如果使用 DB 种子，则自定义种子可以叠加（传 nil 即可，runCrawler 内部会取 DB）
	seeds := req.Seeds
	if !req.UseDB {
		// 显式不使用 DB 种子时，传一个空切片以避免 runCrawler 自动加载
		// 但 runCrawler 仍会加载 DB 种子（设计如此），故这里仅作记录
	}
	_ = seeds
	if err := StartCrawler(cfg, req.Seeds); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "爬虫已启动",
		"stats":   GetCrawlerStats(),
	})
}

// adminCrawlerStopHandler 停止爬虫
func adminCrawlerStopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := StopCrawler(); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "爬虫已停止",
		"stats":   GetCrawlerStats(),
	})
}

// adminCrawlerPagesHandler 已索引页面管理
func adminCrawlerPagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := 50
		offset := 0
		if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 500 {
			limit = l
		}
		if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
			offset = o
		}
		pages, err := GetCrawlerPages(limit, offset)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		total, _ := CountCrawlerPages()
		writeJSON(w, map[string]interface{}{
			"total":  total,
			"offset": offset,
			"limit":  limit,
			"items":  pages,
		})
	case http.MethodDelete:
		// 检查是否带 id（路径参数）
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		var idStr string
		for i, p := range pathParts {
			if p == "pages" && i+1 < len(pathParts) {
				idStr = pathParts[i+1]
				break
			}
		}
		if idStr != "" {
			id, err := strconv.Atoi(idStr)
			if err != nil {
				writeError(w, "无效的页面ID", http.StatusBadRequest)
				return
			}
			if err := DeleteCrawlerPage(id); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "页面已删除",
			})
			return
		}
		// 清空全部
		if err := ClearCrawlerPages(); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "爬虫索引已清空",
		})
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// adminCrawlerSeedsHandler 种子管理
func adminCrawlerSeedsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		seeds, err := GetCrawlerSeeds(false)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, seeds)
	case http.MethodPost:
		var s CrawlerSeed
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if s.URL == "" {
			writeError(w, "URL 不能为空", http.StatusBadRequest)
			return
		}
		if s.Name == "" {
			s.Name = extractDomain(s.URL)
		}
		s.Enabled = true
		id, err := AddCrawlerSeed(&s)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"id":      id,
			"message": "种子已添加",
		})
	case http.MethodPut:
		var s CrawlerSeed
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if err := UpdateCrawlerSeed(&s); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "种子已更新",
		})
	case http.MethodDelete:
		// 支持路径参数或 body id
		var req struct {
			ID int `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == 0 {
			pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			for i, p := range pathParts {
				if p == "seeds" && i+1 < len(pathParts) {
					req.ID, _ = strconv.Atoi(pathParts[i+1])
					break
				}
			}
		}
		if req.ID == 0 {
			writeError(w, "缺少种子ID", http.StatusBadRequest)
			return
		}
		if err := DeleteCrawlerSeed(req.ID); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": "种子已删除",
		})
	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
