package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type SearchResult struct {
	Title       string   `json:"Title"`
	URL         string   `json:"URL"`
	Snippet     string   `json:"Snippet"`
	Source      string   `json:"Source"`
	Sources     []string `json:"Sources,omitempty"`
	ResultCount int      `json:"ResultCount,omitempty"`
	Score       float64  `json:"-"`
}

type Searcher interface {
	Search(query string, limit int) ([]SearchResult, error)
	Name() string
}

// 搜索引擎权重
var engineWeights = map[string]float64{
	"Google":       1.0,
	"Bing":         0.9,
	"Baidu":        0.85,
	"DuckDuckGo":   0.8,
	"Sogou":        0.7,
	"CrawlerIndex": 0.95, // 本地爬虫索引：内容完整、可信度高
}

// BrandOfficialEntry 品牌官方域名条目
type BrandOfficialEntry struct {
	Domain  string   // 官方域名
	Aliases []string // 搜索关键词（小写）
}

// 品牌官方域名数据库 - 搜索品牌名时优先展示官方网站
var brandOfficialDomains = map[string][]BrandOfficialEntry{
	"火绒": {
		BrandOfficialEntry{Domain: "huorong.cn", Aliases: []string{"火绒", "huorong", "huorong.cn"}},
		BrandOfficialEntry{Domain: "huorong.com", Aliases: []string{"火绒", "huorong"}},
	},
	"360": {
		BrandOfficialEntry{Domain: "360.cn", Aliases: []string{"360", "360安全", "360杀毒", "360卫士"}},
		BrandOfficialEntry{Domain: "360.com", Aliases: []string{"360", "360安全", "360杀毒", "360卫士"}},
	},
	"腾讯": {
		BrandOfficialEntry{Domain: "qq.com", Aliases: []string{"腾讯", "qq", "tencent"}},
		BrandOfficialEntry{Domain: "tencent.com", Aliases: []string{"腾讯", "tencent"}},
	},
	"阿里": {
		BrandOfficialEntry{Domain: "alibaba.com", Aliases: []string{"阿里巴巴", "alibaba", "阿里"}},
		BrandOfficialEntry{Domain: "taobao.com", Aliases: []string{"淘宝", "taobao"}},
		BrandOfficialEntry{Domain: "tmall.com", Aliases: []string{"天猫", "tmall"}},
	},
	"百度": {
		BrandOfficialEntry{Domain: "baidu.com", Aliases: []string{"百度", "baidu"}},
	},
	"华为": {
		BrandOfficialEntry{Domain: "huawei.com", Aliases: []string{"华为", "huawei"}},
		BrandOfficialEntry{Domain: "vmall.com", Aliases: []string{"华为商城", "vmall"}},
	},
	"小米": {
		BrandOfficialEntry{Domain: "mi.com", Aliases: []string{"小米", "xiaomi", "mi"}},
		BrandOfficialEntry{Domain: "xiaomi.com", Aliases: []string{"小米", "xiaomi"}},
	},
	"京东": {
		BrandOfficialEntry{Domain: "jd.com", Aliases: []string{"京东", "jd", "jingdong"}},
	},
	"美团": {
		BrandOfficialEntry{Domain: "meituan.com", Aliases: []string{"美团", "meituan"}},
	},
	"滴滴": {
		BrandOfficialEntry{Domain: "didi.cn", Aliases: []string{"滴滴", "didi"}},
	},
	"网易": {
		BrandOfficialEntry{Domain: "163.com", Aliases: []string{"网易", "netease", "163"}},
		BrandOfficialEntry{Domain: "netease.com", Aliases: []string{"网易", "netease"}},
	},
	"新浪": {
		BrandOfficialEntry{Domain: "sina.com.cn", Aliases: []string{"新浪", "sina"}},
	},
	"微软": {
		BrandOfficialEntry{Domain: "microsoft.com", Aliases: []string{"微软", "microsoft", "ms"}},
		BrandOfficialEntry{Domain: "office.com", Aliases: []string{"office", "office.com"}},
		BrandOfficialEntry{Domain: "windows.com", Aliases: []string{"windows", "windows.com"}},
	},
	"苹果": {
		BrandOfficialEntry{Domain: "apple.com", Aliases: []string{"苹果", "apple", "iphone", "ipad", "mac"}},
	},
	"谷歌": {
		BrandOfficialEntry{Domain: "google.com", Aliases: []string{"谷歌", "google", "chrome"}},
	},
	"Chrome": {
		BrandOfficialEntry{Domain: "chrome.google.com", Aliases: []string{"chrome", "chrome浏览器"}},
	},
	"Edge": {
		BrandOfficialEntry{Domain: "microsoft.com", Aliases: []string{"edge", "edge浏览器"}},
	},
	"Firefox": {
		BrandOfficialEntry{Domain: "mozilla.org", Aliases: []string{"firefox", "火狐", "mozilla"}},
	},
	"VSCode": {
		BrandOfficialEntry{Domain: "code.visualstudio.com", Aliases: []string{"vscode", "vs code", "visual studio code"}},
	},
	"GitHub": {
		BrandOfficialEntry{Domain: "github.com", Aliases: []string{"github", "gitlab"}},
	},
	"GitLab": {
		BrandOfficialEntry{Domain: "gitlab.com", Aliases: []string{"gitlab"}},
	},
	"StackOverflow": {
		BrandOfficialEntry{Domain: "stackoverflow.com", Aliases: []string{"stackoverflow", "so", "栈溢出"}},
	},
	"CSDN": {
		BrandOfficialEntry{Domain: "csdn.net", Aliases: []string{"csdn"}},
	},
	"知乎": {
		BrandOfficialEntry{Domain: "zhihu.com", Aliases: []string{"知乎", "zhihu"}},
	},
	"B站": {
		BrandOfficialEntry{Domain: "bilibili.com", Aliases: []string{"b站", "bilibili", "哔哩哔哩"}},
	},
	"抖音": {
		BrandOfficialEntry{Domain: "douyin.com", Aliases: []string{"抖音", "douyin"}},
	},
	"快手": {
		BrandOfficialEntry{Domain: "kuaishou.com", Aliases: []string{"快手", "kuaishou"}},
	},
	"微博": {
		BrandOfficialEntry{Domain: "weibo.com", Aliases: []string{"微博", "weibo"}},
	},
	"优酷": {
		BrandOfficialEntry{Domain: "youku.com", Aliases: []string{"优酷", "youku"}},
	},
	"爱奇艺": {
		BrandOfficialEntry{Domain: "iqiyi.com", Aliases: []string{"爱奇艺", "iqiyi"}},
	},
	"腾讯视频": {
		BrandOfficialEntry{Domain: "v.qq.com", Aliases: []string{"腾讯视频", "qq视频", "v.qq"}},
	},
	"WPS": {
		BrandOfficialEntry{Domain: "wps.cn", Aliases: []string{"wps", "wps office"}},
		BrandOfficialEntry{Domain: "kingsoft.com", Aliases: []string{"金山", "kingsoft", "wps"}},
	},
	"Office": {
		BrandOfficialEntry{Domain: "office.com", Aliases: []string{"office", "microsoft office"}},
	},
	"Adobe": {
		BrandOfficialEntry{Domain: "adobe.com", Aliases: []string{"adobe", "acrobat", "photoshop", "ps"}},
	},
	"Zoom": {
		BrandOfficialEntry{Domain: "zoom.us", Aliases: []string{"zoom", "zoom会议"}},
	},
	"钉钉": {
		BrandOfficialEntry{Domain: "dingtalk.com", Aliases: []string{"钉钉", "dingtalk"}},
	},
	"飞书": {
		BrandOfficialEntry{Domain: "feishu.cn", Aliases: []string{"飞书", "feishu", "lark"}},
	},
	"企业微信": {
		BrandOfficialEntry{Domain: "work.weixin.qq.com", Aliases: []string{"企业微信", "wecom", "企业wx"}},
	},
}

