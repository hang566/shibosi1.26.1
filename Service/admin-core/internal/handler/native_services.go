package handler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"admin-core/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ===== 原生服务数据访问（直连各服务的 SQLite 数据库，无需服务在线） =====

// openSQLiteDB 使用 GORM 打开 SQLite 数据库
func openSQLiteDB(dbPath string) (*gorm.DB, error) {
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("解析路径失败: %v", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("数据库不存在: %s", absPath)
	}

	db, err := gorm.Open(sqlite.Open(absPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}
	return db, nil
}

// getNewspaperDB 获取 Newspaper 服务的 SQLite 连接
func getNewspaperDB() (*gorm.DB, error) {
	workDir, _ := os.Getwd()
	dbPath := filepath.Join(workDir, "..", "..", "db", "newspaper.db")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dbPath = filepath.Join(workDir, "db", "newspaper.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dbPath = filepath.Join(workDir, "..", "db", "newspaper.db")
		}
	}

	return openSQLiteDB(dbPath)
}

// getAdminDBPath 获取 admin-core 的数据库路径
func getAdminDBPath() string {
	workDir, _ := os.Getwd()
	dbPath := filepath.Join(workDir, "data", "admin.db")
	if _, err := os.Stat(dbPath); err == nil {
		abs, _ := filepath.Abs(dbPath)
		return abs
	}
	return ""
}

// GetNewspaperOverview 获取新闻服务总览数据
func GetNewspaperOverview(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	var totalCount int64
	db.Raw("SELECT COUNT(*) FROM news").Scan(&totalCount)

	var categoryCount int64
	db.Raw("SELECT COUNT(DISTINCT category) FROM news").Scan(&categoryCount)

	var sourceCount int64
	db.Raw("SELECT COUNT(DISTINCT source) FROM news").Scan(&sourceCount)

	today := time.Now().Format("2006-01-02")
	var todayCount int64
	db.Raw("SELECT COUNT(*) FROM news WHERE date(fetched_at) = ?", today).Scan(&todayCount)

	var latestFetch string
	db.Raw("SELECT MAX(fetched_at) FROM news").Scan(&latestFetch)

	var analyzedCount int64
	db.Raw("SELECT COUNT(*) FROM news WHERE political_stance != ''").Scan(&analyzedCount)

	workDir, _ := os.Getwd()
	fileSize := getFileSize(filepath.Join(workDir, "..", "..", "db", "newspaper.db"))

	model.Success(c, gin.H{
		"news": gin.H{
			"total_count":    totalCount,
			"category_count": categoryCount,
			"source_count":   sourceCount,
			"today_count":    todayCount,
			"latest_fetch":   latestFetch,
		},
		"analysis": gin.H{
			"analyzed_count": analyzedCount,
		},
		"server": gin.H{
			"uptime": "N/A (离线模式)",
		},
		"memory": gin.H{
			"alloc": fileSize,
		},
		"database": gin.H{
			"size":   fileSize,
			"tables": 5,
		},
	})
}

// GetNewspaperSources 获取新闻源列表
func GetNewspaperSources(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	type Source struct {
		ID        int
		Name      string
		URL       string
		Type      string
		Category  string
		Enabled   int
		LastFetch *string
	}

	var sources []Source
	db.Raw("SELECT id, name, url, type, category, enabled, last_fetch FROM news_sources ORDER BY id").Scan(&sources)

	var result []gin.H
	for _, s := range sources {
		lastFetch := ""
		if s.LastFetch != nil {
			lastFetch = *s.LastFetch
		}
		result = append(result, gin.H{
			"id":         s.ID,
			"name":       s.Name,
			"url":        s.URL,
			"type":       s.Type,
			"category":   s.Category,
			"enabled":    s.Enabled == 1,
			"last_fetch": lastFetch,
		})
	}
	if result == nil {
		result = []gin.H{}
	}

	model.Success(c, result)
}

