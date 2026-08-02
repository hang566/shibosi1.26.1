package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
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
		// ===== 爬虫索引相关表 =====
		`CREATE TABLE IF NOT EXISTS crawler_pages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			content TEXT,
			snippet TEXT,
			domain TEXT,
			source TEXT,
			depth INTEGER NOT NULL DEFAULT 0,
			status_code INTEGER NOT NULL DEFAULT 0,
			content_length INTEGER NOT NULL DEFAULT 0,
			crawled_at DATETIME NOT NULL,
			indexed INTEGER NOT NULL DEFAULT 1
		);`,
		`CREATE INDEX IF NOT EXISTS idx_crawler_pages_domain ON crawler_pages(domain);`,
		`CREATE INDEX IF NOT EXISTS idx_crawler_pages_crawled ON crawler_pages(crawled_at DESC);`,
		`CREATE TABLE IF NOT EXISTS crawler_seeds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL UNIQUE,
			name TEXT,
			category TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			built_in INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS crawler_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			depth INTEGER NOT NULL DEFAULT 0,
			source TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			error TEXT,
			created_at DATETIME NOT NULL,
			processed_at DATETIME
		);`,
		`CREATE INDEX IF NOT EXISTS idx_crawler_tasks_status ON crawler_tasks(status);`,
		// ===== 引擎启停设置 =====
		`CREATE TABLE IF NOT EXISTS engine_settings (
			name TEXT PRIMARY KEY,
			enabled INTEGER NOT NULL DEFAULT 1,
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
	initDefaultCrawlerSeeds()
	return nil
}

// ========== 爬虫索引页面 ==========

// CrawlerPage 已爬取的网页
type CrawlerPage struct {
	ID            int    `json:"id"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	Content       string `json:"content,omitempty"`
	Snippet       string `json:"snippet"`
	Domain        string `json:"domain"`
	Source        string `json:"source"`
	Depth         int    `json:"depth"`
	StatusCode    int    `json:"status_code"`
	ContentLength int    `json:"content_length"`
	CrawledAt     string `json:"crawled_at"`
	Indexed       bool   `json:"indexed"`
}

// SaveCrawlerPage 保存或更新爬取的页面
func SaveCrawlerPage(page *CrawlerPage) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	page.CrawledAt = now
	indexedVal := 0
	if page.Indexed {
		indexedVal = 1
	}
	_, err := db.Exec(`INSERT INTO crawler_pages
		(url, title, content, snippet, domain, source, depth, status_code, content_length, crawled_at, indexed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			title=excluded.title,
			content=excluded.content,
			snippet=excluded.snippet,
			domain=excluded.domain,
			source=excluded.source,
			depth=excluded.depth,
			status_code=excluded.status_code,
			content_length=excluded.content_length,
			crawled_at=excluded.crawled_at,
			indexed=excluded.indexed`,
		page.URL, page.Title, page.Content, page.Snippet,
		page.Domain, page.Source, page.Depth, page.StatusCode,
		page.ContentLength, now, indexedVal)
	return err
}

