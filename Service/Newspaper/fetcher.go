package main

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Items       []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
	Author      string `xml:"author"`
	GUID        string `xml:"guid"`
}

type AtomFeed struct {
	XMLName xml.Name `xml:"feed"`
	Entries []struct {
		Title string `xml:"title"`
		Link  struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
		Summary   string `xml:"summary"`
		Content   string `xml:"content"`
		Author    struct {
			Name string `xml:"name"`
		} `xml:"author"`
	} `xml:"entry"`
}

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
				DisableKeepAlives:  false,
			},
		},
	}
}

func (f *Fetcher) FetchAllSources() ([]News, error) {
	sources, err := GetSources()
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "获取新闻源失败: %v", err)
		}
		return nil, fmt.Errorf("failed to get sources: %w", err)
	}

	if logger != nil {
		logger.Info("fetcher", "开始抓取 %d 个新闻源", len(sources))
	}

	var allNews []News
	var successCount, failCount int
	for _, src := range sources {
		if !src.Enabled {
			continue
		}

		if logger != nil {
			logger.Debug("fetcher", "正在抓取: %s (%s)", src.Name, src.URL)
		}

		news, err := f.FetchSource(src)
		if err != nil {
			if logger != nil {
				logger.Error("fetcher", "抓取失败 [%s]: %v", src.Name, err)
			}
			AddFetchLog(FetchLog{
				SourceName: src.Name,
				Success:    false,
				Count:      0,
				Error:      err.Error(),
			})
			failCount++
			continue
		}

		allNews = append(allNews, news...)

		if logger != nil {
			logger.Info("fetcher", "抓取成功 [%s]: 获取 %d 条新闻", src.Name, len(news))
		}

		AddFetchLog(FetchLog{
			SourceName: src.Name,
			Success:    true,
			Count:      len(news),
		})
		UpdateSourceLastFetch(src.ID)
		successCount++
	}

	if logger != nil {
		logger.Info("fetcher", "抓取完成: 成功 %d 个源, 失败 %d 个源, 共获取 %d 条新闻", successCount, failCount, len(allNews))
	}

	return allNews, nil
}

func (f *Fetcher) FetchSource(source NewsSource) ([]News, error) {
	switch source.Type {
	case "rss":
		return f.fetchRSS(source)
	case "atom":
		return f.fetchAtom(source)
	default:
		return f.fetchRSS(source)
	}
}

func (f *Fetcher) fetchRSS(source NewsSource) ([]News, error) {
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "创建请求失败 [%s]: %v", source.Name, err)
		}
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	if logger != nil {
		logger.Debug("fetcher", "发送请求到: %s", source.URL)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "请求失败 [%s]: %v", source.Name, err)
		}
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if logger != nil {
			logger.Error("fetcher", "HTTP错误 [%s]: 状态码 %d", source.Name, resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, source.Name)
	}

	if logger != nil {
		logger.Debug("fetcher", "收到响应 [%s]: HTTP %d, Content-Length: %s", source.Name, resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "读取响应体失败 [%s]: %v", source.Name, err)
		}
		return nil, err
	}

	if len(body) == 0 {
		if logger != nil {
			logger.Warning("fetcher", "响应体为空 [%s]", source.Name)
		}
		return nil, fmt.Errorf("响应体为空: %s", source.Name)
	}

	var feed RSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		if logger != nil {
			logger.Error("fetcher", "XML解析失败 [%s]: %v (响应前200字符: %s)", source.Name, err, string(body[:min(len(body), 200)]))
		}
		return nil, fmt.Errorf("XML解析错误: %v", err)
	}

	if logger != nil {
		logger.Debug("fetcher", "XML解析成功 [%s]: 找到 %d 条项目", source.Name, len(feed.Channel.Items))
	}

	var newsList []News
	for _, item := range feed.Channel.Items {
		news := News{
			Title:    cleanText(item.Title),
			URL:      item.Link,
			Summary:  cleanHTML(item.Description),
			Source:   source.Name,
			Category: getCategory(item.Category, source.Category),
		}

		if item.GUID != "" && news.URL == "" {
			news.URL = item.GUID
		}

		news.PublishedAt = parseDate(item.PubDate)
		if news.PublishedAt == "" {
			news.PublishedAt = time.Now().Format("2006-01-02 15:04:05")
		}

		if news.Title != "" && news.URL != "" {
			newsList = append(newsList, news)
		}
	}

	if logger != nil {
		logger.Debug("fetcher", "处理完成 [%s]: 有效新闻 %d 条", source.Name, len(newsList))
	}

	return newsList, nil
}

