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
				"name":   s.Name(),
				"weight": engineWeights[s.Name()],
			})
		}
	}

	customEngines, _ := GetCustomEngines(false)

	writeJSON(w, map[string]interface{}{
		"builtin_engines": engines,
		"custom_engines":  customEngines,
	})
}
