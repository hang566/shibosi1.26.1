package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

const (
	MaxContentSize    = 10 * 1024     // 限制单条内容最大 10KB
	MaxSummarySize    = 2000          // 限制摘要最大 2000 字符
	MaxLogAgeDays     = 7             // 日志最多保留 7 天
	MaxNewsAgeDays    = 30            // 新闻最多保留 30 天
	AutoCleanInterval = 6 * time.Hour // 每 6 小时自动清理一次
)

type News struct {
	ID              int     `json:"id"`
	Title           string  `json:"title"`
	URL             string  `json:"url"`
	Summary         string  `json:"summary"`
	Content         string  `json:"content"`
	Source          string  `json:"source"`
	Category        string  `json:"category"`
	PublishedAt     string  `json:"published_at"`
	FetchedAt       string  `json:"fetched_at"`
	URLHash         string  `json:"url_hash"`
	PoliticalStance string  `json:"political_stance"`
	PoliticalScore  float64 `json:"political_score"`
	ConfidenceScore float64 `json:"confidence_score"`
	HasPrivateAd    bool    `json:"has_private_ad"`
	AdScore         float64 `json:"ad_score"`
	AdConfidence    float64 `json:"ad_confidence"`
	TopicCategory   string  `json:"topic_category"`
	Reliability     string  `json:"reliability"`
	AnalysisMethods string  `json:"analysis_methods"`
	AnalysisTags    string  `json:"analysis_tags"`
	AnalysisSummary string  `json:"analysis_summary"`
	IsPermanent     bool    `json:"is_permanent"`
}

func (n *News) GetAnalysis() AnalysisResult {
	result := AnalysisResult{
		PoliticalStance: n.PoliticalStance,
		PoliticalScore:  n.PoliticalScore,
		ConfidenceScore: 0,
		HasPrivateAd:    n.HasPrivateAd,
		AdScore:         n.AdScore,
		AdConfidence:    0,
		TopicCategory:   "",
		Reliability:     "",
		Methods:         []MethodResult{},
		Tags:            splitTags(n.AnalysisTags),
		Summary:         n.AnalysisSummary,
	}
	return result
}

func splitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

type NewsResponse struct {
	Total   int    `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	News    []News `json:"news"`
}

type NewsSource struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Type      string `json:"type"`
	Category  string `json:"category"`
	Enabled   bool   `json:"enabled"`
	LastFetch string `json:"last_fetch"`
}

type FetchLog struct {
	ID         int    `json:"id"`
	SourceName string `json:"source_name"`
	Success    bool   `json:"success"`
	Count      int    `json:"count"`
	Error      string `json:"error"`
	FetchedAt  string `json:"fetched_at"`
}

type NewsSettings struct {
	ID        int    `json:"id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

type NewsFavorite struct {
	ID          int     `json:"id"`
	NewsID      int     `json:"news_id"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Summary     string  `json:"summary"`
	Source      string  `json:"source"`
	Category    string  `json:"category"`
	Stance      string  `json:"stance"`
	StanceScore float64 `json:"stance_score"`
	FavoritedAt string  `json:"favorited_at"`
}

var (
	autoCleanOnce sync.Once
)

func initStorage() error {
	dbDir := filepath.Join("..", "..", "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "newspaper.db")
	var err error
	db, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_pragma=cache_size(1000)")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err = createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err = initDefaultSources(); err != nil {
		return fmt.Errorf("failed to init sources: %w", err)
	}

	// 启动时执行一次清理
	go autoCleanup()

	// 启动定时清理任务
	autoCleanOnce.Do(func() {
		go startAutoCleanup()
	})

	return nil
}

func startAutoCleanup() {
	ticker := time.NewTicker(AutoCleanInterval)
	defer ticker.Stop()

	for range ticker.C {
		autoCleanup()
	}
}

func autoCleanup() {
	if db == nil {
		return
	}

	// 清理过期新闻（保留最近 MaxNewsAgeDays 天）
	deletedNews, err := ClearOldNews(MaxNewsAgeDays)
	if err != nil {
		fmt.Printf("Auto cleanup: error cleaning old news: %v\n", err)
	} else if deletedNews > 0 {
		fmt.Printf("Auto cleanup: removed %d old news\n", deletedNews)
	}

	// 清理过期日志
	deletedLogs, err := CleanLogs(MaxLogAgeDays)
	if err != nil {
		fmt.Printf("Auto cleanup: error cleaning old logs: %v\n", err)
	} else if deletedLogs > 0 {
		fmt.Printf("Auto cleanup: removed %d old logs\n", deletedLogs)
	}

	// 清理过期抓取日志
	deletedFetchLogs, err := CleanFetchLogs(MaxLogAgeDays)
	if err != nil {
		fmt.Printf("Auto cleanup: error cleaning old fetch logs: %v\n", err)
	} else if deletedFetchLogs > 0 {
		fmt.Printf("Auto cleanup: removed %d old fetch logs\n", deletedFetchLogs)
	}

	// 执行 WAL checkpoint 回收空间
	CheckpointWAL()

	fmt.Printf("Auto cleanup completed at %s\n", time.Now().Format("2006-01-02 15:04:05"))
}

func CheckpointWAL() {
	if db == nil {
		return
	}
	_, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		fmt.Printf("WAL checkpoint error: %v\n", err)
	}
}

func VACUUMDB() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// 先执行 checkpoint
	CheckpointWAL()

	// 再执行 VACUUM
	_, err := db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("VACUUM error: %w", err)
	}

	return nil
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS app_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level TEXT NOT NULL,
			module TEXT NOT NULL,
			message TEXT NOT NULL,
			details TEXT,
			log_time DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_level ON app_logs(level);`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_module ON app_logs(module);`,
		`CREATE INDEX IF NOT EXISTS idx_app_logs_time ON app_logs(log_time);`,
		`CREATE TABLE IF NOT EXISTS news (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			summary TEXT,
			content TEXT,
			source TEXT NOT NULL,
			category TEXT DEFAULT '综合',
			published_at DATETIME,
			fetched_at DATETIME NOT NULL,
			url_hash TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			political_stance TEXT DEFAULT 'neutral',
			political_score REAL DEFAULT 0,
			confidence_score REAL DEFAULT 0,
			has_private_ad INTEGER DEFAULT 0,
			ad_score REAL DEFAULT 0,
			ad_confidence REAL DEFAULT 0,
			topic_category TEXT DEFAULT '',
			reliability TEXT DEFAULT 'reliable',
			analysis_methods TEXT DEFAULT '',
			analysis_tags TEXT DEFAULT '',
			analysis_summary TEXT DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_news_category ON news(category);`,
		`CREATE INDEX IF NOT EXISTS idx_news_published ON news(published_at);`,
		`CREATE INDEX IF NOT EXISTS idx_news_source ON news(source);`,
		`CREATE TABLE IF NOT EXISTS news_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'rss',
			category TEXT DEFAULT '综合',
			enabled INTEGER NOT NULL DEFAULT 1,
			last_fetch DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS fetch_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_name TEXT NOT NULL,
			success INTEGER NOT NULL DEFAULT 1,
			count INTEGER NOT NULL DEFAULT 0,
			error TEXT,
			fetched_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS news_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			value TEXT,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS news_favorites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			news_id INTEGER,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			summary TEXT,
			source TEXT,
			category TEXT,
			stance TEXT DEFAULT 'neutral',
			stance_score REAL DEFAULT 0,
			favorited_at DATETIME NOT NULL,
			UNIQUE(url)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_favorites_favorited_at ON news_favorites(favorited_at);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	return migrateSchema()
}

func migrateSchema() error {
	migrations := []string{
		`ALTER TABLE news ADD COLUMN political_stance TEXT DEFAULT 'neutral'`,
		`ALTER TABLE news ADD COLUMN political_score REAL DEFAULT 0`,
		`ALTER TABLE news ADD COLUMN confidence_score REAL DEFAULT 0`,
		`ALTER TABLE news ADD COLUMN has_private_ad INTEGER DEFAULT 0`,
		`ALTER TABLE news ADD COLUMN ad_score REAL DEFAULT 0`,
		`ALTER TABLE news ADD COLUMN ad_confidence REAL DEFAULT 0`,
		`ALTER TABLE news ADD COLUMN topic_category TEXT DEFAULT ''`,
		`ALTER TABLE news ADD COLUMN reliability TEXT DEFAULT 'reliable'`,
		`ALTER TABLE news ADD COLUMN analysis_methods TEXT DEFAULT ''`,
		`ALTER TABLE news ADD COLUMN analysis_tags TEXT DEFAULT ''`,
		`ALTER TABLE news ADD COLUMN analysis_summary TEXT DEFAULT ''`,
		`ALTER TABLE news ADD COLUMN is_permanent INTEGER DEFAULT 0`,
	}

	for _, m := range migrations {
		_, err := db.Exec(m)
		if err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				fmt.Printf("Migration note: %v\n", err)
			}
		}
	}
	return nil
}

