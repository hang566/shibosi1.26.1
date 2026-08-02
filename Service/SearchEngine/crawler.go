package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ================= 配置与状态 =================

// CrawlerConfig 爬虫配置
type CrawlerConfig struct {
	MaxDepth          int  `json:"max_depth"`            // 最大爬取深度
	MaxPages          int  `json:"max_pages"`            // 本次任务最多抓取页数
	Concurrency       int  `json:"concurrency"`          // 并发数
	RequestDelay      int  `json:"request_delay_ms"`     // 同域请求间隔（毫秒）
	Timeout           int  `json:"timeout_ms"`           // HTTP 超时（毫秒）
	SameDomainOnly    bool `json:"same_domain_only"`     // 仅同域爬取
	MaxPagesPerDomain int  `json:"max_pages_per_domain"` // 单域名最多抓取页数
	MaxContentLength  int  `json:"max_content_length"`   // 单页正文最大保存字符数
	FollowExternal    bool `json:"follow_external"`      // 是否跟随外链（默认 false）
}

// defaultCrawlerConfig 默认配置
func defaultCrawlerConfig() CrawlerConfig {
	return CrawlerConfig{
		MaxDepth:          2,
		MaxPages:          100,
		Concurrency:       4,
		RequestDelay:      500,
		Timeout:           10000,
		SameDomainOnly:    true,
		MaxPagesPerDomain: 20,
		MaxContentLength:  20000,
		FollowExternal:    false,
	}
}

// CrawlTask 爬取任务
type CrawlTask struct {
	ID     int    `json:"id"`
	URL    string `json:"url"`
	Depth  int    `json:"depth"`
	Source string `json:"source"`
}

// CrawlerStats 爬虫运行统计
type CrawlerStats struct {
	Running      bool   `json:"running"`
	PagesCrawled int64  `json:"pages_crawled"`
	PagesFailed  int64  `json:"pages_failed"`
	URLsQueued   int64  `json:"urls_queued"`
	StartTime    string `json:"start_time"`
	Elapsed      string `json:"elapsed"`
	CurrentURL   string `json:"current_url"`
	LastCrawlAt  string `json:"last_crawl_at"`
	TotalIndexed int64  `json:"total_indexed"`
	PendingTasks int64  `json:"pending_tasks"`
}

// Crawler 爬虫控制器
type Crawler struct {
	mu        sync.Mutex
	config    CrawlerConfig
	running   atomic.Bool
	stopChan  chan struct{}
	stats     CrawlerStats
	startTime time.Time
	visited   sync.Map // url -> struct{}{}
	domainCnt sync.Map // domain -> *int64
	logger    *Logger
}

// globalCrawler 全局爬虫实例
var globalCrawler = &Crawler{
	config:   defaultCrawlerConfig(),
	stopChan: make(chan struct{}),
	logger:   NewLogger(),
}

// ================= Searcher 接口实现 =================

// CrawlerIndex 本地爬虫索引搜索源
type CrawlerIndex struct{}

func (c *CrawlerIndex) Name() string { return "CrawlerIndex" }

func (c *CrawlerIndex) Search(query string, limit int) ([]SearchResult, error) {
	return SearchCrawlerPages(query, limit)
}

// ================= 爬虫核心 =================

// StartCrawler 启动爬虫任务
// seeds: 自定义种子 URL（为空时从数据库与 indexed_sites 取）
func StartCrawler(cfg CrawlerConfig, customSeeds []string) error {
	if globalCrawler.running.Load() {
		return fmt.Errorf("爬虫正在运行中")
	}
	globalCrawler.mu.Lock()
	globalCrawler.config = cfg
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
		globalCrawler.config.Concurrency = 4
	}
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 100
		globalCrawler.config.MaxPages = 100
	}
	globalCrawler.stopChan = make(chan struct{})
	globalCrawler.visited = sync.Map{}
	globalCrawler.domainCnt = sync.Map{}
	globalCrawler.startTime = time.Now()
	globalCrawler.stats = CrawlerStats{
		Running:   true,
		StartTime: time.Now().Format("2006-01-02 15:04:05"),
	}
	globalCrawler.running.Store(true)
	globalCrawler.mu.Unlock()

	go runCrawler(cfg, customSeeds)
	return nil
}