func (f *Fetcher) fetchAtom(source NewsSource) ([]News, error) {
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "创建请求失败 [%s]: %v", source.Name, err)
		}
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/atom+xml, application/xml, text/xml, */*")

	if logger != nil {
		logger.Debug("fetcher", "发送Atom请求到: %s", source.URL)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "请求失败 [%s]: %v", source.Name, err)
		}
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if logger != nil {
			logger.Error("fetcher", "HTTP错误 [%s]: 状态码 %d", source.Name, resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, source.Name)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "读取响应体失败 [%s]: %v", source.Name, err)
		}
		return nil, err
	}

	if len(body) == 0 {
		if logger != nil {
			logger.Warning("fetcher", "响应体为空 [%s]", source.Name)
		}
		return nil, fmt.Errorf("响应体为空: %s", source.Name)
	}

	var feed AtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		if logger != nil {
			logger.Error("fetcher", "Atom解析失败 [%s]: %v", source.Name, err)
		}
		return nil, fmt.Errorf("Atom解析错误: %v", err)
	}

	var newsList []News
	for _, entry := range feed.Entries {
		linkURL := entry.Link.Href
		if linkURL == "" {
			linkURL = entry.Link.Href
		}

		pubDate := entry.Published
		if pubDate == "" {
			pubDate = entry.Updated
		}

		content := entry.Content
		if content == "" {
			content = entry.Summary
		}

		news := News{
			Title:       cleanText(entry.Title),
			URL:         linkURL,
			Summary:     cleanHTML(entry.Summary),
			Content:     cleanHTML(content),
			Source:      source.Name,
			Category:    source.Category,
			PublishedAt: parseDate(pubDate),
		}

		if news.PublishedAt == "" {
			news.PublishedAt = time.Now().Format("2006-01-02 15:04:05")
		}

		if news.Title != "" && news.URL != "" {
			newsList = append(newsList, news)
		}
	}

	return newsList, nil
}

func (f *Fetcher) FetchCustomURL(rawURL string) ([]News, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	source := NewsSource{
		Name:     parsedURL.Host,
		URL:      rawURL,
		Type:     "rss",
		Category: "综合",
	}

	if strings.Contains(rawURL, "atom") || strings.Contains(rawURL, "feed") {
		source.Type = "atom"
	}

	return f.FetchSource(source)
}

func cleanText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\n", " ")
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	return text
}

func cleanHTML(htmlStr string) string {
	htmlStr = strings.TrimSpace(htmlStr)
	if htmlStr == "" {
		return ""
	}

	tagsRegex := regexp.MustCompile(`<[^>]*>`)
	text := tagsRegex.ReplaceAllString(htmlStr, " ")

	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	text = strings.TrimSpace(text)
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	if len(text) > 500 {
		text = text[:500] + "..."
	}

	return text
}

func getCategory(itemCategory, defaultCategory string) string {
	if itemCategory != "" {
		cat := strings.ToLower(itemCategory)
		categoryMap := map[string]string{
			"politics":      "时政",
			"government":    "时政",
			"财经":            "财经",
			"finance":       "财经",
			"business":      "财经",
			"经济":            "财经",
			"体育":            "体育",
			"sports":        "体育",
			"科技":            "科技",
			"tech":          "科技",
			"technology":    "科技",
			"health":        "健康",
			"健康":            "健康",
			"entertainment": "娱乐",
			"娱乐":            "娱乐",
			"社会":            "社会",
			"society":       "社会",
			"国际":            "国际",
			"world":         "国际",
			"军事":            "军事",
			"military":      "军事",
			"教育":            "教育",
			"education":     "教育",
			"汽车":            "汽车",
			"auto":          "汽车",
			"房产":            "房产",
			"house":         "房产",
		}
		if mapped, ok := categoryMap[cat]; ok {
			return mapped
		}
		if len(itemCategory) <= 10 {
			return itemCategory
		}
	}
	return defaultCategory
}

func parseDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	layouts := []string{
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05+08:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02 Jan 2006",
		time.RFC3339,
		time.RFC1123,
		time.RFC1123Z,
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, strings.TrimSpace(dateStr)); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}

	return ""
}

func FetchAndSaveAll() (int, error) {
	fetcher := NewFetcher()
	newsList, err := fetcher.FetchAllSources()
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "抓取全部新闻源失败: %v", err)
		}
		return 0, err
	}

	if len(newsList) == 0 {
		if logger != nil {
			logger.Warning("fetcher", "所有新闻源都没有获取到新新闻")
		}
		return 0, nil
	}

	if logger != nil {
		logger.Info("fetcher", "准备保存 %d 条新闻", len(newsList))
	}

	saved, err := SaveNewsBatch(newsList)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "保存新闻失败: %v", err)
		}
		return 0, err
	}

	if logger != nil {
		logger.Info("fetcher", "成功保存 %d 条新闻", saved)
	}

	return saved, nil
}

func FetchAndSaveSource(sourceID int) (int, error) {
	source, err := GetSourceByID(sourceID)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "获取新闻源失败 [ID=%d]: %v", sourceID, err)
		}
		return 0, err
	}

	if logger != nil {
		logger.Info("fetcher", "开始抓取单个源: %s", source.Name)
	}

	fetcher := NewFetcher()
	newsList, err := fetcher.FetchSource(*source)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "抓取单个源失败 [%s]: %v", source.Name, err)
		}
		return 0, err
	}

	if len(newsList) == 0 {
		if logger != nil {
			logger.Warning("fetcher", "新闻源 %s 没有获取到新新闻", source.Name)
		}
		return 0, nil
	}

	saved, err := SaveNewsBatch(newsList)
	if err != nil {
		if logger != nil {
			logger.Error("fetcher", "保存新闻失败 [%s]: %v", source.Name, err)
		}
		return 0, err
	}

	if logger != nil {
		logger.Info("fetcher", "成功保存 %d 条新闻 [%s]", saved, source.Name)
	}

	return saved, nil
}

func GetSourceByID(id int) (*NewsSource, error) {
	var s NewsSource
	var enabledInt int
	err := db.QueryRow(
		"SELECT id, name, url, type, category, enabled, last_fetch FROM news_sources WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.URL, &s.Type, &s.Category, &enabledInt, &s.LastFetch)
	if err != nil {
		return nil, err
	}
	s.Enabled = enabledInt == 1
	return &s, nil
}