func initDefaultSources() error {
	defaultSources := []NewsSource{
		{Name: "人民网-时政", URL: "http://www.people.com.cn/rss/politics.xml", Type: "rss", Category: "时政"},
		{Name: "人民网-财经", URL: "http://www.people.com.cn/rss/finance.xml", Type: "rss", Category: "财经"},
		{Name: "人民网-体育", URL: "http://www.people.com.cn/rss/sports.xml", Type: "rss", Category: "体育"},
		{Name: "新华网-时政", URL: "http://www.xinhuanet.com/politics/news_politics.xml", Type: "rss", Category: "时政"},
		{Name: "新华网-财经", URL: "http://www.xinhuanet.com/fortune/news_fortune.xml", Type: "rss", Category: "财经"},
		{Name: "中国新闻网", URL: "https://www.chinanews.com.cn/rss/scroll-news.xml", Type: "rss", Category: "综合"},
		{Name: "36氪", URL: "https://36kr.com/feed", Type: "rss", Category: "科技"},
		{Name: "少数派", URL: "https://sspai.com/feed", Type: "rss", Category: "科技"},
		{Name: "V2EX", URL: "https://www.v2ex.com/feed", Type: "rss", Category: "科技"},
		{Name: "Hacker News", URL: "https://hnrss.org/frontpage", Type: "rss", Category: "科技"},
	}

	for _, src := range defaultSources {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM news_sources WHERE url = ?", src.URL).Scan(&count)
		if err != nil || count == 0 {
			_, err = db.Exec(
				"INSERT INTO news_sources (name, url, type, category, enabled) VALUES (?, ?, ?, ?, 1)",
				src.Name, src.URL, src.Type, src.Category,
			)
			if err != nil {
				fmt.Printf("Warning: cannot insert source %s: %v\n", src.Name, err)
			}
		}
	}

	// 禁用已知失效的源
	brokenURLs := []string{
		"https://www.thepaper.cn/rss_newsDetail_25951",
		"https://www.huxiu.com/rss/0.xml",
		"https://www.jiemian.com.com/rss",
		"https://www.zhihu.com/rss",
	}
	for _, url := range brokenURLs {
		db.Exec("UPDATE news_sources SET enabled = 0 WHERE url = ?", url)
	}

	return nil
}

