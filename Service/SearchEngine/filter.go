package main

import (
	"regexp"
	"sort"
	"strings"
)

// ============ 数据结构设计 ============

// FacetValue 分面值
type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Facet 动态分面
type Facet struct {
	Key      string       `json:"key"`           // 分面标识，如 "domain"、"source"、"file_type"
	Label    string       `json:"label"`         // 显示名称，如 "来源网站"
	Type     string       `json:"type"`          // 控件类型：checkbox | chip | slider | toggle
	Values   []FacetValue `json:"values"`        // 可选值（checkbox/chip 用）
	Min      float64      `json:"min,omitempty"` // 最小值（slider 用）
	Max      float64      `json:"max,omitempty"` // 最大值（slider 用）
	Step     float64      `json:"step,omitempty"`
	Intent   string       `json:"intent,omitempty"` // 语义意图标签
	Priority int          `json:"priority"`         // 排序优先级
}

// FacetResponse 搜索结果附带的分面数据
type FacetResponse struct {
	Facets     []Facet     `json:"facets"`      // 动态分面列表
	QuickChips []QuickChip `json:"quick_chips"` // 顶部快捷热词芯片
	Intent     string      `json:"intent"`      // 解析出的搜索意图
	IntentTags []string    `json:"intent_tags"` // 意图标签（如"购物"、"技术"、"本地"）
}

// QuickChip 快捷热词芯片
type QuickChip struct {
	Label    string `json:"label"`     // 显示文本
	Value    string `json:"value"`     // 筛选值
	FacetKey string `json:"facet_key"` // 关联分面
	Hot      bool   `json:"hot"`       // 是否热门
}

// FilterParams 前端传来的筛选参数
type FilterParams struct {
	Sources        []string // 按来源筛选
	Domains        []string // 按域名筛选（包含）
	ExcludeDomains []string // 域名黑名单
	FileTypes      []string // 按文件类型
	MultiSource    bool     // 只看多引擎收录
	Keyword        string   // 结果内关键词
	MinScore       float64  // 最低分数
	SiteGroup      string   // 站点分组名
}

// ============ 语义意图分析 ============

// intentRule 意图规则
type intentRule struct {
	keywords  []string
	tags      []string
	intent    string
	facetHint []string // 建议的分面类型
}

var intentRules = []intentRule{
	{
		keywords:  []string{"跑鞋", "球鞋", "运动鞋", "篮球鞋", "登山鞋"},
		tags:      []string{"购物", "运动"},
		intent:    "shopping_sports",
		facetHint: []string{"brand", "price_range", "source"},
	},
	{
		keywords:  []string{"显示器", "显卡", "cpu", "笔记本", "手机", "耳机", "键盘"},
		tags:      []string{"购物", "数码"},
		intent:    "shopping_electronics",
		facetHint: []string{"brand", "source", "domain"},
	},
	{
		keywords:  []string{"教程", "怎么", "如何", "安装", "配置", "部署", "debug", "error"},
		tags:      []string{"技术"},
		intent:    "technical",
		facetHint: []string{"domain", "source", "file_type"},
	},
	{
		keywords:  []string{"周末", "带娃", "玩水", "亲子", "遛娃", "附近", "周边"},
		tags:      []string{"本地", "生活"},
		intent:    "local_life",
		facetHint: []string{"domain", "source"},
	},
	{
		keywords:  []string{"论文", "文献", "研究", "学术"},
		tags:      []string{"学术"},
		intent:    "academic",
		facetHint: []string{"domain", "file_type"},
	},
	{
		keywords:  []string{"python", "java", "golang", "rust", "javascript", "c++", "代码"},
		tags:      []string{"编程", "技术"},
		intent:    "programming",
		facetHint: []string{"domain", "source"},
	},
}

// analyzeIntent 分析搜索意图
func analyzeIntent(query string) (intent string, tags []string, facetHints []string) {
	queryLower := strings.ToLower(query)
	matched := false
	for _, rule := range intentRules {
		for _, kw := range rule.keywords {
			if strings.Contains(queryLower, strings.ToLower(kw)) {
				intent = rule.intent
				tags = mergeUnique(tags, rule.tags)
				facetHints = mergeUnique(facetHints, rule.facetHint)
				matched = true
				break
			}
		}
	}
	if !matched {
		intent = "general"
		tags = []string{"通用"}
	}
	return
}