// GetNewspaperAnalysis 获取新闻分析数据
func GetNewspaperAnalysis(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	politicalCategories := map[string]string{
		"left":         "左翼",
		"center-left":  "中左翼",
		"neutral":      "中立",
		"center-right": "中右翼",
		"right":        "右翼",
	}

	// 政治倾向统计
	type StanceCount struct {
		Stance string
		Cnt    int
	}
	var stances []StanceCount
	db.Raw("SELECT political_stance as stance, COUNT(*) as cnt FROM news WHERE political_stance != '' GROUP BY political_stance").Scan(&stances)

	categories := map[string]int{}
	for _, s := range stances {
		label := politicalCategories[s.Stance]
		if label == "" {
			label = s.Stance
		}
		categories[label] = s.Cnt
	}

	// 主题分类统计
	type TopicCount struct {
		Topic string
		Cnt   int
	}
	var topics []TopicCount
	db.Raw("SELECT topic_category as topic, COUNT(*) as cnt FROM news WHERE topic_category != '' GROUP BY topic_category").Scan(&topics)

	topicCategories := map[string]int{}
	for _, t := range topics {
		topicCategories[t.Topic] = t.Cnt
	}

	// 可靠性统计
	type RelCount struct {
		Rel string
		Cnt int
	}
	var rels []RelCount
	db.Raw("SELECT reliability as rel, COUNT(*) as cnt FROM news WHERE reliability != '' GROUP BY reliability").Scan(&rels)

	reliabilityStats := map[string]int{}
	for _, r := range rels {
		reliabilityStats[r.Rel] = r.Cnt
	}

	model.Success(c, gin.H{
		"analysis": gin.H{
			"political_stances": categories,
			"topic_categories":  topicCategories,
			"reliability":       reliabilityStats,
			"categories":        categories,
		},
		"categories": categories,
	})
}

// GetNewspaperLogs 获取新闻服务日志
func GetNewspaperLogs(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	type LogEntry struct {
		ID      int
		Level   string
		Module  string
		Message string
		Detail  *string
		LogTime *string
	}
	var logs []LogEntry
	db.Raw("SELECT id, level, module, message, details, log_time FROM app_logs ORDER BY id DESC LIMIT 200").Scan(&logs)

	var result []gin.H
	for _, l := range logs {
		detail := ""
		if l.Detail != nil {
			detail = *l.Detail
		}
		logTime := ""
		if l.LogTime != nil {
			logTime = *l.LogTime
		}
		result = append(result, gin.H{
			"id":       l.ID,
			"level":    l.Level,
			"module":   l.Module,
			"message":  l.Message,
			"detail":   detail,
			"log_time": logTime,
		})
	}
	if result == nil {
		result = []gin.H{}
	}

	model.Success(c, result)
}

// GetNewspaperFetchLogs 获取抓取日志
func GetNewspaperFetchLogs(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	type FetchLog struct {
		ID         int
		SourceName string
		Success    int
		Count      int
		Error      *string
		FetchedAt  *string
	}
	var logs []FetchLog
	db.Raw("SELECT id, source_name, success, count, error, fetched_at FROM fetch_logs ORDER BY id DESC LIMIT 100").Scan(&logs)

	var result []gin.H
	for _, l := range logs {
		errMsg := ""
		if l.Error != nil {
			errMsg = *l.Error
		}
		fetchedAt := ""
		if l.FetchedAt != nil {
			fetchedAt = *l.FetchedAt
		}
		result = append(result, gin.H{
			"id":          l.ID,
			"source_name": l.SourceName,
			"success":     l.Success == 1,
			"count":       l.Count,
			"error":       errMsg,
			"fetched_at":  fetchedAt,
		})
	}
	if result == nil {
		result = []gin.H{}
	}

	model.Success(c, result)
}