// StopCrawler 停止爬虫
func StopCrawler() error {
	if !globalCrawler.running.Load() {
		return fmt.Errorf("爬虫未在运行")
	}
	close(globalCrawler.stopChan)
	globalCrawler.running.Store(false)
	globalCrawler.mu.Lock()
	globalCrawler.stats.Running = false
	globalCrawler.mu.Unlock()
	return nil
}

// GetCrawlerStats 获取爬虫状态
func GetCrawlerStats() CrawlerStats {
	globalCrawler.mu.Lock()
	defer globalCrawler.mu.Unlock()
	stats := globalCrawler.stats
	stats.Running = globalCrawler.running.Load()
	if stats.Running {
		stats.Elapsed = formatAdminDuration(time.Since(globalCrawler.startTime))
	}
	total, _ := CountCrawlerPages()
	stats.TotalIndexed = total
	pending, _ := CountPendingCrawlTasks()
	stats.PendingTasks = pending
	return stats
}

// getConfig 获取当前爬虫配置（线程安全）
func (c *Crawler) getConfig() CrawlerConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.config
}

// runCrawler 主循环：从任务队列取任务，并发处理，并将发现的新链接入队
func runCrawler(cfg CrawlerConfig, customSeeds []string) {
	defer func() {
		globalCrawler.running.Store(false)
		globalCrawler.mu.Lock()
		globalCrawler.stats.Running = false
		globalCrawler.mu.Unlock()
		globalCrawler.logger.Info("CRAWLER STOPPED pages=%d failed=%d",
			globalCrawler.stats.PagesCrawled, globalCrawler.stats.PagesFailed)
	}()

	// 1. 入队种子：自定义种子 + 数据库种子 + 已审核 indexed_sites
	seedCount := 0
	for _, s := range customSeeds {
		if s = strings.TrimSpace(s); s != "" {
			if err := EnqueueCrawlTask(s, 0, "custom_seed"); err == nil {
				seedCount++
			}
		}
	}
	if dbSeeds, err := GetCrawlerSeeds(true); err == nil {
		for _, s := range dbSeeds {
			if err := EnqueueCrawlTask(s.URL, 0, "seed:"+s.Name); err == nil {
				seedCount++
			}
		}
	}
	if sites, err := GetAllIndexedSites(true); err == nil {
		for _, s := range sites {
			if err := EnqueueCrawlTask(s.URL, 0, "indexed:"+s.Name); err == nil {
				seedCount++
			}
		}
	}
	globalCrawler.logger.Info("CRAWLER STARTED seeds=%d cfg=%+v", seedCount, cfg)
	atomic.StoreInt64(&globalCrawler.stats.URLsQueued, int64(seedCount))

	// 2. 并发 worker 池
	type result struct {
		task    *CrawlTask
		links   []string
		pageErr string
	}
	resultCh := make(chan result, cfg.Concurrency)
	taskCh := make(chan *CrawlTask, cfg.Concurrency)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听停止信号
	go func() {
		select {
		case <-globalCrawler.stopChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	// 启动 worker
	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				if globalCrawler.running.Load() == false {
					return
				}
				links, err := processCrawlTask(ctx, task, cfg)
				errStr := ""
				if err != nil {
					errStr = err.Error()
				}
				resultCh <- result{task: task, links: links, pageErr: errStr}
			}
		}()
	}

	// 调度协程：从 DB 取任务并投递到 taskCh
	go func() {
		defer close(taskCh)
		pagesCrawled := int64(0)
		for globalCrawler.running.Load() {
			if pagesCrawled >= int64(cfg.MaxPages) {
				globalCrawler.logger.Info("CRAWLER MaxPages reached %d", pagesCrawled)
				break
			}
			task, err := ClaimNextCrawlTask()
			if err != nil {
				globalCrawler.logger.Error("CRAWLER claim task error: %v", err)
				time.Sleep(time.Second)
				continue
			}
			if task == nil {
				// 队列空，等待 worker 处理完入队的新链接
				if len(taskCh) > 0 {
					time.Sleep(200 * time.Millisecond)
					continue
				}
				// 检查是否还有 pending
				pending, _ := CountPendingCrawlTasks()
				if pending == 0 {
					globalCrawler.logger.Info("CRAWLER queue empty, exit")
					break
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			// 跳过已访问
			if _, loaded := globalCrawler.visited.LoadOrStore(task.URL, struct{}{}); loaded {
				CompleteCrawlTask(task.ID, "duplicate")
				continue
			}
			// 单域名限额
			if cfg.MaxPagesPerDomain > 0 {
				d := extractDomain(task.URL)
				if cnt, ok := globalCrawler.domainCnt.Load(d); ok {
					if atomic.LoadInt64(cnt.(*int64)) >= int64(cfg.MaxPagesPerDomain) {
						CompleteCrawlTask(task.ID, "domain_limit")
						continue
					}
				}
			}
			select {
			case taskCh <- task:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 收集结果（处理完成后将新链接入队）
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for r := range resultCh {
		if r.pageErr != "" {
			atomic.AddInt64(&globalCrawler.stats.PagesFailed, 1)
			CompleteCrawlTask(r.task.ID, r.pageErr)
		} else {
			atomic.AddInt64(&globalCrawler.stats.PagesCrawled, 1)
			CompleteCrawlTask(r.task.ID, "")
			// 域名计数 +1
			d := extractDomain(r.task.URL)
			if cnt, ok := globalCrawler.domainCnt.LoadOrStore(d, new(int64)); ok {
				atomic.AddInt64(cnt.(*int64), 1)
			} else {
				atomic.AddInt64(cnt.(*int64), 1)
			}
			// 入队新链接
			if r.task.Depth < cfg.MaxDepth {
				for _, link := range r.links {
					if cfg.SameDomainOnly && !sameDomain(r.task.URL, link) {
						if !cfg.FollowExternal {
							continue
						}
					}
					if _, loaded := globalCrawler.visited.Load(link); !loaded {
						if err := EnqueueCrawlTask(link, r.task.Depth+1, "discovered"); err == nil {
							atomic.AddInt64(&globalCrawler.stats.URLsQueued, 1)
						}
					}
				}
			}
		}
		// 更新当前URL
		globalCrawler.mu.Lock()
		globalCrawler.stats.CurrentURL = r.task.URL
		globalCrawler.stats.PagesCrawled = atomic.LoadInt64(&globalCrawler.stats.PagesCrawled)
		globalCrawler.stats.PagesFailed = atomic.LoadInt64(&globalCrawler.stats.PagesFailed)
		globalCrawler.stats.URLsQueued = atomic.LoadInt64(&globalCrawler.stats.URLsQueued)
		globalCrawler.stats.LastCrawlAt = time.Now().Format("2006-01-02 15:04:05")
		globalCrawler.mu.Unlock()
		// 同域延迟
		if cfg.RequestDelay > 0 {
			time.Sleep(time.Duration(cfg.RequestDelay) * time.Millisecond)
		}
	}

	// 清理任务队列历史
	ClearCrawlerTasks()
}

// processCrawlTask 处理单个爬取任务：抓取、解析、保存页面、返回新链接
func processCrawlTask(ctx context.Context, task *CrawlTask, cfg CrawlerConfig) ([]string, error) {
	// URL 合法性
	parsed, err := url.Parse(task.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid url")
	}

	// robots.txt 检查（简单缓存：内存中记录被禁止的路径）
	if !robotsAllowed(parsed) {
		return nil, fmt.Errorf("disallowed by robots.txt")
	}

	client := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", task.URL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req)
	// 注意：不要手动设置 Accept-Encoding，Go http.Client 会自动处理 gzip 解压
	// 仅抓 HTML
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	// 检查 Content-Type
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "html") {
		return nil, fmt.Errorf("non-html content-type: %s", ct)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 限制最大响应体 5MB
	if len(body) > 5*1024*1024 {
		body = body[:5*1024*1024]
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	// 提取标题：优先 og:title，再 <title>，再 h1，最后域名
	title := ""
	if ogTitle, ok := doc.Find(`meta[property="og:title"]`).First().Attr("content"); ok {
		title = strings.TrimSpace(ogTitle)
	}
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	if title == "" {
		title = parsed.Host
	}
	// 清理标题中的多余空白
	title = collapseWhitespace(title)
	if len(title) > 200 {
		title = title[:200]
	}

	// 提取摘要：优先 meta description，其次 og:description，其次从正文取前 200 字
	snippet := ""
	if desc, ok := doc.Find(`meta[name="description"]`).First().Attr("content"); ok {
		snippet = strings.TrimSpace(desc)
	}
	if snippet == "" {
		if desc, ok := doc.Find(`meta[property="og:description"]`).First().Attr("content"); ok {
			snippet = strings.TrimSpace(desc)
		}
	}

	// 提取正文
	content := extractContent(doc)
	maxLen := cfg.MaxContentLength
	if maxLen <= 0 {
		maxLen = 20000
	}
	if len(content) > maxLen {
		content = content[:maxLen]
	}

	// 摘要兜底：从正文取前 200 字作为摘要
	if snippet == "" {
		runes := []rune(content)
		if len(runes) > 200 {
			snippet = string(runes[:200]) + "..."
		} else {
			snippet = string(runes)
		}
	}
	if len(snippet) > 300 {
		snippet = snippet[:300] + "..."
	}

	// 内容质量检查：正文太短则标记为未索引，避免噪音污染搜索
	contentRunes := []rune(content)
	contentLen := len(contentRunes)
	contentQuality := "good"
	if contentLen < 50 {
		contentQuality = "too_short"
	} else if contentLen < 150 {
		// 检查是否主要是标题 + 元数据，而非正文
		lowerContent := strings.ToLower(content)
		noiseKeywords := []string{"首页", "导航", "menu", "nav", "copyright", "登录", "注册", "关于我们"}
		noiseCount := 0
		for _, nk := range noiseKeywords {
			if strings.Contains(lowerContent, nk) {
				noiseCount++
			}
		}
		if noiseCount >= 3 || contentLen < 80 {
			contentQuality = "too_noisy"
		}
	}

	// 保存页面
	domain := extractDomain(task.URL)
	page := &CrawlerPage{
		URL:           task.URL,
		Title:         title,
		Content:       content,
		Snippet:       snippet,
		Domain:        domain,
		Source:        task.Source,
		Depth:         task.Depth,
		StatusCode:    resp.StatusCode,
		ContentLength: len(body),
		Indexed:       contentQuality == "good",
	}
	if err := SaveCrawlerPage(page); err != nil {
		return nil, fmt.Errorf("save page: %v", err)
	}

	// 提取新链接（仅同域且为 http/https）
	var links []string
	baseURL := parsed
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if len(links) >= 50 {
			return
		}
		href, _ := s.Attr("href")
		if href == "" {
			return
		}
		// 解析相对链接
		absURL := resolveURL(baseURL, href)
		if absURL == "" {
			return
		}
		// 去除 fragment
		if idx := strings.Index(absURL, "#"); idx > 0 {
			absURL = absURL[:idx]
		}
		// 仅 http/https
		if !strings.HasPrefix(absURL, "http://") && !strings.HasPrefix(absURL, "https://") {
			return
		}
		// 跳过非页面资源
		if isLikelyAssetURL(absURL) {
			return
		}
		links = append(links, absURL)
	})

	return links, nil
}

// extractContent 从 goquery 文档提取正文文本（改进版：文本密度分析 + 噪音过滤）
func extractContent(doc *goquery.Document) string {
	// 移除无关元素
	doc.Find("script,style,noscript,iframe,nav,header,footer,aside,form,button,svg,canvas,video,audio,select,option").Remove()
	// 移除隐藏元素
	doc.Find("[style*='display:none'],[style*='display: none'],[hidden]").Remove()
	// 移除常见噪音 class/id
	doc.Find("[class*='comment'],[class*='sidebar'],[class*='widget'],[class*='ad-'],[class*='advertisement'],[class*='recommend'],[class*='related'],[class*='copyright'],[class*='footer'],[class*='header'],[class*='menu'],[class*='nav'],[class*='breadcrumb'],[id*='comment'],[id*='sidebar'],[id*='widget'],[id*='ad-'],[id*='footer'],[id*='header'],[id*='menu'],[id*='nav']").Remove()
	// 移除空标签
	doc.Find("div,p,span,section").Each(func(_ int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "" {
			s.Remove()
		}
	})

	// 优先取常见内容容器
	contentSelectors := []string{
		"article", "main",
		"#content", "#article", "#post", "#main-content",
		".content", ".post-content", ".article-content", ".entry-content",
		".post-body", ".article-body", ".main-content", ".post-text",
		".rich_media_content", ".rich-content", // 微信公众号
		".article-content", ".article-detail", // 通用 CMS
	}
	var contentText string
	for _, sel := range contentSelectors {
		if s := doc.Find(sel).First(); s.Length() > 0 {
			candidate := strings.TrimSpace(s.Text())
			// 文本密度检测：至少 100 个有效字符才算有效内容
			if len(candidate) >= 100 {
				contentText = candidate
				break
			}
		}
	}
	if strings.TrimSpace(contentText) == "" {
		// 回退到 body：使用文本密度分析，只取内容最密集的区域
		contentText = extractBodyWithDensity(doc)
	}

	// 清理多余空白
	contentText = collapseWhitespace(contentText)
	contentText = strings.TrimSpace(contentText)

	// 移除常见噪音文本行
	noiseLines := []string{
		"相关推荐", "相关文章", "推荐阅读", "推荐文章", "热门文章",
		"上一篇", "下一篇", "返回首页", "返回顶部", "分享到",
		"评论", "发表评论", "全部评论", "热门评论", "最新评论",
		"版权声明", "转载", "免责声明", "本文链接", "原文链接",
		"关注我们", "扫码关注", "微信扫一扫", "订阅", "投稿",
		"广告", "赞助", "推广",
		"Copyright", "All Rights Reserved", "All rights reserved",
		"Privacy Policy", "Terms of Service", "Contact Us",
		"Follow us", "Subscribe", "Share this",
	}
	lines := strings.Split(contentText, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		isNoise := false
		for _, n := range noiseLines {
			if strings.Contains(trimmed, n) {
				isNoise = true
				break
			}
		}
		if !isNoise {
			cleanLines = append(cleanLines, trimmed)
		}
	}
	contentText = strings.Join(cleanLines, "\n")

	return contentText
}

// extractBodyWithDensity 通过文本密度分析从 body 提取最密集的文本区域
// 将 body 按块级元素分割，取文本密度最高的连续块序列
func extractBodyWithDensity(doc *goquery.Document) string {
	// 按常见的块级元素分割
	var blocks []string
	doc.Find("body > *").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) < 20 {
			return
		}
		// 计算文本密度：字符数 / 标签数
		html, _ := s.Html()
		tagCount := len(regexp.MustCompile(`<[^>]+>`).FindAllString(html, -1))
		if tagCount <= 0 {
			tagCount = 1
		}
		density := float64(len(text)) / float64(tagCount)
		// 密度阈值：文本密集的区域密度高，反之噪音多
		if density >= 15.0 {
			blocks = append(blocks, text)
		}
	})

	if len(blocks) > 0 {
		return strings.Join(blocks, "\n")
	}
	// 兜底：直接取 body 文本
	return doc.Find("body").Text()
}