// ============ 动态分面生成 ============

// extractDomain 从 URL 提取域名
func extractDomain(rawURL string) string {
	u := strings.TrimSpace(rawURL)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "www.")
	if idx := strings.Index(u, "/"); idx > 0 {
		u = u[:idx]
	}
	return u
}

// extractFileType 从 URL 提取文件类型
func extractFileType(rawURL string) string {
	lower := strings.ToLower(rawURL)
	if strings.Contains(lower, ".pdf") {
		return "PDF"
	}
	if strings.Contains(lower, ".doc") || strings.Contains(lower, ".docx") {
		return "DOC"
	}
	if strings.Contains(lower, ".ppt") || strings.Contains(lower, ".pptx") {
		return "PPT"
	}
	if strings.Contains(lower, ".xlsx") || strings.Contains(lower, ".xls") {
		return "XLS"
	}
	return ""
}

// extractBrand 从标题/摘要中提取品牌名
var brandPatterns = map[string][]string{
	"Nike":     {"nike", "耐克"},
	"Adidas":   {"adidas", "阿迪达斯"},
	"Apple":    {"apple", "苹果"},
	"Samsung":  {"samsung", "三星"},
	"Huawei":   {"huawei", "华为"},
	"Xiaomi":   {"xiaomi", "小米"},
	"Dell":     {"dell", "戴尔"},
	"HP":       {"hp ", "惠普"},
	"Lenovo":   {"lenovo", "联想"},
	"ASUS":     {"asus", "华硕"},
	"Logitech": {"logitech", "罗技"},
	"Razer":    {"razer", "雷蛇"},
	"Corsair":  {"corsair"},
	"Bose":     {"bose"},
	"Sony":     {"sony", "索尼"},
}

func extractBrand(title, snippet string) string {
	text := strings.ToLower(title + " " + snippet)
	for brand, patterns := range brandPatterns {
		for _, p := range patterns {
			if strings.Contains(text, p) {
				return brand
			}
		}
	}
	return ""
}