// 可疑域名特征 - 用于检测钓鱼网站
var suspiciousDomainPatterns = []struct {
	Pattern string  // 可疑特征
	Weight  float64 // 惩罚权重
	Desc    string  // 描述
}{
	{Pattern: "-", Weight: 0.1, Desc: "域名含连字符（常见于钓鱼站）"},
	{Pattern: "www2.", Weight: 0.3, Desc: "www2前缀（可疑）"},
	{Pattern: "www3.", Weight: 0.3, Desc: "www3前缀（可疑）"},
	{Pattern: "security.", Weight: 0.2, Desc: "含security子域（钓鱼常用）"},
	{Pattern: "login.", Weight: 0.15, Desc: "含login子域（钓鱼常用）"},
	{Pattern: "verify.", Weight: 0.2, Desc: "含verify子域（钓鱼常用）"},
	{Pattern: "account.", Weight: 0.15, Desc: "含account子域（钓鱼常用）"},
	{Pattern: "update.", Weight: 0.15, Desc: "含update子域（钓鱼常用）"},
	{Pattern: "download.", Weight: 0.1, Desc: "含download子域（需谨慎）"},
}

// 检查域名是否为品牌官方域名
func isBrandOfficialDomain(domain, query string) (bool, string) {
	domainLower := strings.ToLower(domain)
	queryLower := strings.ToLower(query)

	// 先检查域名是否匹配某个官方域名
	for brand, entries := range brandOfficialDomains {
		for _, entry := range entries {
			if domainLower == strings.ToLower(entry.Domain) || strings.HasSuffix(domainLower, "."+strings.ToLower(entry.Domain)) {
				// 域名匹配，再检查查询词是否关联
				for _, alias := range entry.Aliases {
					if strings.Contains(queryLower, strings.ToLower(alias)) {
						return true, brand
					}
				}
				// 即使查询词不完全匹配，官方域名也应该加分
				return true, brand
			}
		}
	}
	return false, ""
}