func getURLHash(url string) string {
	hash := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(url))))
	return hex.EncodeToString(hash[:])
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func SaveNews(n News) (int64, bool, error) {
	n.URLHash = getURLHash(n.URL)
	n.FetchedAt = time.Now().Format("2006-01-02 15:04:05")

	// 限制内容长度，防止数据库膨胀
	n.Content = truncateString(n.Content, MaxContentSize)
	n.Summary = truncateString(n.Summary, MaxSummarySize)

	analysis := AnalyzeNews(n)
	n.PoliticalStance = analysis.PoliticalStance
	n.PoliticalScore = analysis.PoliticalScore
	n.ConfidenceScore = analysis.ConfidenceScore
	n.HasPrivateAd = analysis.HasPrivateAd
	n.AdScore = analysis.AdScore
	n.AdConfidence = analysis.AdConfidence
	n.TopicCategory = analysis.TopicCategory
	n.Reliability = analysis.Reliability
	n.AnalysisTags = strings.Join(analysis.Tags, ",")
	n.AnalysisSummary = analysis.Summary

	methodsJSON, _ := json.Marshal(analysis.Methods)
	n.AnalysisMethods = string(methodsJSON)

	var existingID int64
	err := db.QueryRow("SELECT id FROM news WHERE url_hash = ?", n.URLHash).Scan(&existingID)
	if err == nil && existingID > 0 {
		return existingID, false, nil
	}

	result, err := db.Exec(
		`INSERT INTO news (title, url, summary, content, source, category, published_at, fetched_at, url_hash,
			political_stance, political_score, confidence_score, has_private_ad, ad_score, ad_confidence,
			topic_category, reliability, analysis_methods, analysis_tags, analysis_summary) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Title, n.URL, n.Summary, n.Content, n.Source, n.Category, n.PublishedAt, n.FetchedAt, n.URLHash,
		n.PoliticalStance, n.PoliticalScore, n.ConfidenceScore, n.HasPrivateAd, n.AdScore, n.AdConfidence,
		n.TopicCategory, n.Reliability, n.AnalysisMethods, n.AnalysisTags, n.AnalysisSummary,
	)
	if err != nil {
		return 0, false, err
	}

	id, _ := result.LastInsertId()
	return id, true, nil
}

func SaveNewsBatch(newsList []News) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}

	saved := 0
	for _, n := range newsList {
		n.URLHash = getURLHash(n.URL)
		n.FetchedAt = time.Now().Format("2006-01-02 15:04:05")

		// 限制内容长度，防止数据库膨胀
		n.Content = truncateString(n.Content, MaxContentSize)
		n.Summary = truncateString(n.Summary, MaxSummarySize)

		analysis := AnalyzeNews(n)
		n.PoliticalStance = analysis.PoliticalStance
		n.PoliticalScore = analysis.PoliticalScore
		n.ConfidenceScore = analysis.ConfidenceScore
		n.HasPrivateAd = analysis.HasPrivateAd
		n.AdScore = analysis.AdScore
		n.AdConfidence = analysis.AdConfidence
		n.TopicCategory = analysis.TopicCategory
		n.Reliability = analysis.Reliability
		n.AnalysisTags = strings.Join(analysis.Tags, ",")
		n.AnalysisSummary = analysis.Summary

		methodsJSON, _ := json.Marshal(analysis.Methods)
		n.AnalysisMethods = string(methodsJSON)

		var existingID int64
		err := tx.QueryRow("SELECT id FROM news WHERE url_hash = ?", n.URLHash).Scan(&existingID)
		if err == nil && existingID > 0 {
			continue
		}

		_, err = tx.Exec(
			`INSERT INTO news (title, url, summary, content, source, category, published_at, fetched_at, url_hash,
				political_stance, political_score, confidence_score, has_private_ad, ad_score, ad_confidence,
				topic_category, reliability, analysis_methods, analysis_tags, analysis_summary) 
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.Title, n.URL, n.Summary, n.Content, n.Source, n.Category, n.PublishedAt, n.FetchedAt, n.URLHash,
			n.PoliticalStance, n.PoliticalScore, n.ConfidenceScore, n.HasPrivateAd, n.AdScore, n.AdConfidence,
			n.TopicCategory, n.Reliability, n.AnalysisMethods, n.AnalysisTags, n.AnalysisSummary,
		)
		if err != nil {
			if logger != nil {
				logger.Error("storage", "保存新闻失败 [%s]: %v", n.Title, err)
			}
		} else {
			saved++
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return saved, nil
}

func GetNews(page, perPage int, category string, keyword string, dateFrom, dateTo string) (*NewsResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}

	conditions := []string{"1=1"}
	args := []interface{}{}

	if category != "" && category != "all" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	if keyword != "" {
		conditions = append(conditions, "(title LIKE ? OR summary LIKE ? OR content LIKE ?)")
		kw := "%" + keyword + "%"
		args = append(args, kw, kw, kw)
	}

	if dateFrom != "" {
		conditions = append(conditions, "published_at >= ?")
		args = append(args, dateFrom)
	}

	if dateTo != "" {
		conditions = append(conditions, "published_at <= ?")
		args = append(args, dateTo+" 23:59:59")
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM news WHERE %s", whereClause)
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (page - 1) * perPage
	query := fmt.Sprintf(
		"SELECT id, title, url, summary, content, source, category, published_at, fetched_at, political_stance, political_score, confidence_score, has_private_ad, ad_score, ad_confidence, topic_category, reliability, analysis_methods, analysis_tags, analysis_summary, is_permanent FROM news WHERE %s ORDER BY published_at DESC LIMIT ? OFFSET ?",
		whereClause,
	)
	args = append(args, perPage, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var newsList []News
	for rows.Next() {
		var n News
		var content, publishedAt sql.NullString
		var politicalStance, analysisTags, analysisSummary sql.NullString
		var confidenceScore, politicalScore, adScore, adConfidence sql.NullFloat64
		var hasPrivateAd, isPermanent sql.NullBool
		var topicCategory, reliability, analysisMethods sql.NullString
		if err := rows.Scan(&n.ID, &n.Title, &n.URL, &n.Summary, &content, &n.Source, &n.Category, &publishedAt, &n.FetchedAt,
			&politicalStance, &politicalScore, &confidenceScore, &hasPrivateAd, &adScore, &adConfidence,
			&topicCategory, &reliability, &analysisMethods, &analysisTags, &analysisSummary, &isPermanent); err != nil {
			continue
		}
		if content.Valid {
			n.Content = content.String
		}
		if publishedAt.Valid {
			n.PublishedAt = publishedAt.String
		}
		if politicalStance.Valid {
			n.PoliticalStance = politicalStance.String
		}
		if politicalScore.Valid {
			n.PoliticalScore = politicalScore.Float64
		}
		if confidenceScore.Valid {
			n.ConfidenceScore = confidenceScore.Float64
		}
		if hasPrivateAd.Valid {
			n.HasPrivateAd = hasPrivateAd.Bool
		}
		if adScore.Valid {
			n.AdScore = adScore.Float64
		}
		if adConfidence.Valid {
			n.AdConfidence = adConfidence.Float64
		}
		if topicCategory.Valid {
			n.TopicCategory = topicCategory.String
		}
		if reliability.Valid {
			n.Reliability = reliability.String
		}
		if analysisMethods.Valid {
			n.AnalysisMethods = analysisMethods.String
		}
		if analysisTags.Valid {
			n.AnalysisTags = analysisTags.String
		}
		if analysisSummary.Valid {
			n.AnalysisSummary = analysisSummary.String
		}
		if isPermanent.Valid {
			n.IsPermanent = isPermanent.Bool
		}
		newsList = append(newsList, n)
	}

	if newsList == nil {
		newsList = []News{}
	}

	return &NewsResponse{
		Total:   total,
		Page:    page,
		PerPage: perPage,
		News:    newsList,
	}, nil
}

func GetNewsByID(id int) (*News, error) {
	var n News
	var content, publishedAt, politicalStance, analysisTags, analysisSummary sql.NullString
	var confidenceScore, politicalScore, adScore, adConfidence sql.NullFloat64
	var hasPrivateAd, isPermanent sql.NullBool
	var topicCategory, reliability, analysisMethods sql.NullString
	err := db.QueryRow(
		"SELECT id, title, url, summary, content, source, category, published_at, fetched_at, political_stance, political_score, confidence_score, has_private_ad, ad_score, ad_confidence, topic_category, reliability, analysis_methods, analysis_tags, analysis_summary, is_permanent FROM news WHERE id = ?",
		id,
	).Scan(&n.ID, &n.Title, &n.URL, &n.Summary, &content, &n.Source, &n.Category, &publishedAt, &n.FetchedAt,
		&politicalStance, &politicalScore, &confidenceScore, &hasPrivateAd, &adScore, &adConfidence,
		&topicCategory, &reliability, &analysisMethods, &analysisTags, &analysisSummary, &isPermanent)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if content.Valid {
		n.Content = content.String
	}
	if publishedAt.Valid {
		n.PublishedAt = publishedAt.String
	}
	if politicalStance.Valid {
		n.PoliticalStance = politicalStance.String
	}
	if politicalScore.Valid {
		n.PoliticalScore = politicalScore.Float64
	}
	if confidenceScore.Valid {
		n.ConfidenceScore = confidenceScore.Float64
	}
	if hasPrivateAd.Valid {
		n.HasPrivateAd = hasPrivateAd.Bool
	}
	if adScore.Valid {
		n.AdScore = adScore.Float64
	}
	if adConfidence.Valid {
		n.AdConfidence = adConfidence.Float64
	}
	if topicCategory.Valid {
		n.TopicCategory = topicCategory.String
	}
	if reliability.Valid {
		n.Reliability = reliability.String
	}
	if analysisMethods.Valid {
		n.AnalysisMethods = analysisMethods.String
	}
	if analysisTags.Valid {
		n.AnalysisTags = analysisTags.String
	}
	if analysisSummary.Valid {
		n.AnalysisSummary = analysisSummary.String
	}
	if isPermanent.Valid {
		n.IsPermanent = isPermanent.Bool
	}
	return &n, nil
}

func GetCategories() ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT category FROM news ORDER BY category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			continue
		}
		categories = append(categories, cat)
	}
	if categories == nil {
		categories = []string{}
	}
	return categories, nil
}