// buildFacets 根据搜索结果动态构建分面
func buildFacets(results []SearchResult, intent string, tags []string, facetHints []string) []Facet {
	facets := make([]Facet, 0)

	// 1. 来源引擎分面（总是生成）
	sourceCount := make(map[string]int)
	for _, r := range results {
		sources := r.Sources
		if len(sources) == 0 && r.Source != "" {
			sources = []string{r.Source}
		}
		for _, s := range sources {
			sourceCount[s]++
		}
	}
	if len(sourceCount) > 0 {
		fv := make([]FacetValue, 0, len(sourceCount))
		for s, c := range sourceCount {
			fv = append(fv, FacetValue{Value: s, Count: c})
		}
		sort.Slice(fv, func(i, j int) bool { return fv[i].Count > fv[j].Count })
		facets = append(facets, Facet{
			Key:      "source",
			Label:    "搜索引擎",
			Type:     "chip",
			Values:   fv,
			Priority: 100,
		})
	}

	// 2. 域名分面（总是生成，取 top 8）
	domainCount := make(map[string]int)
	for _, r := range results {
		d := extractDomain(r.URL)
		if d != "" {
			domainCount[d]++
		}
	}
	if len(domainCount) > 0 {
		fv := make([]FacetValue, 0, len(domainCount))
		for d, c := range domainCount {
			fv = append(fv, FacetValue{Value: d, Count: c})
		}
		sort.Slice(fv, func(i, j int) bool { return fv[i].Count > fv[j].Count })
		if len(fv) > 8 {
			fv = fv[:8]
		}
		facets = append(facets, Facet{
			Key:      "domain",
			Label:    "来源网站",
			Type:     "checkbox",
			Values:   fv,
			Priority: 90,
		})
	}

	// 3. 多引擎收录（总是生成）
	multiCount := 0
	for _, r := range results {
		if r.ResultCount > 1 {
			multiCount++
		}
	}
	facets = append(facets, Facet{
		Key:      "multi_source",
		Label:    "多引擎收录",
		Type:     "toggle",
		Priority: 80,
		Values:   []FacetValue{{Value: "true", Count: multiCount}},
	})

	// 4. 品牌分面（购物意图时生成）
	if containsAny(tags, "购物") || containsStr(facetHints, "brand") {
		brandCount := make(map[string]int)
		for _, r := range results {
			b := extractBrand(r.Title, r.Snippet)
			if b != "" {
				brandCount[b]++
			}
		}
		if len(brandCount) > 0 {
			fv := make([]FacetValue, 0, len(brandCount))
			for b, c := range brandCount {
				fv = append(fv, FacetValue{Value: b, Count: c})
			}
			sort.Slice(fv, func(i, j int) bool { return fv[i].Count > fv[j].Count })
			facets = append(facets, Facet{
				Key:      "brand",
				Label:    "品牌",
				Type:     "chip",
				Values:   fv,
				Intent:   "shopping",
				Priority: 70,
			})
		}
	}

	// 5. 文件类型分面（学术/技术意图时生成）
	if containsAny(tags, "学术") || containsAny(tags, "技术") || containsStr(facetHints, "file_type") {
		ftCount := make(map[string]int)
		for _, r := range results {
			ft := extractFileType(r.URL)
			if ft != "" {
				ftCount[ft]++
			}
		}
		if len(ftCount) > 0 {
			fv := make([]FacetValue, 0, len(ftCount))
			for ft, c := range ftCount {
				fv = append(fv, FacetValue{Value: ft, Count: c})
			}
			sort.Slice(fv, func(i, j int) bool { return fv[i].Count > fv[j].Count })
			facets = append(facets, Facet{
				Key:      "file_type",
				Label:    "文件类型",
				Type:     "checkbox",
				Values:   fv,
				Intent:   "academic",
				Priority: 60,
			})
		}
	}

	// 6. HTTPS 安全分面
	httpsCount := 0
	for _, r := range results {
		if strings.HasPrefix(r.URL, "https://") {
			httpsCount++
		}
	}
	facets = append(facets, Facet{
		Key:      "https",
		Label:    "安全连接",
		Type:     "toggle",
		Priority: 50,
		Values:   []FacetValue{{Value: "true", Count: httpsCount}},
	})

	// 按优先级排序
	sort.Slice(facets, func(i, j int) bool {
		return facets[i].Priority > facets[j].Priority
	})

	return facets
}

// buildQuickChips 构建快捷热词芯片
func buildQuickChips(results []SearchResult, facets []Facet, intent string) []QuickChip {
	chips := make([]QuickChip, 0)

	// 从来源分面生成芯片
	for _, f := range facets {
		if f.Key == "source" {
			for i, v := range f.Values {
				if i >= 4 {
					break
				}
				chips = append(chips, QuickChip{
					Label:    v.Value,
					Value:    v.Value,
					FacetKey: "source",
					Hot:      i < 2,
				})
			}
		}
		if f.Key == "brand" {
			for i, v := range f.Values {
				if i >= 3 {
					break
				}
				chips = append(chips, QuickChip{
					Label:    v.Value,
					Value:    v.Value,
					FacetKey: "brand",
					Hot:      i < 2,
				})
			}
		}
	}

	// 多引擎收录芯片
	chips = append(chips, QuickChip{
		Label:    "🔥 多引擎收录",
		Value:    "true",
		FacetKey: "multi_source",
		Hot:      true,
	})

	return chips
}

// BuildFacetResponse 构建完整分面响应
func BuildFacetResponse(results []SearchResult, query string) FacetResponse {
	intent, tags, facetHints := analyzeIntent(query)
	facets := buildFacets(results, intent, tags, facetHints)
	chips := buildQuickChips(results, facets, intent)
	return FacetResponse{
		Facets:     facets,
		QuickChips: chips,
		Intent:     intent,
		IntentTags: tags,
	}
}

// ============ 筛选执行 ============