// 检查域名是否可疑
func getSuspiciousScore(domain string) float64 {
	domainLower := strings.ToLower(domain)
	penalty := 0.0

	for _, pattern := range suspiciousDomainPatterns {
		if strings.Contains(domainLower, strings.ToLower(pattern.Pattern)) {
			penalty += pattern.Weight
		}
	}

	// 检查是否仿冒知名品牌域名（域名包含品牌关键词但不是官方域名）
	brandKeywords := []string{"huorong", "360", "qq", "taobao", "tmall", "baidu", "huawei", "xiaomi", "jd", "meituan", "didi", "163", "sina", "microsoft", "apple", "google", "chrome", "github", "csdn", "zhihu", "bilibili", "douyin", "weibo", "youku", "iqiyi", "wps", "adobe", "zoom", "dingtalk", "feishu"}
	for _, kw := range brandKeywords {
		if strings.Contains(domainLower, kw) {
			// 包含品牌关键词但不是官方域名，可能是仿冒
			isOfficial := false
			for _, entries := range brandOfficialDomains {
				for _, entry := range entries {
					if strings.Contains(strings.ToLower(entry.Domain), kw) && (domainLower == strings.ToLower(entry.Domain) || strings.HasSuffix(domainLower, "."+strings.ToLower(entry.Domain))) {
						isOfficial = true
						break
					}
				}
				if isOfficial {
					break
				}
			}
			if !isOfficial {
				penalty += 0.3
				break
			}
		}
	}

	// 检查是否为临时/免费域名后缀
	suspiciousTLDs := []string{".tk", ".ml", ".ga", ".cf", ".gq", ".xyz", ".top", ".club", ".site", ".online", "temp", ".ru", ".cn"}
	for _, tld := range suspiciousTLDs {
		if strings.HasSuffix(domainLower, tld) && !strings.HasSuffix(domainLower, ".cn") {
			penalty += 0.15
			break
		}
	}

	return penalty
}

type MultiSearcher struct {
	searchers []Searcher
	cache     *SearchCache
	logger    *Logger
}

func NewMultiSearcher(searchers ...Searcher) *MultiSearcher {
	return &MultiSearcher{
		searchers: searchers,
		cache:     NewSearchCache(2 * time.Minute),
		logger:    NewLogger(),
	}
}

