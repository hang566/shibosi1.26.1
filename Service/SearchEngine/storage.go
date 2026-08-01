package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func initStorage() error {
	var err error
	db, err = sql.Open("sqlite", "search_history.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	err = createTables()
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS search_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS search_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			history_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			snippet TEXT,
			source TEXT,
			FOREIGN KEY (history_id) REFERENCES search_history(id)
		);`,
		`CREATE TABLE IF NOT EXISTS favorites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			snippet TEXT,
			source TEXT,
			created_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS keyword_feedback (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			keyword TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			link_text TEXT,
			link_url TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS indexed_sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			description TEXT,
			tags TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			approved INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS search_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			keyword TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 1,
			last_searched_at DATETIME NOT NULL,
			UNIQUE(keyword)
		);`,
		`CREATE TABLE IF NOT EXISTS custom_engines (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url_template TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			weight REAL NOT NULL DEFAULT 0.7,
			created_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS search_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query TEXT NOT NULL,
			results_json TEXT NOT NULL,
			result_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
	}

	for _, q := range queries {
		_, err := db.Exec(q)
		if err != nil {
			return err
		}
	}

	initDefaultKeywordFeedback()
	return nil
}

func initDefaultKeywordFeedback() {
	now := time.Now().Format("2006-01-02 15:04:05")
	defaults := []struct {
		keyword  string
		title    string
		content  string
		linkText string
		linkURL  string
	}{
		{
			keyword:  "自杀",
			title:    "💙 请珍惜生命",
			content:  "您似乎正在搜索与自杀相关的内容。生命是最宝贵的，请不要放弃。无论遇到什么困难，都有解决的办法，有人愿意帮助您。请拨打全国心理援助热线：400-161-9995 或 010-82951332，24小时为您服务。",
			linkText: "了解更多心理健康资源",
			linkURL:  "https://www.nhc.gov.cn/",
		},
		{
			keyword:  "想死",
			title:    "💙 请珍惜生命",
			content:  "请您一定要坚持下去！生命是最宝贵的财富。如果您正处于痛苦之中，请立即拨打心理援助热线：400-161-9995，您会得到专业人士的帮助和倾听。",
			linkText: "点击查看全国援助热线列表",
			linkURL:  "https://www.nhc.gov.cn/",
		},
		{
			keyword:  "抑郁",
			title:    "🌿 关于抑郁症",
			content:  "抑郁症是一种可以治疗的心理疾病。如果您持续感到情绪低落、失去兴趣，请不要独自承受。建议您寻求专业心理医生的帮助，或拨打心理援助热线：400-161-9995。",
			linkText: "了解抑郁症的相关知识",
			linkURL:  "https://www.nhc.gov.cn/",
		},
		{
			keyword:  "自残",
			title:    "💙 请停止伤害自己",
			content:  "自残行为会带来永久的伤害。如果您有伤害自己的冲动，请立即联系亲友陪伴您，或拨打心理援助热线：400-161-9995。您值得被温柔以待。",
			linkText: "获取紧急帮助",
			linkURL:  "https://www.nhc.gov.cn/",
		},
	}

	for _, d := range defaults {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM keyword_feedback WHERE keyword = ?", d.keyword).Scan(&count)
		if count == 0 {
			db.Exec(`INSERT INTO keyword_feedback 
				(keyword, title, content, link_text, link_url, enabled, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
				d.keyword, d.title, d.content, d.linkText, d.linkURL, now, now)
		}
	}
}

// ========== 搜索历史 ==========
func SaveSearchResults(query string, results []SearchResult) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := tx.Exec("INSERT INTO search_history (query, created_at) VALUES (?, ?)", query, now)
	if err != nil {
		tx.Rollback()
		return err
	}

	historyID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, r := range results {
		primarySource := r.Source
		if len(r.Sources) > 0 {
			primarySource = r.Sources[0]
		}
		_, err = tx.Exec(
			"INSERT INTO search_results (history_id, title, url, snippet, source) VALUES (?, ?, ?, ?, ?)",
			historyID, r.Title, r.URL, r.Snippet, primarySource,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return UpdateSearchStats(query)
}

type HistoryItem struct {
	ID        int            `json:"id"`
	Query     string         `json:"query"`
	CreatedAt string         `json:"created_at"`
	Results   []SearchResult `json:"results"`
}

func GetSearchHistory(limit int) ([]HistoryItem, error) {
	rows, err := db.Query(
		"SELECT id, query, created_at FROM search_history ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []HistoryItem

	for rows.Next() {
		var item HistoryItem
		err := rows.Scan(&item.ID, &item.Query, &item.CreatedAt)
		if err != nil {
			return nil, err
		}

		item.Results, err = GetResultsByHistoryID(item.ID)
		if err != nil {
			return nil, err
		}

		history = append(history, item)
	}

	return history, nil
}

func GetResultsByHistoryID(historyID int) ([]SearchResult, error) {
	rows, err := db.Query(
		"SELECT title, url, snippet, source FROM search_results WHERE history_id = ?",
		historyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult

	for rows.Next() {
		var r SearchResult
		err := rows.Scan(&r.Title, &r.URL, &r.Snippet, &r.Source)
		if err != nil {
			return nil, err
		}
		if r.Source != "" {
			r.Sources = []string{r.Source}
			r.ResultCount = 1
		}
		results = append(results, r)
	}

	return results, nil
}

func ClearSearchHistory() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM search_results")
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM search_history")
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// ========== 收藏 ==========
type Favorite struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

func AddFavorite(r SearchResult) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	var existingID int64
	db.QueryRow("SELECT id FROM favorites WHERE url = ?", r.URL).Scan(&existingID)
	if existingID > 0 {
		return existingID, fmt.Errorf("已收藏")
	}
	primarySource := r.Source
	if len(r.Sources) > 0 {
		primarySource = r.Sources[0]
	}
	result, err := db.Exec(
		"INSERT INTO favorites (title, url, snippet, source, created_at) VALUES (?, ?, ?, ?, ?)",
		r.Title, r.URL, r.Snippet, primarySource, now,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func RemoveFavorite(id int) error {
	_, err := db.Exec("DELETE FROM favorites WHERE id = ?", id)
	return err
}

func RemoveFavoriteByURL(url string) error {
	_, err := db.Exec("DELETE FROM favorites WHERE url = ?", url)
	return err
}

func GetFavorites() ([]Favorite, error) {
	rows, err := db.Query("SELECT id, title, url, snippet, source, created_at FROM favorites ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var favs []Favorite
	for rows.Next() {
		var f Favorite
		err := rows.Scan(&f.ID, &f.Title, &f.URL, &f.Snippet, &f.Source, &f.CreatedAt)
		if err != nil {
			return nil, err
		}
		favs = append(favs, f)
	}
	return favs, nil
}

func IsFavorite(url string) (bool, int) {
	var id int
	db.QueryRow("SELECT id FROM favorites WHERE url = ?", url).Scan(&id)
	return id > 0, id
}

// ========== 关键词反馈 ==========
type KeywordFeedback struct {
	ID        int    `json:"id"`
	Keyword   string `json:"keyword"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	LinkText  string `json:"link_text"`
	LinkURL   string `json:"link_url"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func GetKeywordFeedback(keyword string) (*KeywordFeedback, error) {
	var kf KeywordFeedback
	var enabledInt int
	err := db.QueryRow(`SELECT id, keyword, title, content, link_text, link_url, enabled, created_at, updated_at 
		FROM keyword_feedback WHERE keyword = ? AND enabled = 1`, keyword).Scan(
		&kf.ID, &kf.Keyword, &kf.Title, &kf.Content, &kf.LinkText, &kf.LinkURL, &enabledInt, &kf.CreatedAt, &kf.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	kf.Enabled = enabledInt == 1
	return &kf, nil
}

func MatchKeywordFeedback(query string) (*KeywordFeedback, error) {
	rows, err := db.Query("SELECT keyword FROM keyword_feedback WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	queryLower := strings.ToLower(query)
	var matchedKeyword string
	for rows.Next() {
		var kw string
		rows.Scan(&kw)
		if strings.Contains(queryLower, strings.ToLower(kw)) {
			matchedKeyword = kw
			break
		}
	}
	if matchedKeyword == "" {
		return nil, nil
	}
	return GetKeywordFeedback(matchedKeyword)
}

func GetAllKeywordFeedback() ([]KeywordFeedback, error) {
	rows, err := db.Query(`SELECT id, keyword, title, content, link_text, link_url, enabled, created_at, updated_at 
		FROM keyword_feedback ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []KeywordFeedback
	for rows.Next() {
		var kf KeywordFeedback
		var enabledInt int
		err := rows.Scan(&kf.ID, &kf.Keyword, &kf.Title, &kf.Content, &kf.LinkText, &kf.LinkURL, &enabledInt, &kf.CreatedAt, &kf.UpdatedAt)
		if err != nil {
			return nil, err
		}
		kf.Enabled = enabledInt == 1
		list = append(list, kf)
	}
	return list, nil
}

func AddKeywordFeedback(kf *KeywordFeedback) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	enabledInt := 0
	if kf.Enabled {
		enabledInt = 1
	}
	result, err := db.Exec(`INSERT INTO keyword_feedback 
		(keyword, title, content, link_text, link_url, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		kf.Keyword, kf.Title, kf.Content, kf.LinkText, kf.LinkURL, enabledInt, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func UpdateKeywordFeedback(kf *KeywordFeedback) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	enabledInt := 0
	if kf.Enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`UPDATE keyword_feedback SET 
		keyword=?, title=?, content=?, link_text=?, link_url=?, enabled=?, updated_at=?
		WHERE id=?`,
		kf.Keyword, kf.Title, kf.Content, kf.LinkText, kf.LinkURL, enabledInt, now, kf.ID)
	return err
}

func DeleteKeywordFeedback(id int) error {
	_, err := db.Exec("DELETE FROM keyword_feedback WHERE id = ?", id)
	return err
}

// ========== 收录站点 ==========
type IndexedSite struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Enabled     bool   `json:"enabled"`
	Approved    bool   `json:"approved"`
	CreatedAt   string `json:"created_at"`
}

func AddIndexedSite(site *IndexedSite) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	enabledInt := 0
	if site.Enabled {
		enabledInt = 1
	}
	approvedInt := 0
	if site.Approved {
		approvedInt = 1
	}
	result, err := db.Exec(`INSERT INTO indexed_sites 
		(name, url, description, tags, enabled, approved, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		site.Name, site.URL, site.Description, site.Tags, enabledInt, approvedInt, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func GetAllIndexedSites(approvedOnly bool) ([]IndexedSite, error) {
	var rows *sql.Rows
	var err error
	if approvedOnly {
		rows, err = db.Query(`SELECT id, name, url, description, tags, enabled, approved, created_at 
			FROM indexed_sites WHERE enabled = 1 AND approved = 1 ORDER BY id DESC`)
	} else {
		rows, err = db.Query(`SELECT id, name, url, description, tags, enabled, approved, created_at 
			FROM indexed_sites ORDER BY id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []IndexedSite
	for rows.Next() {
		var s IndexedSite
		var enabledInt, approvedInt int
		err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Description, &s.Tags, &enabledInt, &approvedInt, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		s.Enabled = enabledInt == 1
		s.Approved = approvedInt == 1
		list = append(list, s)
	}
	return list, nil
}

func UpdateIndexedSite(site *IndexedSite) error {
	enabledInt := 0
	if site.Enabled {
		enabledInt = 1
	}
	approvedInt := 0
	if site.Approved {
		approvedInt = 1
	}
	_, err := db.Exec(`UPDATE indexed_sites SET 
		name=?, url=?, description=?, tags=?, enabled=?, approved=?
		WHERE id=?`,
		site.Name, site.URL, site.Description, site.Tags, enabledInt, approvedInt, site.ID)
	return err
}

func DeleteIndexedSite(id int) error {
	_, err := db.Exec("DELETE FROM indexed_sites WHERE id = ?", id)
	return err
}

func SearchIndexedSites(query string) ([]SearchResult, error) {
	query = "%" + query + "%"
	rows, err := db.Query(`SELECT name, url, description FROM indexed_sites 
		WHERE enabled = 1 AND approved = 1 AND (name LIKE ? OR description LIKE ? OR tags LIKE ? OR url LIKE ?)`,
		query, query, query, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var name, url, desc string
		err := rows.Scan(&name, &url, &desc)
		if err != nil {
			continue
		}
		results = append(results, SearchResult{
			Title:       name,
			URL:         url,
			Snippet:     desc,
			Source:      "收录站点",
			Sources:     []string{"收录站点"},
			ResultCount: 1,
			Score:       1.5,
		})
	}
	return results, nil
}

// ========== 搜索统计 ==========
type SearchStat struct {
	ID           int    `json:"id"`
	Keyword      string `json:"keyword"`
	Count        int    `json:"count"`
	LastSearched string `json:"last_searched_at"`
}

func UpdateSearchStats(keyword string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	var existingCount int
	db.QueryRow("SELECT count FROM search_stats WHERE keyword = ?", keyword).Scan(&existingCount)
	if existingCount > 0 {
		_, err := db.Exec("UPDATE search_stats SET count = count + 1, last_searched_at = ? WHERE keyword = ?", now, keyword)
		return err
	}
	_, err := db.Exec("INSERT INTO search_stats (keyword, count, last_searched_at) VALUES (?, 1, ?)", keyword, now)
	return err
}

func GetTopSearchStats(limit int) ([]SearchStat, error) {
	rows, err := db.Query(`SELECT id, keyword, count, last_searched_at 
		FROM search_stats ORDER BY count DESC, last_searched_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SearchStat
	for rows.Next() {
		var s SearchStat
		err := rows.Scan(&s.ID, &s.Keyword, &s.Count, &s.LastSearched)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

func ClearSearchStats() error {
	_, err := db.Exec("DELETE FROM search_stats")
	return err
}

// ========== 自定义搜索源 ==========
type CustomEngine struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	URLTemplate string  `json:"url_template"`
	Enabled     bool    `json:"enabled"`
	Weight      float64 `json:"weight"`
	CreatedAt   string  `json:"created_at"`
}

func GetCustomEngines(enabledOnly bool) ([]CustomEngine, error) {
	var rows *sql.Rows
	var err error
	if enabledOnly {
		rows, err = db.Query(`SELECT id, name, url_template, enabled, weight, created_at 
			FROM custom_engines WHERE enabled = 1 ORDER BY id DESC`)
	} else {
		rows, err = db.Query(`SELECT id, name, url_template, enabled, weight, created_at 
			FROM custom_engines ORDER BY id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CustomEngine
	for rows.Next() {
		var e CustomEngine
		var enabledInt int
		err := rows.Scan(&e.ID, &e.Name, &e.URLTemplate, &enabledInt, &e.Weight, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		e.Enabled = enabledInt == 1
		list = append(list, e)
	}
	return list, nil
}

func AddCustomEngine(e *CustomEngine) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	enabledInt := 0
	if e.Enabled {
		enabledInt = 1
	}
	result, err := db.Exec(`INSERT INTO custom_engines 
		(name, url_template, enabled, weight, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		e.Name, e.URLTemplate, enabledInt, e.Weight, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func UpdateCustomEngine(e *CustomEngine) error {
	enabledInt := 0
	if e.Enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`UPDATE custom_engines SET 
		name=?, url_template=?, enabled=?, weight=?
		WHERE id=?`,
		e.Name, e.URLTemplate, enabledInt, e.Weight, e.ID)
	return err
}

func DeleteCustomEngine(id int) error {
	_, err := db.Exec("DELETE FROM custom_engines WHERE id = ?", id)
	return err
}

// ============ 搜索会话 ============
type SearchSession struct {
	ID          int    `json:"id"`
	Query       string `json:"query"`
	ResultsJSON string `json:"results_json"`
	ResultCount int    `json:"result_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func SaveSearchSession(query string, results []SearchResult) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return 0, err
	}
	result, err := db.Exec(
		"INSERT INTO search_sessions (query, results_json, result_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		query, string(resultsJSON), len(results), now, now,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func GetSearchSessions(limit int) ([]SearchSession, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query("SELECT id, query, results_json, result_count, created_at, updated_at FROM search_sessions ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []SearchSession
	for rows.Next() {
		var s SearchSession
		if err := rows.Scan(&s.ID, &s.Query, &s.ResultsJSON, &s.ResultCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func GetSearchSessionByID(id int) (*SearchSession, error) {
	var s SearchSession
	err := db.QueryRow("SELECT id, query, results_json, result_count, created_at, updated_at FROM search_sessions WHERE id = ?", id).Scan(&s.ID, &s.Query, &s.ResultsJSON, &s.ResultCount, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func DeleteSearchSession(id int) error {
	_, err := db.Exec("DELETE FROM search_sessions WHERE id = ?", id)
	return err
}

func ClearSearchSessions() error {
	_, err := db.Exec("DELETE FROM search_sessions")
	return err
}