// collapseWhitespace 合并多个连续空白为一个空格
var whitespaceRe = regexp.MustCompile(`\s+`)

func collapseWhitespace(s string) string {
	return whitespaceRe.ReplaceAllString(s, " ")
}

// resolveURL 解析相对 URL 为绝对 URL
func resolveURL(base *url.URL, href string) string {
	if href == "" {
		return ""
	}
	rel, err := url.Parse(href)
	if err != nil {
		return ""
	}
	abs := base.ResolveReference(rel)
	return abs.String()
}

// sameDomain 判断两个 URL 是否同域
func sameDomain(u1, u2 string) bool {
	d1 := extractDomain(u1)
	d2 := extractDomain(u2)
	if d1 == "" || d2 == "" {
		return false
	}
	// 子域也算同域：blog.csdn.net 与 csdn.net 算同域
	if d1 == d2 {
		return true
	}
	if strings.HasSuffix(d1, "."+d2) || strings.HasSuffix(d2, "."+d1) {
		return true
	}
	return false
}

// isLikelyAssetURL 根据扩展名判断是否为静态资源
func isLikelyAssetURL(u string) bool {
	lower := strings.ToLower(u)
	exts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico",
		".css", ".js", ".mjs", ".woff", ".woff2", ".ttf", ".eot",
		".mp4", ".mp3", ".webm", ".ogg", ".wav", ".avi", ".mov",
		".zip", ".rar", ".7z", ".tar", ".gz", ".pdf", ".doc", ".docx",
		".xls", ".xlsx", ".ppt", ".pptx", ".exe", ".dmg", ".apk",
		".json", ".xml", ".rss", ".atom"}
	for _, e := range exts {
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	// 包含常见资源路径
	if strings.Contains(lower, "/static/") || strings.Contains(lower, "/assets/") ||
		strings.Contains(lower, "/cdn/") || strings.Contains(lower, "/wp-content/uploads/") {
		return true
	}
	return false
}