func (m *MultiSearcher) Search(query string, limit int, forceRefresh bool) ([]SearchResult, error) {
	cacheKey := fmt.Sprintf("%s_%d", query, limit)
	if !forceRefresh {
		if cached, ok := m.cache.Get(cacheKey); ok {
			m.logger.Info("CACHE HIT query=%q", query)
			return cached, nil
		}
	}

	var wg sync.WaitGroup
	resultsChan := make(chan []SearchResult, len(m.searchers))
	var mu sync.Mutex
	engineResults := make(map[string]int)

	for _, s := range m.searchers {
		// 跳过已禁用的引擎
		if !IsEngineEnabled(s.Name()) {
			m.logger.Info("ENGINE DISABLED, skip %s query=%q", s.Name(), query)
			continue
		}
		wg.Add(1)
		go func(searcher Searcher) {
			defer wg.Done()
			start := time.Now()

			time.Sleep(time.Duration(100) * time.Millisecond)

			results, err := searcher.Search(query, limit)
			duration := time.Since(start)

			m.logger.SearchLog(query, searcher.Name(), len(results), duration, err)

			if err == nil && len(results) > 0 {
				resultsChan <- results
				mu.Lock()
				engineResults[searcher.Name()] = len(results)
				mu.Unlock()
			}
		}(s)
	}

	wg.Wait()
	close(resultsChan)

	var allResults []SearchResult
	urlIndexMap := make(map[string]int)

	for results := range resultsChan {
		for _, r := range results {
			if r.URL == "" {
				continue
			}

			existingIdx, exists := urlIndexMap[r.URL]
			if !exists {
				r.Sources = []string{r.Source}
				r.ResultCount = 1
				r.Score = calculateScore(r, query, 1)
				allResults = append(allResults, r)
				urlIndexMap[r.URL] = len(allResults) - 1
			} else {
				existing := &allResults[existingIdx]
				alreadyHasSource := false
				for _, src := range existing.Sources {
					if src == r.Source {
						alreadyHasSource = true
						break
					}
				}
				if !alreadyHasSource {
					existing.Sources = append(existing.Sources, r.Source)
				}
				existing.ResultCount++
				if len(r.Snippet) > len(existing.Snippet) {
					existing.Snippet = r.Snippet
				}
				existing.Score = calculateScore(*existing, query, existing.ResultCount)
			}
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// 域名多样性去重：每个主域最多 3 条
	allResults = applyDomainDiversity(allResults, 3)

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	m.cache.Set(cacheKey, allResults)

	m.logger.Info("SEARCH COMPLETE query=%q total=%d engines=%v",
		query, len(allResults), engineResults)

	return allResults, nil
}

func calculateScore(result SearchResult, query string, urlCount int) float64 {
	score := 0.0

	if len(result.Sources) > 0 {
		totalWeight := 0.0
		for _, src := range result.Sources {
			if w, ok := engineWeights[src]; ok {
				totalWeight += w
			} else {
				totalWeight += 0.5
			}
		}
		score += (totalWeight / float64(len(result.Sources))) * 0.4
	} else {
		if weight, ok := engineWeights[result.Source]; ok {
			score += weight * 0.4
		}
	}

	queryLower := strings.ToLower(query)
	titleLower := strings.ToLower(result.Title)
	if strings.Contains(titleLower, queryLower) {
		score += 0.3
		if titleLower == queryLower {
			score += 0.1
		}
	}

	snippetLower := strings.ToLower(result.Snippet)
	if strings.Contains(snippetLower, queryLower) {
		score += 0.2
	}

	// 提取域名进行可信度评估
	domain := extractDomain(result.URL)

	// 1. 品牌官方域名大幅加分
	isOfficial, brandName := isBrandOfficialDomain(domain, query)
	if isOfficial {
		score += 0.5
		// 如果是精确匹配品牌查询，额外加分
		queryTrimmed := strings.TrimSpace(queryLower)
		if strings.Contains(queryTrimmed, strings.ToLower(brandName)) {
			score += 0.3
		}
	}

	// 2. 可信域名加分
	trustedDomains := []string{"wikipedia.org", "github.com", "stackoverflow.com",
		"zhihu.com", "csdn.net", "baidu.com", "gov.cn", "edu.cn"}
	for _, tDomain := range trustedDomains {
		if strings.Contains(domain, tDomain) {
			score += 0.15
			break
		}
	}

	// 3. 检测可疑域名并降权
	suspiciousPenalty := getSuspiciousScore(domain)
	if suspiciousPenalty > 0 {
		score -= suspiciousPenalty
	}

	// 4. HTTPS 加分
	if strings.HasPrefix(result.URL, "https://") {
		score += 0.05
	}

	// 5. 多引擎收录加分
	if urlCount > 1 {
		score += float64(urlCount-1) * 0.25
	}

	// 确保分数不为负
	if score < 0 {
		score = 0
	}

	return score
}

// MergeResults 合并多组搜索结果，按 URL 去重，合并来源，提高权重
func MergeResults(query string, resultGroups ...[]SearchResult) []SearchResult {
	var merged []SearchResult
	urlIndexMap := make(map[string]int)

	for _, group := range resultGroups {
		for _, r := range group {
			if r.URL == "" {
				continue
			}

			existingIdx, exists := urlIndexMap[r.URL]
			if !exists {
				sources := r.Sources
				if len(sources) == 0 {
					sources = []string{r.Source}
				}
				r.Sources = sources
				if r.ResultCount == 0 {
					r.ResultCount = 1
				}
				r.Score = calculateScore(r, query, r.ResultCount)
				merged = append(merged, r)
				urlIndexMap[r.URL] = len(merged) - 1
			} else {
				existing := &merged[existingIdx]

				// 合并来源
				hasSource := false
				for _, src := range existing.Sources {
					if src == r.Source {
						hasSource = true
						break
					}
				}
				if !hasSource {
					existing.Sources = append(existing.Sources, r.Source)
				}

				// 合并 Sources 数组
				for _, s := range r.Sources {
					found := false
					for _, es := range existing.Sources {
						if es == s {
							found = true
							break
						}
					}
					if !found {
						existing.Sources = append(existing.Sources, s)
					}
				}

				existing.ResultCount++

				// 保留更好的标题和摘要
				if len(r.Title) > len(existing.Title) {
					existing.Title = r.Title
				}
				if len(r.Snippet) > len(existing.Snippet) {
					existing.Snippet = r.Snippet
				}

				// 提高权重：每多一个来源加 0.3 分
				existing.Score = calculateScore(*existing, query, existing.ResultCount)
			}
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	// 域名多样性去重
	merged = applyDomainDiversity(merged, 3)

	return merged
}

// applyDomainDiversity 对搜索结果做域名多样性过滤，每个主域最多 maxPerDomain 条
func applyDomainDiversity(results []SearchResult, maxPerDomain int) []SearchResult {
	domainCount := make(map[string]int)
	var filtered []SearchResult
	for _, r := range results {
		rootDomain := extractRootDomainFromURL(r.URL)
		if rootDomain == "" {
			filtered = append(filtered, r)
			continue
		}
		if domainCount[rootDomain] >= maxPerDomain {
			continue
		}
		domainCount[rootDomain]++
		filtered = append(filtered, r)
	}
	return filtered
}

// extractRootDomainFromURL 从 URL 提取主域
func extractRootDomainFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		// 简单 fallback
		parts := strings.Split(rawURL, "/")
		if len(parts) >= 3 {
			host := parts[2]
			dotParts := strings.Split(host, ".")
			if len(dotParts) >= 2 {
				return dotParts[len(dotParts)-2] + "." + dotParts[len(dotParts)-1]
			}
			return host
		}
		return ""
	}
	host := u.Hostname()
	dotParts := strings.Split(host, ".")
	if len(dotParts) >= 2 {
		return dotParts[len(dotParts)-2] + "." + dotParts[len(dotParts)-1]
	}
	return host
}

// newHTTPClient 创建带超时的HTTP客户端
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second,
	}
}