// GetNewspaperNews 获取新闻列表
func GetNewspaperNews(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	page := 1
	perPage := 20
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if pp := c.Query("per_page"); pp != "" {
		fmt.Sscanf(pp, "%d", &perPage)
	}
	if perPage > 100 {
		perPage = 100
	}

	category := c.Query("category")
	keyword := c.Query("keyword")

	conditions := []string{"1=1"}
	args := []interface{}{}

	if category != "" && category != "all" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	if keyword != "" {
		conditions = append(conditions, "(title LIKE ? OR summary LIKE ?)")
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int64
	db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM news WHERE %s", whereClause), args...).Scan(&total)

	offset := (page - 1) * perPage

	type NewsItem struct {
		ID              int
		Title           string
		URL             string
		Summary         string
		Source          string
		Category        string
		PublishedAt     *string
		FetchedAt       *string
		PoliticalStance *string
		PoliticalScore  *float64
		TopicCategory   *string
		Reliability     *string
	}

	query := fmt.Sprintf(
		"SELECT id, title, url, summary, source, category, published_at, fetched_at, political_stance, political_score, topic_category, reliability FROM news WHERE %s ORDER BY published_at DESC LIMIT ? OFFSET ?",
		whereClause,
	)
	queryArgs := append(args, perPage, offset)

	var newsItems []NewsItem
	db.Raw(query, queryArgs...).Scan(&newsItems)

	var newsList []gin.H
	for _, n := range newsItems {
		publishedAt := ""
		if n.PublishedAt != nil {
			publishedAt = *n.PublishedAt
		}
		fetchedAt := ""
		if n.FetchedAt != nil {
			fetchedAt = *n.FetchedAt
		}
		stance := ""
		if n.PoliticalStance != nil {
			stance = *n.PoliticalStance
		}
		score := float64(0)
		if n.PoliticalScore != nil {
			score = *n.PoliticalScore
		}
		topic := ""
		if n.TopicCategory != nil {
			topic = *n.TopicCategory
		}
		reliability := ""
		if n.Reliability != nil {
			reliability = *n.Reliability
		}
		newsList = append(newsList, gin.H{
			"id":               n.ID,
			"title":            n.Title,
			"url":              n.URL,
			"summary":          n.Summary,
			"source":           n.Source,
			"category":         n.Category,
			"published_at":     publishedAt,
			"fetched_at":       fetchedAt,
			"political_stance": stance,
			"political_score":  score,
			"topic_category":   topic,
			"reliability":      reliability,
		})
	}
	if newsList == nil {
		newsList = []gin.H{}
	}

	model.Success(c, gin.H{
		"total":    total,
		"page":     page,
		"per_page": perPage,
		"list":     newsList,
	})
}

// GetNewspaperCategories 获取所有新闻分类
func GetNewspaperCategories(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	var categories []string
	db.Raw("SELECT DISTINCT category FROM news ORDER BY category").Scan(&categories)
	if categories == nil {
		categories = []string{}
	}

	model.Success(c, categories)
}

// GetUserServiceStats 获取用户服务统计（从 admin-core 自身数据库查询）
func GetUserServiceStats(c *gin.Context) {
	adminDBPath := getAdminDBPath()
	if adminDBPath == "" {
		model.Success(c, gin.H{
			"total_users": 0,
			"note":        "管理后台数据库未找到",
		})
		return
	}

	db, err := openSQLiteDB(adminDBPath)
	if err != nil {
		model.Fail(c, 500, "打开管理后台数据库失败: "+err.Error())
		return
	}

	// 检查 users 表是否存在
	var tableCount int64
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableCount)
	if tableCount == 0 {
		model.Success(c, gin.H{
			"total_users": 0,
			"note":        "用户表不存在",
		})
		return
	}

	var totalUsers int64
	db.Raw("SELECT COUNT(*) FROM users").Scan(&totalUsers)

	today := time.Now().Format("2006-01-02")
	var todayNew int64
	db.Raw("SELECT COUNT(*) FROM users WHERE date(created_at) = ?", today).Scan(&todayNew)

	var activeUsers int64
	db.Raw("SELECT COUNT(*) FROM users WHERE status = 1").Scan(&activeUsers)

	model.Success(c, gin.H{
		"total_users":  totalUsers,
		"today_new":    todayNew,
		"active_users": activeUsers,
		"total_logins": 0,
		"total_regs":   totalUsers,
		"online_users": 0,
	})
}