// ApplyFilters 对结果应用筛选
func ApplyFilters(results []SearchResult, params FilterParams) []SearchResult {
	filtered := make([]SearchResult, 0, len(results))

	sourceSet := toSet(params.Sources)
	domainSet := toSet(params.Domains)
	excludeSet := toSet(params.ExcludeDomains)
	fileTypeSet := toSet(params.FileTypes)
	keywordLower := strings.ToLower(params.Keyword)

	for _, r := range results {
		// 来源筛选
		if len(sourceSet) > 0 {
			sources := r.Sources
			if len(sources) == 0 && r.Source != "" {
				sources = []string{r.Source}
			}
			matched := false
			for _, s := range sources {
				if sourceSet[s] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 域名包含筛选
		if len(domainSet) > 0 {
			d := extractDomain(r.URL)
			if !domainMatched(d, domainSet) {
				continue
			}
		}

		// 域名黑名单
		if len(excludeSet) > 0 {
			d := extractDomain(r.URL)
			if domainMatched(d, excludeSet) {
				continue
			}
		}

		// 文件类型筛选
		if len(fileTypeSet) > 0 {
			ft := extractFileType(r.URL)
			if !fileTypeSet[ft] {
				continue
			}
		}

		// 多引擎收录
		if params.MultiSource && r.ResultCount <= 1 {
			continue
		}

		// 关键词
		if keywordLower != "" {
			titleLower := strings.ToLower(r.Title)
			snippetLower := strings.ToLower(r.Snippet)
			if !strings.Contains(titleLower, keywordLower) && !strings.Contains(snippetLower, keywordLower) {
				continue
			}
		}

		// 最低分数
		if params.MinScore > 0 && r.Score < params.MinScore {
			continue
		}

		filtered = append(filtered, r)
	}

	return filtered
}

// ============ 自然语言指令解析 ============

// nlPatterns 自然语言指令模式
var nlPatterns = []struct {
	pattern *regexp.Regexp
	action  string // "exclude_domain" | "include_domain" | "source" | "file_type"
	extract func(matches []string) (key string, value string)
}{
	{
		pattern: regexp.MustCompile(`(?i)排除\s*(CSDN|知乎|百度|微博|简书|掘金|博客园)\s*结果?`),
		action:  "exclude_domain",
		extract: func(m []string) (string, string) {
			return "exclude_domain", domainAlias(m[1])
		},
	},
	{
		pattern: regexp.MustCompile(`(?i)不要\s*(CSDN|知乎|百度|微博|简书|掘金|博客园)`),
		action:  "exclude_domain",
		extract: func(m []string) (string, string) {
			return "exclude_domain", domainAlias(m[1])
		},
	},
	{
		pattern: regexp.MustCompile(`(?i)只看\s*(CSDN|知乎|百度|微博|简书|掘金|博客园|GitHub|StackOverflow)`),
		action:  "include_domain",
		extract: func(m []string) (string, string) {
			return "include_domain", domainAlias(m[1])
		},
	},
	{
		pattern: regexp.MustCompile(`(?i)只要\s*(PDF|DOC|PPT|XLS)`),
		action:  "file_type",
		extract: func(m []string) (string, string) {
			return "file_type", m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?i)排除\s*([\w.]+\.\w+)`),
		action:  "exclude_domain",
		extract: func(m []string) (string, string) {
			return "exclude_domain", m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?i)用\s*(Bing|Baidu|Google|DuckDuckGo|Sogou)\s*搜`),
		action:  "source",
		extract: func(m []string) (string, string) {
			return "source", m[1]
		},
	},
	{
		pattern: regexp.MustCompile(`(?i)多引擎|多来源`),
		action:  "multi_source",
		extract: func(m []string) (string, string) {
			return "multi_source", "true"
		},
	},
}

// domainAlias 域名别名映射
func domainAlias(name string) string {
	alias := map[string]string{
		"CSDN":          "csdn.net",
		"知乎":            "zhihu.com",
		"百度":            "baidu.com",
		"微博":            "weibo.com",
		"简书":            "jianshu.com",
		"掘金":            "juejin.cn",
		"博客园":           "cnblogs.com",
		"GitHub":        "github.com",
		"StackOverflow": "stackoverflow.com",
	}
	if v, ok := alias[name]; ok {
		return v
	}
	return strings.ToLower(name)
}

// ParseNLCommand 解析自然语言筛选指令
// 返回：解析出的指令列表，清理后的 query
func ParseNLCommand(query string) ([]NLCommand, string) {
	commands := make([]NLCommand, 0)
	cleanQuery := query

	for _, p := range nlPatterns {
		matches := p.pattern.FindStringSubmatch(query)
		if matches != nil {
			key, value := p.extract(matches)
			commands = append(commands, NLCommand{
				Action: p.action,
				Key:    key,
				Value:  value,
			})
			// 从 query 中移除匹配的部分
			cleanQuery = strings.ReplaceAll(cleanQuery, matches[0], "")
		}
	}

	cleanQuery = strings.TrimSpace(cleanQuery)
	// 移除多余空格
	cleanQuery = regexp.MustCompile(`\s+`).ReplaceAllString(cleanQuery, " ")

	return commands, cleanQuery
}

// NLCommand 自然语言解析出的指令
type NLCommand struct {
	Action string `json:"action"` // exclude_domain | include_domain | source | file_type | multi_source
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// NLCommandsToFilterParams 将指令转为筛选参数
func NLCommandsToFilterParams(cmds []NLCommand) FilterParams {
	fp := FilterParams{}
	for _, c := range cmds {
		switch c.Action {
		case "exclude_domain":
			fp.ExcludeDomains = append(fp.ExcludeDomains, c.Value)
		case "include_domain":
			fp.Domains = append(fp.Domains, c.Value)
		case "source":
			fp.Sources = append(fp.Sources, c.Value)
		case "file_type":
			fp.FileTypes = append(fp.FileTypes, c.Value)
		case "multi_source":
			fp.MultiSource = true
		}
	}
	return fp
}

// ============ 站点分组（Brave Goggles 机制）============

// SiteGroup 站点分组
type SiteGroup struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Domains     []string `json:"domains"`
	Icon        string   `json:"icon"`
	BuiltIn     bool     `json:"built_in"`
}

// 内置站点分组
var builtInGroups = []SiteGroup{
	{
		Name:        "技术社区",
		Description: "GitHub, StackOverflow, 掘金, CSDN 等技术站点",
		Domains:     []string{"github.com", "stackoverflow.com", "juejin.cn", "csdn.net", "cnblogs.com"},
		Icon:        "💻",
		BuiltIn:     true,
	},
	{
		Name:        "学术资源",
		Description: "维基百科, 知网, 万方等学术站点",
		Domains:     []string{"wikipedia.org", "cnki.net", "wanfangdata.com.cn", "arxiv.org", "scholar.google.com"},
		Icon:        "📚",
		BuiltIn:     true,
	},
	{
		Name:        "官方权威",
		Description: "政府、教育机构官网",
		Domains:     []string{"gov.cn", "edu.cn"},
		Icon:        "🏛️",
		BuiltIn:     true,
	},
	{
		Name:        "问答平台",
		Description: "知乎, 百度知道等问答社区",
		Domains:     []string{"zhihu.com", "baike.baidu.com", "zhihu.com"},
		Icon:        "❓",
		BuiltIn:     true,
	},
	{
		Name:        "新闻媒体",
		Description: "主流新闻媒体",
		Domains:     []string{"thepaper.cn", "163.com", "sina.com.cn", "sohu.com", "ifeng.com"},
		Icon:        "📰",
		BuiltIn:     true,
	},
}

// GetSiteGroups 获取所有站点分组
func GetSiteGroups() []SiteGroup {
	return builtInGroups
}

// ApplySiteGroup 应用站点分组筛选
func ApplySiteGroup(results []SearchResult, group SiteGroup) []SearchResult {
	domainSet := make(map[string]bool)
	for _, d := range group.Domains {
		domainSet[d] = true
	}
	filtered := make([]SearchResult, 0)
	for _, r := range results {
		d := extractDomain(r.URL)
		if domainSet[d] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// ============ 辅助函数 ============

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

// domainMatched 检查域名是否匹配集合（支持子域名，如 csdn.net 匹配 blog.csdn.net）
func domainMatched(domain string, set map[string]bool) bool {
	if set[domain] {
		return true
	}
	// 检查父域名是否在集合中
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if set[parent] {
			return true
		}
	}
	return false
}

func mergeUnique(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		set[s] = true
	}
	result := make([]string, 0, len(set))
	for s := range set {
		result = append(result, s)
	}
	sort.Strings(result)
	return result
}

func containsAny(slice []string, items ...string) bool {
	set := toSet(slice)
	for _, item := range items {
		if set[item] {
			return true
		}
	}
	return false
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