// setCommonHeaders 设置通用请求头
func setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

type DuckDuckGo struct{}

func (d *DuckDuckGo) Name() string {
	return "DuckDuckGo"
}

func (d *DuckDuckGo) Search(query string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	baseURL := "https://html.duckduckgo.com/html/"
	params := url.Values{}
	params.Add("q", query)

	fullURL := baseURL + "?" + params.Encode()

	client := newHTTPClient()
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setCommonHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo returned status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if i >= limit {
			return
		}

		title := s.Find(".result__title").Text()
		linkTag := s.Find(".result__a")
		href, _ := linkTag.Attr("href")
		snippet := s.Find(".result__snippet").Text()

		if href != "" && strings.HasPrefix(href, "http") {
			results = append(results, SearchResult{
				Title:   strings.TrimSpace(title),
				URL:     href,
				Snippet: strings.TrimSpace(snippet),
				Source:  "DuckDuckGo",
			})
		}
	})

	return results, nil
}

type Bing struct{}

func (b *Bing) Name() string {
	return "Bing"
}

func (b *Bing) Search(query string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	baseURL := "https://www.bing.com/search"
	params := url.Values{}
	params.Add("q", query)
	params.Add("count", fmt.Sprintf("%d", limit))

	fullURL := baseURL + "?" + params.Encode()

	client := newHTTPClient()
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setCommonHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bing returned status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	doc.Find("#b_results .b_algo").Each(func(i int, s *goquery.Selection) {
		if i >= limit {
			return
		}

		titleTag := s.Find("h2 a")
		title := titleTag.Text()
		href, _ := titleTag.Attr("href")
		snippet := s.Find(".b_caption p").Text()

		if href != "" && strings.HasPrefix(href, "http") {
			results = append(results, SearchResult{
				Title:   strings.TrimSpace(title),
				URL:     href,
				Snippet: strings.TrimSpace(snippet),
				Source:  "Bing",
			})
		}
	})

	return results, nil
}