func GetSources() ([]NewsSource, error) {
	rows, err := db.Query("SELECT id, name, url, type, category, enabled, last_fetch FROM news_sources ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []NewsSource
	for rows.Next() {
		var s NewsSource
		var enabledInt int
		var lastFetch sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Type, &s.Category, &enabledInt, &lastFetch); err != nil {
			continue
		}
		s.Enabled = enabledInt == 1
		if lastFetch.Valid {
			s.LastFetch = lastFetch.String
		}
		sources = append(sources, s)
	}
	if sources == nil {
		sources = []NewsSource{}
	}
	return sources, nil
}

func AddSource(s NewsSource) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO news_sources (name, url, type, category, enabled) VALUES (?, ?, ?, ?, ?)",
		s.Name, s.URL, s.Type, s.Category, 1,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func UpdateSource(s NewsSource) error {
	enabledInt := 0
	if s.Enabled {
		enabledInt = 1
	}
	_, err := db.Exec(
		"UPDATE news_sources SET name=?, url=?, type=?, category=?, enabled=? WHERE id=?",
		s.Name, s.URL, s.Type, s.Category, enabledInt, s.ID,
	)
	return err
}

func DeleteSource(id int) error {
	_, err := db.Exec("DELETE FROM news_sources WHERE id = ?", id)
	return err
}