// ================= robots.txt 简易实现 =================

type robotsCacheEntry struct {
	allowAll bool
	disallow []string
	loadedAt time.Time
}

var (
	robotsMu    sync.Mutex
	robotsCache = make(map[string]*robotsCacheEntry)
)

// robotsAllowed 简单解析 robots.txt 并判断是否允许抓取
func robotsAllowed(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := u.Host
	robotsMu.Lock()
	entry, ok := robotsCache[host]
	robotsMu.Unlock()
	if !ok || time.Since(entry.loadedAt) > 6*time.Hour {
		entry = loadRobots(u.Scheme, host)
		robotsMu.Lock()
		robotsCache[host] = entry
		robotsMu.Unlock()
	}
	if entry == nil || entry.allowAll {
		return true
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	for _, d := range entry.disallow {
		if d == "" {
			continue
		}
		if strings.HasPrefix(path, d) {
			return false
		}
	}
	return true
}

// loadRobots 加载并解析 robots.txt
func loadRobots(scheme, host string) *robotsCacheEntry {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", scheme, host)
	client := &http.Client{Timeout: 4 * time.Second}
	req, err := http.NewRequest("GET", robotsURL, nil)
	if err != nil {
		return &robotsCacheEntry{allowAll: true, loadedAt: time.Now()}
	}
	setCommonHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return &robotsCacheEntry{allowAll: true, loadedAt: time.Now()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &robotsCacheEntry{allowAll: true, loadedAt: time.Now()}
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &robotsCacheEntry{allowAll: true, loadedAt: time.Now()}
	}

	entry := &robotsCacheEntry{loadedAt: time.Now()}
	// 解析 User-agent: * 段
	lines := strings.Split(string(body), "\n")
	inAgentStar := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "user-agent:") {
			agent := strings.TrimSpace(strings.TrimPrefix(lower, "user-agent:"))
			inAgentStar = (agent == "*")
			continue
		}
		if strings.HasPrefix(lower, "disallow:") {
			if !inAgentStar {
				continue
			}
			path := strings.TrimSpace(strings.TrimPrefix(line, "Disallow:"))
			path = strings.TrimSpace(strings.TrimPrefix(path, "disallow:"))
			if path == "" {
				// Disallow: 表示允许全部
				entry.allowAll = true
				entry.disallow = nil
				return entry
			}
			entry.disallow = append(entry.disallow, path)
		}
		if strings.HasPrefix(lower, "allow:") && inAgentStar {
			// 简单处理：忽略 Allow 规则（保守拒绝）
		}
	}
	return entry
}

// ================= 调试辅助 =================

// normalizeURL 规范化 URL（去除 fragment、统一大小写 scheme/host）
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	// 去除默认端口
	if u.Host != "" {
		host := u.Host
		if strings.HasSuffix(host, ":80") && u.Scheme == "http" {
			u.Host = strings.TrimSuffix(host, ":80")
		}
		if strings.HasSuffix(host, ":443") && u.Scheme == "https" {
			u.Host = strings.TrimSuffix(host, ":443")
		}
	}
	// 规范化路径（去除 ./ ../）
	u.Path = path.Clean(u.Path)
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