// Baidu 百度搜索引擎
type Baidu struct{}

func (b *Baidu) Name() string {
	return "Baidu"
}

func (b *Baidu) Search(query string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	baseURL := "https://www.baidu.com/s"
	params := url.Values{}
	params.Add("wd", query)
	params.Add("rn", fmt.Sprintf("%d", limit))

	fullURL := baseURL + "?" + params.Encode()

	client := newHTTPClient()
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setCommonHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Baidu returned status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	doc.Find(".result, .c-container").Each(func(i int, s *goquery.Selection) {
		if i >= limit {
			return
		}

		titleTag := s.Find("h3 a")
		title := titleTag.Text()
		href, _ := titleTag.Attr("href")
		snippet := s.Find(".c-abstract, [class*='content-right']").Text()

		if href != "" && strings.HasPrefix(href, "http") {
			results = append(results, SearchResult{
				Title:   strings.TrimSpace(title),
				URL:     href,
				Snippet: strings.TrimSpace(snippet),
				Source:  "Baidu",
			})
		}
	})

	return results, nil
}

// Sogou 搜狗搜索引擎
type Sogou struct{}

func (s *Sogou) Name() string {
	return "Sogou"
}

func (s *Sogou) Search(query string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	baseURL := "https://www.sogou.com/web"
	params := url.Values{}
	params.Add("query", query)
	params.Add("num", fmt.Sprintf("%d", limit))

	fullURL := baseURL + "?" + params.Encode()

	client := newHTTPClient()
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setCommonHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Sogou returned status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	doc.Find(".results .vrwrap, .results .rb").Each(func(i int, s *goquery.Selection) {
		if i >= limit {
			return
		}

		titleTag := s.Find("h3 a")
		title := titleTag.Text()
		href, _ := titleTag.Attr("href")
		snippet := s.Find(".space-txt, [class*='fz-mid']").Text()

		if href != "" {
			if strings.HasPrefix(href, "/link") {
				href = "https://www.sogou.com" + href
			}
			if strings.HasPrefix(href, "http") {
				results = append(results, SearchResult{
					Title:   strings.TrimSpace(title),
					URL:     href,
					Snippet: strings.TrimSpace(snippet),
					Source:  "Sogou",
				})
			}
		}
	})

	return results, nil
}

// Google Google搜索引擎
type Google struct{}

func (g *Google) Name() string {
	return "Google"
}

func (g *Google) Search(query string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	baseURL := "https://www.google.com/search"
	params := url.Values{}
	params.Add("q", query)
	params.Add("num", fmt.Sprintf("%d", limit))

	fullURL := baseURL + "?" + params.Encode()

	client := newHTTPClient()
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	setCommonHeaders(req)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google returned status code %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	doc.Find(".g").Each(func(i int, s *goquery.Selection) {
		if i >= limit {
			return
		}

		titleTag := s.Find("h3")
		title := titleTag.Text()
		linkTag := s.Find("a")
		href, _ := linkTag.Attr("href")
		snippet := s.Find(".VwiC3b, .s").Text()

		if href != "" && strings.HasPrefix(href, "http") && !strings.Contains(href, "google.com") {
			results = append(results, SearchResult{
				Title:   strings.TrimSpace(title),
				URL:     href,
				Snippet: strings.TrimSpace(snippet),
				Source:  "Google",
			})
		}
	})

	return results, nil
}