func UpdateSourceLastFetch(id int) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec("UPDATE news_sources SET last_fetch = ? WHERE id = ?", now, id)
	return err
}

func AddFetchLog(log FetchLog) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		"INSERT INTO fetch_logs (source_name, success, count, error, fetched_at) VALUES (?, ?, ?, ?, ?)",
		log.SourceName, log.Success, log.Count, log.Error, now,
	)
	return err
}

func GetFetchLogs(limit int) ([]FetchLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query("SELECT id, source_name, success, count, error, fetched_at FROM fetch_logs ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []FetchLog
	for rows.Next() {
		var l FetchLog
		var successInt int
		var errMsg sql.NullString
		if err := rows.Scan(&l.ID, &l.SourceName, &successInt, &l.Count, &errMsg, &l.FetchedAt); err != nil {
			continue
		}
		l.Success = successInt == 1
		if errMsg.Valid {
			l.Error = errMsg.String
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []FetchLog{}
	}
	return logs, nil
}

func GetSettings() (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM news_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		settings[k] = v
	}
	return settings, nil
}

func UpdateSetting(key, value string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		`INSERT INTO news_settings (key, value, updated_at) VALUES (?, ?, ?) 
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, now,
	)
	return err
}

func ClearOldNews(days int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	result, err := db.Exec("DELETE FROM news WHERE fetched_at < ? AND is_permanent = 0", cutoff)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func SetNewsPermanent(id int, permanent bool) error {
	_, err := db.Exec("UPDATE news SET is_permanent = ? WHERE id = ?", permanent, id)
	return err
}

func GetPermanentNewsCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM news WHERE is_permanent = 1").Scan(&count)
	return count, err
}

func SetBulkPermanent(ids []int, permanent bool) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = permanent
	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}
	query := fmt.Sprintf("UPDATE news SET is_permanent = ? WHERE id IN (%s)", strings.Join(placeholders, ","))
	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func SetAllPermanent(permanent bool) (int, error) {
	result, err := db.Exec("UPDATE news SET is_permanent = ?", permanent)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func PermanentOldNews(days int, permanent bool) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	result, err := db.Exec("UPDATE news SET is_permanent = ? WHERE fetched_at < ? AND is_permanent != ?", permanent, cutoff, permanent)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func CleanFetchLogs(olderThanDays int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays).Format("2006-01-02 15:04:05")
	result, err := db.Exec("DELETE FROM fetch_logs WHERE fetched_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func CleanAllData() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// 清空所有表
	tables := []string{"news", "fetch_logs", "app_logs"}
	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			return fmt.Errorf("failed to clean %s: %w", table, err)
		}
	}

	// 重置自增ID
	for _, table := range tables {
		db.Exec(fmt.Sprintf("DELETE FROM sqlite_sequence WHERE name='%s'", table))
	}

	// VACUUM 回收空间
	return VACUUMDB()
}

func GetNewsByDateRange(dateFrom, dateTo string) ([]News, error) {
	query := "SELECT id, title, url, summary, content, source, category, published_at, fetched_at, political_stance, political_score, confidence_score, has_private_ad, ad_score, ad_confidence, topic_category, reliability, analysis_methods, analysis_tags, analysis_summary FROM news WHERE 1=1"
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND published_at >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND published_at <= ?"
		args = append(args, dateTo+" 23:59:59")
	}
	query += " ORDER BY published_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var newsList []News
	for rows.Next() {
		var n News
		var content, publishedAt sql.NullString
		var politicalStance, analysisTags, analysisSummary sql.NullString
		var confidenceScore, politicalScore, adScore, adConfidence sql.NullFloat64
		var hasPrivateAd sql.NullBool
		var topicCategory, reliability, analysisMethods sql.NullString
		if err := rows.Scan(&n.ID, &n.Title, &n.URL, &n.Summary, &content, &n.Source, &n.Category, &publishedAt, &n.FetchedAt,
			&politicalStance, &politicalScore, &confidenceScore, &hasPrivateAd, &adScore, &adConfidence,
			&topicCategory, &reliability, &analysisMethods, &analysisTags, &analysisSummary); err != nil {
			continue
		}
		if content.Valid {
			n.Content = content.String
		}
		if publishedAt.Valid {
			n.PublishedAt = publishedAt.String
		}
		if politicalStance.Valid {
			n.PoliticalStance = politicalStance.String
		}
		if politicalScore.Valid {
			n.PoliticalScore = politicalScore.Float64
		}
		if confidenceScore.Valid {
			n.ConfidenceScore = confidenceScore.Float64
		}
		if hasPrivateAd.Valid {
			n.HasPrivateAd = hasPrivateAd.Bool
		}
		if adScore.Valid {
			n.AdScore = adScore.Float64
		}
		if adConfidence.Valid {
			n.AdConfidence = adConfidence.Float64
		}
		if topicCategory.Valid {
			n.TopicCategory = topicCategory.String
		}
		if reliability.Valid {
			n.Reliability = reliability.String
		}
		if analysisMethods.Valid {
			n.AnalysisMethods = analysisMethods.String
		}
		if analysisTags.Valid {
			n.AnalysisTags = analysisTags.String
		}
		if analysisSummary.Valid {
			n.AnalysisSummary = analysisSummary.String
		}
		newsList = append(newsList, n)
	}
	if newsList == nil {
		newsList = []News{}
	}
	return newsList, nil
}

func GetNewsStats() (map[string]interface{}, error) {
	var totalCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM news").Scan(&totalCount); err != nil {
		return nil, err
	}

	var categoryCount int
	if err := db.QueryRow("SELECT COUNT(DISTINCT category) FROM news").Scan(&categoryCount); err != nil {
		return nil, err
	}

	var sourceCount int
	if err := db.QueryRow("SELECT COUNT(DISTINCT source) FROM news").Scan(&sourceCount); err != nil {
		return nil, err
	}

	var todayCount int
	today := time.Now().Format("2006-01-02")
	if err := db.QueryRow("SELECT COUNT(*) FROM news WHERE date(fetched_at) = ?", today).Scan(&todayCount); err != nil {
		return nil, err
	}

	var permanentCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM news WHERE is_permanent = 1").Scan(&permanentCount); err != nil {
		return nil, err
	}

	var latestFetch sql.NullString
	db.QueryRow("SELECT MAX(fetched_at) FROM news").Scan(&latestFetch)

	return map[string]interface{}{
		"total_count":     totalCount,
		"category_count":  categoryCount,
		"source_count":    sourceCount,
		"today_count":     todayCount,
		"permanent_count": permanentCount,
		"latest_fetch":    latestFetch.String,
	}, nil
}

func SearchNewsAdvanced(query string, categories []string, sources []string, dateFrom, dateTo string) ([]News, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}

	if query != "" {
		conditions = append(conditions, "(title LIKE ? OR summary LIKE ? OR content LIKE ?)")
		kw := "%" + query + "%"
		args = append(args, kw, kw, kw)
	}

	if len(categories) > 0 {
		placeholders := make([]string, len(categories))
		for i, c := range categories {
			placeholders[i] = "?"
			args = append(args, c)
		}
		conditions = append(conditions, fmt.Sprintf("category IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, s := range sources {
			placeholders[i] = "?"
			args = append(args, s)
		}
		conditions = append(conditions, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ",")))
	}

	if dateFrom != "" {
		conditions = append(conditions, "published_at >= ?")
		args = append(args, dateFrom)
	}

	if dateTo != "" {
		conditions = append(conditions, "published_at <= ?")
		args = append(args, dateTo+" 23:59:59")
	}

	whereClause := strings.Join(conditions, " AND ")
	sqlQuery := fmt.Sprintf(
		"SELECT id, title, url, summary, content, source, category, published_at, fetched_at, political_stance, political_score, confidence_score, has_private_ad, ad_score, ad_confidence, topic_category, reliability, analysis_methods, analysis_tags, analysis_summary FROM news WHERE %s ORDER BY published_at DESC",
		whereClause,
	)

	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var newsList []News
	for rows.Next() {
		var n News
		var content, publishedAt sql.NullString
		var politicalStance, analysisTags, analysisSummary sql.NullString
		var confidenceScore, politicalScore, adScore, adConfidence sql.NullFloat64
		var hasPrivateAd sql.NullBool
		var topicCategory, reliability, analysisMethods sql.NullString
		if err := rows.Scan(&n.ID, &n.Title, &n.URL, &n.Summary, &content, &n.Source, &n.Category, &publishedAt, &n.FetchedAt,
			&politicalStance, &politicalScore, &confidenceScore, &hasPrivateAd, &adScore, &adConfidence,
			&topicCategory, &reliability, &analysisMethods, &analysisTags, &analysisSummary); err != nil {
			continue
		}
		if content.Valid {
			n.Content = content.String
		}
		if publishedAt.Valid {
			n.PublishedAt = publishedAt.String
		}
		if politicalStance.Valid {
			n.PoliticalStance = politicalStance.String
		}
		if politicalScore.Valid {
			n.PoliticalScore = politicalScore.Float64
		}
		if confidenceScore.Valid {
			n.ConfidenceScore = confidenceScore.Float64
		}
		if hasPrivateAd.Valid {
			n.HasPrivateAd = hasPrivateAd.Bool
		}
		if adScore.Valid {
			n.AdScore = adScore.Float64
		}
		if adConfidence.Valid {
			n.AdConfidence = adConfidence.Float64
		}
		if topicCategory.Valid {
			n.TopicCategory = topicCategory.String
		}
		if reliability.Valid {
			n.Reliability = reliability.String
		}
		if analysisMethods.Valid {
			n.AnalysisMethods = analysisMethods.String
		}
		if analysisTags.Valid {
			n.AnalysisTags = analysisTags.String
		}
		if analysisSummary.Valid {
			n.AnalysisSummary = analysisSummary.String
		}
		newsList = append(newsList, n)
	}
	if newsList == nil {
		newsList = []News{}
	}
	return newsList, nil
}

func ExportNewsJSON(newsList []News) (string, error) {
	data := map[string]interface{}{
		"exported_at": time.Now().Format("2006-01-02 15:04:05"),
		"count":       len(newsList),
		"news":        newsList,
	}
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

func ReanalyzeNews(limit int) (int, error) {
	query := "SELECT id, title, url, summary, content, source, category, published_at FROM news WHERE political_stance = '' OR political_stance = 'neutral' OR political_stance IS NULL ORDER BY id DESC"
	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := db.Query(query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var n News
		var content, publishedAt sql.NullString
		if err := rows.Scan(&n.ID, &n.Title, &n.URL, &n.Summary, &content, &n.Source, &n.Category, &publishedAt); err != nil {
			continue
		}
		if content.Valid {
			n.Content = content.String
		}
		if publishedAt.Valid {
			n.PublishedAt = publishedAt.String
		}

		analysis := AnalyzeNews(n)

		methodsJSON, _ := json.Marshal(analysis.Methods)
		_, err := db.Exec(
			`UPDATE news SET political_stance = ?, political_score = ?, confidence_score = ?, has_private_ad = ?, ad_score = ?, ad_confidence = ?, topic_category = ?, reliability = ?, analysis_methods = ?, analysis_tags = ?, analysis_summary = ? WHERE id = ?`,
			analysis.PoliticalStance, analysis.PoliticalScore, analysis.ConfidenceScore, analysis.HasPrivateAd,
			analysis.AdScore, analysis.AdConfidence, analysis.TopicCategory, analysis.Reliability,
			string(methodsJSON), strings.Join(analysis.Tags, ","), analysis.Summary, n.ID,
		)
		if err == nil {
			count++
		}
	}
	return count, nil
}

func GetAnalysisStats() (map[string]interface{}, error) {
	var totalCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM news WHERE political_stance != ''").Scan(&totalCount); err != nil {
		return nil, err
	}

	var leftCount, rightCount, neutralCount int
	db.QueryRow("SELECT COUNT(*) FROM news WHERE political_stance IN ('left', 'center-left')").Scan(&leftCount)
	db.QueryRow("SELECT COUNT(*) FROM news WHERE political_stance IN ('right', 'center-right')").Scan(&rightCount)
	db.QueryRow("SELECT COUNT(*) FROM news WHERE political_stance = 'neutral'").Scan(&neutralCount)

	var adCount int
	db.QueryRow("SELECT COUNT(*) FROM news WHERE has_private_ad = 1").Scan(&adCount)

	var adScoreSum sql.NullFloat64
	db.QueryRow("SELECT AVG(ad_score) FROM news").Scan(&adScoreSum)

	var politicalScoreSum sql.NullFloat64
	db.QueryRow("SELECT AVG(ABS(political_score)) FROM news WHERE political_stance != 'neutral'").Scan(&politicalScoreSum)

	return map[string]interface{}{
		"analyzed_count":        totalCount,
		"left_count":            leftCount,
		"right_count":           rightCount,
		"neutral_count":         neutralCount,
		"private_ad_count":      adCount,
		"avg_ad_score":          adScoreSum.Float64,
		"avg_political_extreme": politicalScoreSum.Float64,
	}, nil
}

func GetDBSize() (int64, error) {
	if db == nil {
		return 0, nil
	}

	// 获取数据库文件实际大小
	dbPath := getDBPath()
	var totalSize int64
	if dbPath != "" {
		if info, err := os.Stat(dbPath); err == nil {
			totalSize = info.Size()
		}
		// 加上 WAL 和 SHM 文件大小
		if walInfo, err := os.Stat(dbPath + "-wal"); err == nil {
			totalSize += walInfo.Size()
		}
		if shmInfo, err := os.Stat(dbPath + "-shm"); err == nil {
			totalSize += shmInfo.Size()
		}
	}

	return totalSize, nil
}

func GetTableStats() ([]map[string]interface{}, error) {
	if db == nil {
		return nil, nil
	}

	tables := []string{"news", "news_sources", "fetch_logs", "news_settings", "app_logs"}
	var result []map[string]interface{}

	// 获取数据库文件实际大小
	dbPath := getDBPath()
	var totalFileSize int64
	if dbPath != "" {
		if info, err := os.Stat(dbPath); err == nil {
			totalFileSize = info.Size()
		}
		// 加上 WAL 文件大小
		if walInfo, err := os.Stat(dbPath + "-wal"); err == nil {
			totalFileSize += walInfo.Size()
		}
	}

	// 计算每个表的实际使用大小（使用 sqlite_db_page_size * page_count）
	var totalUsedPages int64
	for _, table := range tables {
		var pageCount int64
		pageQuery := fmt.Sprintf("SELECT SUM(pgsize) FROM dbstat WHERE name='%s'", table)
		err := db.QueryRow(pageQuery).Scan(&pageCount)
		if err != nil {
			pageCount = 0
		}
		totalUsedPages += pageCount
	}

	pageSize := int64(4096) // SQLite 默认页大小

	for _, table := range tables {
		var rowsCount int
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		db.QueryRow(countQuery).Scan(&rowsCount)

		var pageCount int64
		pageQuery := fmt.Sprintf("SELECT SUM(pgsize) FROM dbstat WHERE name='%s'", table)
		err := db.QueryRow(pageQuery).Scan(&pageCount)
		if err != nil {
			pageCount = 0
		}

		// 使用实际文件大小按比例计算
		var tableSize int64
		if totalUsedPages > 0 && totalFileSize > 0 {
			tableSize = int64(float64(totalFileSize) * (float64(pageCount) / float64(totalUsedPages)))
		} else {
			tableSize = pageCount * pageSize
		}

		result = append(result, map[string]interface{}{
			"name":       table,
			"row_count":  rowsCount,
			"size_bytes": tableSize,
		})
	}

	if result == nil {
		result = []map[string]interface{}{}
	}
	return result, nil
}

func getDBPath() string {
	dbDir := filepath.Join("..", "..", "db")
	return filepath.Join(dbDir, "newspaper.db")
}

func GetNewsSourcesEnabled() ([]NewsSource, error) {
	rows, err := db.Query("SELECT id, name, url, type, category, enabled, last_fetch FROM news_sources WHERE enabled = 1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []NewsSource
	for rows.Next() {
		var s NewsSource
		var enabledInt int
		var lastFetch sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Type, &s.Category, &enabledInt, &lastFetch); err != nil {
			continue
		}
		s.Enabled = enabledInt == 1
		if lastFetch.Valid {
			s.LastFetch = lastFetch.String
		}
		sources = append(sources, s)
	}
	if sources == nil {
		sources = []NewsSource{}
	}
	return sources, nil
}

func GetAnalysisResultsSummary() ([]map[string]interface{}, error) {
	if db == nil {
		return nil, nil
	}

	queries := map[string]string{
		"political_stance": "SELECT political_stance as value, COUNT(*) as count FROM news WHERE political_stance != '' GROUP BY political_stance",
		"topic_category":   "SELECT topic_category as value, COUNT(*) as count FROM news WHERE topic_category != '' GROUP BY topic_category",
		"reliability":      "SELECT reliability as value, COUNT(*) as count FROM news WHERE reliability != '' GROUP BY reliability",
	}

	var result []map[string]interface{}
	for key, query := range queries {
		rows, err := db.Query(query)
		if err != nil {
			continue
		}

		for rows.Next() {
			var value string
			var count int
			if err := rows.Scan(&value, &count); err != nil {
				continue
			}
			result = append(result, map[string]interface{}{
				"type":  key,
				"value": value,
				"count": count,
			})
		}
		rows.Close()
	}

	if result == nil {
		result = []map[string]interface{}{}
	}
	return result, nil
}

func GetAnalysisTimeStats() (map[string]interface{}, error) {
	if db == nil {
		return nil, nil
	}

	var totalCount int
	db.QueryRow("SELECT COUNT(*) FROM news WHERE political_stance != ''").Scan(&totalCount)

	var avgConfidence sql.NullFloat64
	db.QueryRow("SELECT AVG(confidence_score) FROM news WHERE confidence_score > 0").Scan(&avgConfidence)

	var highConfidenceCount int
	db.QueryRow("SELECT COUNT(*) FROM news WHERE confidence_score >= 0.7").Scan(&highConfidenceCount)

	var adDetectedCount int
	db.QueryRow("SELECT COUNT(*) FROM news WHERE has_private_ad = 1").Scan(&adDetectedCount)

	return map[string]interface{}{
		"analyzed_count":        totalCount,
		"avg_confidence":        avgConfidence.Float64,
		"high_confidence_count": highConfidenceCount,
		"ad_detected_count":     adDetectedCount,
	}, nil
}

func AddFavorite(fav NewsFavorite) (int64, error) {
	fav.FavoritedAt = time.Now().Format("2006-01-02 15:04:05")

	var existingID int64
	err := db.QueryRow("SELECT id FROM news_favorites WHERE url = ?", fav.URL).Scan(&existingID)
	if err == nil && existingID > 0 {
		_, err = db.Exec("UPDATE news_favorites SET news_id=?, title=?, summary=?, source=?, category=?, stance=?, stance_score=? WHERE id=?",
			fav.NewsID, fav.Title, fav.Summary, fav.Source, fav.Category, fav.Stance, fav.StanceScore, existingID)
		return existingID, err
	}

	result, err := db.Exec(
		`INSERT INTO news_favorites (news_id, title, url, summary, source, category, stance, stance_score, favorited_at) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fav.NewsID, fav.Title, fav.URL, fav.Summary, fav.Source, fav.Category, fav.Stance, fav.StanceScore, fav.FavoritedAt,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func GetFavorites(page, perPage int) ([]NewsFavorite, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 50
	}

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM news_favorites").Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := db.Query(
		"SELECT id, news_id, title, url, summary, source, category, stance, stance_score, favorited_at FROM news_favorites ORDER BY favorited_at DESC LIMIT ? OFFSET ?",
		perPage, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var favs []NewsFavorite
	for rows.Next() {
		var f NewsFavorite
		var newsID sql.NullInt64
		var summary, source, category, stance sql.NullString
		var stanceScore sql.NullFloat64
		if err := rows.Scan(&f.ID, &newsID, &f.Title, &f.URL, &summary, &source, &category, &stance, &stanceScore, &f.FavoritedAt); err != nil {
			continue
		}
		if newsID.Valid {
			f.NewsID = int(newsID.Int64)
		}
		if summary.Valid {
			f.Summary = summary.String
		}
		if source.Valid {
			f.Source = source.String
		}
		if category.Valid {
			f.Category = category.String
		}
		if stance.Valid {
			f.Stance = stance.String
		}
		if stanceScore.Valid {
			f.StanceScore = stanceScore.Float64
		}
		favs = append(favs, f)
	}
	if favs == nil {
		favs = []NewsFavorite{}
	}
	return favs, total, nil
}

func RemoveFavorite(id int) error {
	_, err := db.Exec("DELETE FROM news_favorites WHERE id = ?", id)
	return err
}

func RemoveFavoriteByURL(url string) error {
	_, err := db.Exec("DELETE FROM news_favorites WHERE url = ?", url)
	return err
}

func IsFavorite(url string) (bool, int) {
	var id int
	err := db.QueryRow("SELECT id FROM news_favorites WHERE url = ?", url).Scan(&id)
	if err != nil {
		return false, 0
	}
	return true, id
}

func GetFavoriteCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM news_favorites").Scan(&count)
	return count, err
}

func ClearAllFavorites() error {
	_, err := db.Exec("DELETE FROM news_favorites")
	return err
}