// GetCrawlerPages 分页获取已索引页面
func GetCrawlerPages(limit, offset int) ([]CrawlerPage, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := db.Query(`SELECT id, url, title, snippet, domain, source, depth, status_code, content_length, crawled_at
		FROM crawler_pages ORDER BY crawled_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []CrawlerPage
	for rows.Next() {
		var p CrawlerPage
		if err := rows.Scan(&p.ID, &p.URL, &p.Title, &p.Snippet, &p.Domain,
			&p.Source, &p.Depth, &p.StatusCode, &p.ContentLength, &p.CrawledAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

// CountCrawlerPages 统计已索引页数
func CountCrawlerPages() (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM crawler_pages WHERE indexed = 1").Scan(&count)
	return count, err
}

// DeleteCrawlerPage 删除单个页面
func DeleteCrawlerPage(id int) error {
	_, err := db.Exec("DELETE FROM crawler_pages WHERE id = ?", id)
	return err
}

// ClearCrawlerPages 清空所有爬虫索引
func ClearCrawlerPages() error {
	_, err := db.Exec("DELETE FROM crawler_pages")
	return err
}

// SearchCrawlerPages 在已索引页面中搜索（改进版：标题优先 + 内容质量过滤）
func SearchCrawlerPages(query string, limit int) ([]SearchResult, error) {
	if query = strings.TrimSpace(query); query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	keywords := splitQueryKeywords(query)
	if len(keywords) == 0 {
		keywords = []string{query}
	}

	// 构建 SQL：每个关键词都 LIKE title 或 content 或 snippet
	// 使用分组条件：匹配 title 的强匹配，匹配 content/snippet 的弱匹配
	whereParts := make([]string, 0, len(keywords)*3)
	args := make([]interface{}, 0, len(keywords)*3+2)
	for _, kw := range keywords {
		like := "%" + kw + "%"
		whereParts = append(whereParts, "title LIKE ?", "content LIKE ?", "snippet LIKE ?")
		args = append(args, like, like, like)
	}
	where := strings.Join(whereParts, " OR ")
	// 过滤短内容（content_length < 200 的页面几乎没正文），多取一些用于评分排序 + 域去重
	sqlStr := "SELECT url, title, snippet, domain, source, content_length FROM crawler_pages WHERE indexed = 1 AND content_length >= 200 AND (" + where + ") LIMIT ?"
	args = append(args, limit*5)

	rows, err := db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		r      SearchResult
		score  float64
		domain string
	}
	var hits []scored
	domainCount := make(map[string]int)
	const maxPerDomain = 3 // 每个域名最多 3 条

	for rows.Next() {
		var url, title, snippet, domain, source string
		var contentLength int
		if err := rows.Scan(&url, &title, &snippet, &domain, &source, &contentLength); err != nil {
			continue
		}

		// 域名去重限制：主域相同算一个域
		rootDomain := extractRootDomain(domain)
		if domainCount[rootDomain] >= maxPerDomain {
			continue
		}

		score := 0.0
		titleLower := strings.ToLower(title)
		snippetLower := strings.ToLower(snippet)
		allKWInTitle := true
		for _, kw := range keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(titleLower, kwLower) {
				score += 5.0 // 标题匹配权重大幅提升
				if titleLower == kwLower {
					score += 3.0 // 完全匹配额外加分
				}
			} else {
				allKWInTitle = false
			}
			if strings.Contains(snippetLower, kwLower) {
				score += 3.0 // 摘要匹配权重提升
			}
		}
		// 所有关键词都在标题中出现：额外加分
		if allKWInTitle && len(keywords) > 1 {
			score += 2.0
		}
		// 内容长度加分：更长的页面通常内容更丰富（但有限度）
		if contentLength > 1000 {
			score += 1.0
		} else if contentLength > 500 {
			score += 0.5
		}

		if isOfficial, _ := isBrandOfficialDomain(domain, query); isOfficial {
			score += 1.5
		}
		// 域名多样性加分：当前域名首次出现额外奖励
		if domainCount[rootDomain] == 0 {
			score += 0.5
		}
		if score <= 0 {
			score = 0.5
		}
		if snippet == "" {
			snippet = domain
		}

		domainCount[rootDomain]++
		hits = append(hits, scored{
			r: SearchResult{
				Title:   title,
				URL:     url,
				Snippet: snippet,
				Source:  "CrawlerIndex",
				Sources: []string{"CrawlerIndex"},
			},
			score:  score,
			domain: rootDomain,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})

	// 再次做域去重，确保排序后仍限制
	domainCount2 := make(map[string]int)
	var filtered []scored
	for _, h := range hits {
		if domainCount2[h.domain] >= maxPerDomain {
			continue
		}
		domainCount2[h.domain]++
		filtered = append(filtered, h)
	}
	hits = filtered

	if len(hits) > limit {
		hits = hits[:limit]
	}
	results := make([]SearchResult, 0, len(hits))
	for _, h := range hits {
		h.r.Score = h.score
		results = append(results, h.r)
	}
	return results, nil
}

// extractRootDomain 从域名提取主域（如 blog.csdn.net -> csdn.net）
func extractRootDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return domain
}

// splitQueryKeywords 简单中文分词：按空格/标点切分 + 单字切分兜底
func splitQueryKeywords(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	// 按空白和常见标点切分
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ',' || r == '，' || r == ';' || r == '；' ||
			r == '|' || r == '/' || r == '\\' || r == '+' || r == '、'
	})
	var result []string
	seen := make(map[string]bool)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && len(s) <= 32 && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, f := range fields {
		add(f)
	}

	// 如果没有切分（纯中文无空格），或只有一个词条，则生成 2-gram 作为补充
	// 这样 "筷子兄弟" 会同时搜索 "筷子兄弟" + "筷子" + "兄弟"
	if len(result) <= 1 {
		runes := []rune(query)
		// 添加完整查询作为第一个关键词
		if len(runes) > 0 && !seen[query] {
			seen[query] = true
			result = append([]string{query}, result...)
		}
		// 生成 2-gram
		if len(runes) >= 2 {
			for i := 0; i+2 <= len(runes); i++ {
				gram := string(runes[i : i+2])
				add(gram)
			}
		} else if len(runes) == 1 {
			add(query)
		}
	}
	return result
}

// ========== 爬虫种子 ==========

// CrawlerSeed 爬虫种子
type CrawlerSeed struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Enabled   bool   `json:"enabled"`
	BuiltIn   bool   `json:"built_in"`
	CreatedAt string `json:"created_at"`
}

// initDefaultCrawlerSeeds 初始化预置种子
func initDefaultCrawlerSeeds() {
	now := time.Now().Format("2006-01-02 15:04:05")
	defaults := []struct {
		url, name, category string
	}{
		{"https://github.com/trending", "GitHub Trending", "技术"},
		{"https://www.v2ex.com/?tab=hot", "V2EX 热门", "技术"},
		{"https://juejin.cn/", "掘金", "技术"},
		{"https://www.cnblogs.com/", "博客园", "技术"},
		{"https://blog.csdn.net/", "CSDN", "技术"},
		{"https://www.zhihu.com/hot", "知乎热门", "资讯"},
		{"https://news.ycombinator.com/", "Hacker News", "技术"},
		{"https://www.ithome.com/", "IT之家", "资讯"},
		{"https://www.36kr.com/", "36氪", "资讯"},
		{"https://sspai.com/", "少数派", "资讯"},
		{"https://www.infoq.cn/", "InfoQ 中文", "技术"},
		{"https://segmentfault.com/", "SegmentFault", "技术"},
		{"https://www.bilibili.com/v/popular/rank/all", "B站热门", "娱乐"},
		{"https://weibo.com/hot/search", "微博热搜", "资讯"},
		{"https://top.baidu.com/board?tab=realtime", "百度热搜", "资讯"},
		{"https://tophub.today/", "今日热榜", "资讯"},
		{"https://www.douban.com/", "豆瓣", "文化"},
		{"https://book.douban.com/", "豆瓣读书", "文化"},
		{"https://movie.douban.com/", "豆瓣电影", "娱乐"},
		{"https://music.douban.com/", "豆瓣音乐", "娱乐"},
		{"https://www.kuaemba.com/", "快手热榜", "娱乐"},
		{"https://top.baidu.com/board?tab=entertainment", "百度娱乐", "娱乐"},
		{"https://www.sohu.com/category/ent", "搜狐娱乐", "娱乐"},
		{"https://ent.163.com/", "网易娱乐", "娱乐"},
	}
	for _, d := range defaults {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM crawler_seeds WHERE url = ?", d.url).Scan(&count)
		if count == 0 {
			db.Exec(`INSERT INTO crawler_seeds (url, name, category, enabled, built_in, created_at)
				VALUES (?, ?, ?, 1, 1, ?)`, d.url, d.name, d.category, now)
		}
	}
}

// GetCrawlerSeeds 获取种子列表
func GetCrawlerSeeds(enabledOnly bool) ([]CrawlerSeed, error) {
	var rows *sql.Rows
	var err error
	if enabledOnly {
		rows, err = db.Query(`SELECT id, url, name, category, enabled, built_in, created_at
			FROM crawler_seeds WHERE enabled = 1 ORDER BY built_in DESC, id ASC`)
	} else {
		rows, err = db.Query(`SELECT id, url, name, category, enabled, built_in, created_at
			FROM crawler_seeds ORDER BY built_in DESC, id ASC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []CrawlerSeed
	for rows.Next() {
		var s CrawlerSeed
		var enabledInt, builtInInt int
		if err := rows.Scan(&s.ID, &s.URL, &s.Name, &s.Category, &enabledInt, &builtInInt, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.Enabled = enabledInt == 1
		s.BuiltIn = builtInInt == 1
		list = append(list, s)
	}
	return list, nil
}

// AddCrawlerSeed 添加种子
func AddCrawlerSeed(s *CrawlerSeed) (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	enabledInt := 0
	if s.Enabled {
		enabledInt = 1
	}
	result, err := db.Exec(`INSERT OR IGNORE INTO crawler_seeds (url, name, category, enabled, built_in, created_at)
		VALUES (?, ?, ?, ?, 0, ?)`, s.URL, s.Name, s.Category, enabledInt, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateCrawlerSeed 更新种子
func UpdateCrawlerSeed(s *CrawlerSeed) error {
	enabledInt := 0
	if s.Enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`UPDATE crawler_seeds SET url=?, name=?, category=?, enabled=? WHERE id=?`,
		s.URL, s.Name, s.Category, enabledInt, s.ID)
	return err
}

// DeleteCrawlerSeed 删除种子（内置种子不允许删除）
func DeleteCrawlerSeed(id int) error {
	_, err := db.Exec("DELETE FROM crawler_seeds WHERE id = ? AND built_in = 0", id)
	return err
}

// ========== 爬虫任务队列 ==========

// EnqueueCrawlTask 入队一个爬取任务
func EnqueueCrawlTask(url string, depth int, source string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`INSERT OR IGNORE INTO crawler_tasks (url, depth, source, status, created_at)
		VALUES (?, ?, ?, 'pending', ?)`, url, depth, source, now)
	return err
}

// ClaimNextCrawlTask 认领下一个待处理任务
func ClaimNextCrawlTask() (*CrawlTask, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	row := tx.QueryRow(`SELECT id, url, depth, source FROM crawler_tasks
		WHERE status = 'pending' ORDER BY id ASC LIMIT 1`)
	var t CrawlTask
	err = row.Scan(&t.ID, &t.URL, &t.Depth, &t.Source)
	if err == sql.ErrNoRows {
		tx.Rollback()
		return nil, nil
	}
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	if _, err := tx.Exec("UPDATE crawler_tasks SET status='processing', processed_at=? WHERE id=?", now, t.ID); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &t, nil
}

// CompleteCrawlTask 标记任务完成
func CompleteCrawlTask(id int, taskErr string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	status := "done"
	if taskErr != "" {
		status = "failed"
	}
	_, err := db.Exec("UPDATE crawler_tasks SET status=?, error=?, processed_at=? WHERE id=?", status, taskErr, now, id)
	return err
}

// CountPendingCrawlTasks 统计待处理任务数
func CountPendingCrawlTasks() (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM crawler_tasks WHERE status = 'pending'").Scan(&count)
	return count, err
}

// ClearCrawlerTasks 清理已完成/失败的任务（保留最近1000条）
func ClearCrawlerTasks() error {
	_, err := db.Exec(`DELETE FROM crawler_tasks WHERE id NOT IN (
		SELECT id FROM crawler_tasks ORDER BY id DESC LIMIT 1000)`)
	return err
}

// ========== 引擎启停设置 ==========

// EngineSetting 引擎启停设置
type EngineSetting struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	UpdatedAt string `json:"updated_at"`
}

// IsEngineEnabled 判断引擎是否启用（未配置默认启用）
func IsEngineEnabled(name string) bool {
	var enabled int
	err := db.QueryRow("SELECT enabled FROM engine_settings WHERE name = ?", name).Scan(&enabled)
	if err == sql.ErrNoRows {
		return true // 默认启用
	}
	if err != nil {
		return true
	}
	return enabled == 1
}

// SetEngineEnabled 设置引擎启停
func SetEngineEnabled(name string, enabled bool) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`INSERT INTO engine_settings (name, enabled, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET enabled=excluded.enabled, updated_at=excluded.updated_at`,
		name, enabledInt, now)
	return err
}

// GetEngineSettings 获取所有引擎设置
func GetEngineSettings() ([]EngineSetting, error) {
	rows, err := db.Query("SELECT name, enabled, updated_at FROM engine_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []EngineSetting
	for rows.Next() {
		var s EngineSetting
		var enabledInt int
		if err := rows.Scan(&s.Name, &enabledInt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Enabled = enabledInt == 1
		list = append(list, s)
	}
	return list, nil
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

	// 查找是否已存在相同 query 的会话，存在则更新，不存在则插入
	var existingID int64
	err = db.QueryRow("SELECT id FROM search_sessions WHERE query = ? ORDER BY created_at DESC LIMIT 1", query).Scan(&existingID)
	if err == nil && existingID > 0 {
		_, err = db.Exec(
			"UPDATE search_sessions SET results_json = ?, result_count = ?, updated_at = ? WHERE id = ?",
			string(resultsJSON), len(results), now, existingID,
		)
		if err != nil {
			return 0, err
		}
		return existingID, nil
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
