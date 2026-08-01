package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

var startTime = time.Now()

func getSystemInfo() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var dbSize int64
	dbSize, _ = GetDBSize()

	tableStats, _ := GetTableStats()

	newsStats, _ := GetNewsStats()
	analysisStats, _ := GetAnalysisStats()
	analysisTimeStats, _ := GetAnalysisTimeStats()

	logCount, _ := GetLogCount("", "")

	enabledSources, _ := GetNewsSourcesEnabled()

	uptime := time.Since(startTime)
	uptimeStr := formatDuration(uptime)

	return map[string]interface{}{
		"server": map[string]interface{}{
			"start_time":     startTime.Format("2006-01-02 15:04:05"),
			"uptime":         uptimeStr,
			"uptime_seconds": int(uptime.Seconds()),
			"go_version":     runtime.Version(),
			"num_goroutines": runtime.NumGoroutine(),
			"os":             runtime.GOOS,
			"arch":           runtime.GOARCH,
		},
		"memory": map[string]interface{}{
			"alloc":          m.Alloc,
			"total_alloc":    m.TotalAlloc,
			"sys":            m.Sys,
			"num_goroutines": runtime.NumGoroutine(),
		},
		"news": map[string]interface{}{
			"total_count":    newsStats["total_count"],
			"source_count":   newsStats["source_count"],
			"today_count":    newsStats["today_count"],
			"category_count": newsStats["category_count"],
		},
		"analysis": map[string]interface{}{
			"analyzed_count":        analysisStats["analyzed_count"],
			"left_count":            analysisStats["left_count"],
			"right_count":           analysisStats["right_count"],
			"neutral_count":         analysisStats["neutral_count"],
			"private_ad_count":      analysisStats["private_ad_count"],
			"avg_confidence":        analysisTimeStats["avg_confidence"],
			"high_confidence_count": analysisTimeStats["high_confidence_count"],
			"ad_detected_count":     analysisTimeStats["ad_detected_count"],
		},
		"sources": map[string]interface{}{
			"enabled_count": len(enabledSources),
			"sources":       enabledSources,
		},
		"database": map[string]interface{}{
			"total_size_bytes": dbSize,
			"table_stats":      tableStats,
		},
		"logs": map[string]interface{}{
			"total_count": logCount,
		},
		"log_level": GetLogLevelName(GetLogLevel()),
	}
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分 %d秒", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时 %d分 %d秒", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%d分 %d秒", minutes, seconds)
	}
	return fmt.Sprintf("%d秒", seconds)
}

func adminOverviewHandler(w http.ResponseWriter, r *http.Request) {
	info := getSystemInfo()
	writeJSON(w, info)
}

func adminLogsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		level := r.URL.Query().Get("level")
		module := r.URL.Query().Get("module")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage <= 0 || perPage > 100 {
			perPage = 20
		}
		offset := (page - 1) * perPage

		logs, err := GetLogs(level, module, perPage, offset)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		total, _ := GetLogCount(level, module)
		levels, _ := GetLogLevels()
		modules, _ := GetLogModules()

		writeJSON(w, map[string]interface{}{
			"logs":     logs,
			"total":    total,
			"page":     page,
			"per_page": perPage,
			"levels":   levels,
			"modules":  modules,
		})

	case http.MethodDelete:
		var req struct {
			Days int `json:"days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.Days = 30
		}
		if req.Days <= 0 {
			req.Days = 30
		}

		deleted, err := CleanLogs(req.Days)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if logger != nil {
			logger.Info("admin", "清理了 %d 天前的 %d 条日志", req.Days, deleted)
		}

		writeJSON(w, map[string]interface{}{
			"success": true,
			"deleted": deleted,
			"message": fmt.Sprintf("已清理 %d 天前的 %d 条日志", req.Days, deleted),
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func adminLogLevelHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{
			"level":     GetLogLevelName(GetLogLevel()),
			"level_num": GetLogLevel(),
		})

	case http.MethodPost:
		var req struct {
			Level string `json:"level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}

		levelNum := ParseLogLevelName(req.Level)
		SetLogLevel(levelNum)

		if logger != nil {
			logger.Info("admin", "日志级别已设置为: %s", GetLogLevelName(levelNum))
		}

		writeJSON(w, map[string]interface{}{
			"success":   true,
			"level":     GetLogLevelName(levelNum),
			"level_num": levelNum,
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func adminAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		stats, err := GetAnalysisStats()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		timeStats, err := GetAnalysisTimeStats()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		summary, err := GetAnalysisResultsSummary()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]interface{}{
			"stats":      stats,
			"time_stats": timeStats,
			"summary":    summary,
		})

	case http.MethodPost:
		var req struct {
			Limit int `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.Limit = 500
		}
		if req.Limit <= 0 || req.Limit > 5000 {
			req.Limit = 500
		}

		count, err := ReanalyzeNews(req.Limit)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if logger != nil {
			logger.Info("analysis", "重新分析了 %d 条新闻", count)
		}

		writeJSON(w, map[string]interface{}{
			"success": true,
			"count":   count,
			"message": fmt.Sprintf("成功重新分析 %d 条新闻", count),
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func adminFetchHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		sourceID := r.URL.Query().Get("source_id")

		if sourceID != "" {
			id, err := strconv.Atoi(sourceID)
			if err != nil {
				writeError(w, "无效的源ID", http.StatusBadRequest)
				return
			}
			saved, err := FetchAndSaveSource(id)
			if err != nil {
				if logger != nil {
					logger.Error("fetch", "抓取源 %d 失败: %v", id, err)
				}
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("fetch", "成功抓取源 %d 的 %d 条新闻", id, saved)
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"saved":   saved,
			})
			return
		}

		saved, err := FetchAndSaveAll()
		if err != nil {
			if logger != nil {
				logger.Error("fetch", "抓取所有源失败: %v", err)
			}
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if logger != nil {
			logger.Info("fetch", "成功抓取 %d 条新闻", saved)
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"saved":   saved,
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func adminDatabaseHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tableStats, err := GetTableStats()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		totalSize, _ := GetDBSize()

		writeJSON(w, map[string]interface{}{
			"total_size_bytes": totalSize,
			"table_stats":      tableStats,
		})

	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
			Days   int    `json:"days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "clean_news":
			if req.Days <= 0 {
				req.Days = 30
			}
			deleted, err := ClearOldNews(req.Days)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "清理了 %d 天前的 %d 条新闻", req.Days, deleted)
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"deleted": deleted,
				"message": fmt.Sprintf("已清理 %d 天前的 %d 条新闻", req.Days, deleted),
			})

		case "clean_logs":
			if req.Days <= 0 {
				req.Days = 30
			}
			deleted, err := CleanLogs(req.Days)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "清理了 %d 天前的 %d 条日志", req.Days, deleted)
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"deleted": deleted,
				"message": fmt.Sprintf("已清理 %d 天前的 %d 条日志", req.Days, deleted),
			})

		case "clean_fetch_logs":
			if req.Days <= 0 {
				req.Days = 30
			}
			deleted, err := CleanFetchLogs(req.Days)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "清理了 %d 天前的 %d 条抓取日志", req.Days, deleted)
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"deleted": deleted,
				"message": fmt.Sprintf("已清理 %d 天前的 %d 条抓取日志", req.Days, deleted),
			})

		case "vacuum":
			if err := VACUUMDB(); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "执行数据库 VACUUM 成功")
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "数据库 VACUUM 完成，空间已回收",
			})

		case "clean_all":
			if err := CleanAllData(); err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "清空所有数据并 VACUUM 成功")
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"message": "所有数据已清空，空间已回收",
			})

		case "permanent_all":
			count, err := SetAllPermanent(true)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "批量永久保存了 %d 条新闻", count)
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"updated": count,
				"message": fmt.Sprintf("已将 %d 条新闻标记为永久保存", count),
			})

		case "unpermanent_all":
			count, err := SetAllPermanent(false)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "批量取消永久保存了 %d 条新闻", count)
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"updated": count,
				"message": fmt.Sprintf("已取消 %d 条新闻的永久保存", count),
			})

		case "permanent_old":
			if req.Days <= 0 {
				req.Days = 30
			}
			count, err := PermanentOldNews(req.Days, true)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if logger != nil {
				logger.Info("admin", "将 %d 天前的 %d 条新闻标记为永久保存", req.Days, count)
			}
			writeJSON(w, map[string]interface{}{
				"success": true,
				"updated": count,
				"message": fmt.Sprintf("已将 %d 天前的 %d 条新闻标记为永久保存", req.Days, count),
			})

		default:
			writeError(w, "未知的操作", http.StatusBadRequest)
	}

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func adminSourcesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sources, err := GetSources()
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, sources)

	case http.MethodPost:
		var src NewsSource
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		id, err := AddSource(src)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if logger != nil {
			logger.Info("admin", "添加了新闻源: %s", src.Name)
		}
		writeJSON(w, map[string]interface{}{"success": true, "id": id})

	case http.MethodPut:
		var src NewsSource
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			writeError(w, "无效的请求", http.StatusBadRequest)
			return
		}
		if err := UpdateSource(src); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if logger != nil {
			logger.Info("admin", "更新了新闻源: %s", src.Name)
		}
		writeJSON(w, map[string]interface{}{"success": true})

	case http.MethodDelete:
		var req struct {
			ID int `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == 0 {
			writeError(w, "缺少源ID", http.StatusBadRequest)
			return
		}
		if err := DeleteSource(req.ID); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if logger != nil {
			logger.Info("admin", "删除了新闻源 ID: %d", req.ID)
		}
		writeJSON(w, map[string]interface{}{"success": true})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func adminBackupHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		exportDir := filepath.Join("..", "..", "db", "backups")
		os.MkdirAll(exportDir, 0755)

		timestamp := time.Now().Format("20060102_150405")
		backupPath := filepath.Join(exportDir, fmt.Sprintf("newspaper_backup_%s.db", timestamp))

		dbPath := filepath.Join("..", "..", "db", "newspaper.db")
		data, err := os.ReadFile(dbPath)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if logger != nil {
			logger.Info("admin", "数据库备份成功: %s", backupPath)
		}

		writeJSON(w, map[string]interface{}{
			"success":     true,
			"backup_path": backupPath,
			"size_bytes":  len(data),
			"timestamp":   timestamp,
		})

	default:
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