// GetSearchEngineStats 获取搜索引擎统计
func GetSearchEngineStats(c *gin.Context) {
	workDir, _ := os.Getwd()
	seDBPath := filepath.Join(workDir, "..", "SearchEngine", "search_history.db")

	if _, err := os.Stat(seDBPath); os.IsNotExist(err) {
		altPath := filepath.Join(workDir, "..", "..", "db", "search.db")
		if _, err := os.Stat(altPath); os.IsNotExist(err) {
			model.Success(c, gin.H{
				"index_count":   0,
				"query_count":   0,
				"engine_status": "offline",
				"note":          "搜索引擎数据库未找到",
			})
			return
		}
		seDBPath = altPath
	}

	db, err := openSQLiteDB(seDBPath)
	if err != nil {
		model.Fail(c, 500, "打开搜索引擎数据库失败: "+err.Error())
		return
	}

	var indexCount int64
	err = db.Raw("SELECT COUNT(*) FROM documents").Scan(&indexCount).Error
	if err != nil {
		err = db.Raw("SELECT COUNT(*) FROM search_index").Scan(&indexCount).Error
		if err != nil {
			err = db.Raw("SELECT COUNT(*) FROM search_history").Scan(&indexCount).Error
			if err != nil {
				indexCount = 0
			}
		}
	}

	model.Success(c, gin.H{
		"index_count":   indexCount,
		"query_count":   indexCount,
		"engine_status": "online",
	})
}

// ===== 辅助函数 =====

func getFileSize(path string) int64 {
	absPath, _ := filepath.Abs(path)
	info, err := os.Stat(absPath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// GetDatabaseSizeInfo 获取数据库大小信息
func GetDatabaseSizeInfo(c *gin.Context) {
	workDir, _ := os.Getwd()
	dbDir := filepath.Join(workDir, "..", "..", "db")

	var databases []gin.H

	files := []string{"newspaper.db", "search.db", "user.db"}
	for _, name := range files {
		path := filepath.Join(dbDir, name)
		size := getFileSize(path)
		if size > 0 {
			databases = append(databases, gin.H{
				"name": name,
				"size": size,
				"path": path,
			})
		}
	}

	adminDBPath := filepath.Join(workDir, "data", "admin.db")
	if size := getFileSize(adminDBPath); size > 0 {
		databases = append(databases, gin.H{
			"name": "admin.db",
			"size": size,
			"path": adminDBPath,
		})
	}

	if databases == nil {
		databases = []gin.H{}
	}

	model.Success(c, gin.H{
		"databases": databases,
		"db_dir":    dbDir,
	})
}

// ClearNewspaperData 清空新闻数据
func ClearNewspaperData(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	tables := []string{"news", "fetch_logs", "app_logs"}
	for _, table := range tables {
		result := db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if result.Error != nil {
			log.Printf("清空表 %s 失败: %v", table, result.Error)
		} else {
			log.Printf("清空表 %s: 删除完成", table)
		}
	}

	model.Success(c, gin.H{
		"message": "新闻数据已清空",
		"tables":  tables,
	})
}

// TriggerNewspaperAnalysis 触发新闻分析
func TriggerNewspaperAnalysis(c *gin.Context) {
	db, err := getNewspaperDB()
	if err != nil {
		model.Fail(c, 500, err.Error())
		return
	}

	var unanalyzedCount int64
	db.Raw("SELECT COUNT(*) FROM news WHERE political_stance = '' OR political_stance IS NULL").Scan(&unanalyzedCount)

	model.Success(c, gin.H{
		"message":          "分析请求已记录（原生模式下无法执行分析，请启动新闻服务）",
		"unanalyzed_count": unanalyzedCount,
		"total_analyzed":   0,
	})
}
