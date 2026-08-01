package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExportOptions struct {
	Format   string   `json:"format"`
	DateFrom string   `json:"date_from"`
	DateTo   string   `json:"date_to"`
	Category string   `json:"category"`
	Keyword  string   `json:"keyword"`
	Sources  []string `json:"sources"`
	Title    string   `json:"title"`
	MaxItems int      `json:"max_items"`
}

var gradients = []string{
	"linear-gradient(135deg, #2c1810 0%, #5c3d2e 50%, #8b6f4e 100%)",
	"linear-gradient(160deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%)",
	"linear-gradient(145deg, #2d1b4e 0%, #4a2c6d 50%, #6b3fa0 100%)",
	"linear-gradient(130deg, #1a3a1a 0%, #2d5a2d 50%, #4a7c4a 100%)",
	"linear-gradient(155deg, #3a1a1a 0%, #5a2d2d 50%, #8b4a4a 100%)",
}

var stanceColorMap = map[string]struct {
	bg    string
	label string
}{
	"left":         {"#c62828", "← 倾向保守"},
	"center-left":  {"#ef5350", "↖ 中左"},
	"neutral":      {"#757575", "○ 中立"},
	"center-right": {"#64b5f6", "↗ 中右"},
	"right":        {"#1565c0", "→ 倾向自由"},
}

var stanceLabelCN = map[string]string{
	"left":         "立场偏左",
	"center-left":  "立场中左",
	"neutral":      "中立",
	"center-right": "立场中右",
	"right":        "立场偏右",
}

func getGradient(index int, category string) string {
	idx := (index + len(category)) % len(gradients)
	return gradients[idx]
}

func getStanceBadgeHTML(stance string) string {
	if info, ok := stanceColorMap[stance]; ok {
		return fmt.Sprintf(`<span class="analysis-badge" style="background:%s">%s</span>`, info.bg, info.label)
	}
	return ""
}

func getReliabilityBadgeHTML(n News) string {
	if n.HasPrivateAd {
		return `<span class="analysis-badge warning" title="疑似夹带私活/广告">⚠️ 私活</span>`
	}
	if n.AdScore > 0.3 {
		return `<span class="analysis-badge warning-light" title="可能包含推广内容">🔍 低可信度</span>`
	}
	tags := splitTags(n.AnalysisTags)
	for _, tag := range tags {
		if strings.Contains(tag, "可信度:低") {
			return `<span class="analysis-badge warning-light">🔍 可信度低</span>`
		}
	}
	return ""
}

func getConfidenceFromTags(tagsStr string) string {
	tags := splitTags(tagsStr)
	for _, tag := range tags {
		if strings.Contains(tag, "可信度:高") {
			return "高"
		}
		if strings.Contains(tag, "可信度:中等") {
			return "中等"
		}
		if strings.Contains(tag, "可信度:低") {
			return "低"
		}
	}
	return ""
}

func ExportNews(options ExportOptions) (string, string, error) {
	var newsList []News
	var err error

	if options.DateFrom != "" || options.DateTo != "" {
		newsList, err = GetNewsByDateRange(options.DateFrom, options.DateTo)
	} else if options.Keyword != "" || options.Category != "" || len(options.Sources) > 0 {
		newsList, err = SearchNewsAdvanced(options.Keyword, []string{options.Category}, options.Sources, options.DateFrom, options.DateTo)
	} else {
		resp, err := GetNews(1, 1000, "", "", "", "")
		if err == nil {
			newsList = resp.News
		}
	}

	if err != nil {
		return "", "", err
	}

	if options.MaxItems > 0 && len(newsList) > options.MaxItems {
		newsList = newsList[:options.MaxItems]
	}

	if len(newsList) == 0 {
		return "", "", fmt.Errorf("没有可导出的新闻")
	}

	switch options.Format {
	case "md":
		return exportMarkdown(newsList, options)
	case "json":
		return exportJSON(newsList, options)
	case "html":
		return exportHTML(newsList, options)
	default:
		return exportMarkdown(newsList, options)
	}
}

func exportMarkdown(newsList []News, options ExportOptions) (string, string, error) {
	if options.Title == "" {
		dateStr := time.Now().Format("2006年01月02日")
		options.Title = fmt.Sprintf("昨日晚报 - %s", dateStr)
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", options.Title))
	sb.WriteString(fmt.Sprintf("> 导出时间：%s | 新闻数量：%d\n\n", time.Now().Format("2006-01-02 15:04:05"), len(newsList)))

	if options.Category != "" && options.Category != "all" {
		sb.WriteString(fmt.Sprintf("> 分类：%s\n\n", options.Category))
	}
	if options.Keyword != "" {
		sb.WriteString(fmt.Sprintf("> 关键词：%s\n\n", options.Keyword))
	}

	sb.WriteString("---\n\n")

	categories := make(map[string][]News)
	for _, n := range newsList {
		cat := n.Category
		if cat == "" {
			cat = "综合"
		}
		categories[cat] = append(categories[cat], n)
	}

	catOrder := []string{"时政", "财经", "科技", "体育", "社会", "国际", "军事", "教育", "健康", "娱乐", "汽车", "房产", "综合"}
	orderedCats := make([]string, 0)
	for _, c := range catOrder {
		if _, ok := categories[c]; ok {
			orderedCats = append(orderedCats, c)
		}
	}
	for c := range categories {
		found := false
		for _, oc := range orderedCats {
			if oc == c {
				found = true
				break
			}
		}
		if !found {
			orderedCats = append(orderedCats, c)
		}
	}

	for _, cat := range orderedCats {
		list := categories[cat]
		if len(list) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n\n", cat))

		for i, n := range list {
			if i >= 50 {
				sb.WriteString(fmt.Sprintf("*... 还有 %d 条新闻*\n\n", len(list)-i))
				break
			}

			title := html.UnescapeString(n.Title)
			if n.URL != "" {
				sb.WriteString(fmt.Sprintf("### %d. [%s](%s)\n\n", i+1, title, n.URL))
			} else {
				sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, title))
			}

			analysisBadges := []string{}
			if n.PoliticalStance != "" {
				if label, ok := stanceLabelCN[n.PoliticalStance]; ok {
					analysisBadges = append(analysisBadges, fmt.Sprintf("立场: %s", label))
				}
			}
			if n.PoliticalScore != 0 {
				analysisBadges = append(analysisBadges, fmt.Sprintf("倾向分数: %+.2f", n.PoliticalScore))
			}
			confidence := getConfidenceFromTags(n.AnalysisTags)
			if confidence != "" {
				analysisBadges = append(analysisBadges, fmt.Sprintf("置信度: %s", confidence))
			}
			if n.HasPrivateAd {
				analysisBadges = append(analysisBadges, "⚠️ 疑似夹带私活/广告")
			} else if n.AdScore > 0.3 {
				analysisBadges = append(analysisBadges, fmt.Sprintf("广告评分: %.2f (可能含推广)", n.AdScore))
			}
			if len(analysisBadges) > 0 {
				sb.WriteString(fmt.Sprintf("**【分析】%s**\n\n", strings.Join(analysisBadges, " | ")))
			}

			if n.AnalysisSummary != "" {
				sb.WriteString(fmt.Sprintf("> %s\n\n", html.UnescapeString(n.AnalysisSummary)))
			}

			if n.Summary != "" {
				summary := html.UnescapeString(n.Summary)
				if len(summary) > 300 {
					summary = summary[:300] + "..."
				}
				sb.WriteString(fmt.Sprintf("%s\n\n", summary))
			}

			meta := []string{}
			if n.Source != "" {
				meta = append(meta, fmt.Sprintf("来源：%s", n.Source))
			}
			if n.PublishedAt != "" {
				meta = append(meta, fmt.Sprintf("时间：%s", n.PublishedAt))
			}
			if len(meta) > 0 {
				sb.WriteString(fmt.Sprintf("*%s*\n\n", strings.Join(meta, " | ")))
			}
		}

		sb.WriteString("---\n\n")
	}

	sb.WriteString(fmt.Sprintf("*本文由市舶司昨日晚报自动生成于 %s*\n", time.Now().Format("2006-01-02 15:04:05")))

	filename := fmt.Sprintf("newspaper_%s.md", time.Now().Format("20060102_%150405"))
	return filename, sb.String(), nil
}

func exportJSON(newsList []News, options ExportOptions) (string, string, error) {
	exportData := map[string]interface{}{
		"title":       options.Title,
		"exported_at": time.Now().Format("2006-01-02 15:04:05"),
		"count":       len(newsList),
		"date_from":   options.DateFrom,
		"date_to":     options.DateTo,
		"news":        newsList,
	}

	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return "", "", err
	}

	filename := fmt.Sprintf("newspaper_%s.json", time.Now().Format("20060102_%150405"))
	return filename, string(jsonBytes), nil
}

func exportHTML(newsList []News, options ExportOptions) (string, string, error) {
	if options.Title == "" {
		dateStr := time.Now().Format("2006年01月02日")
		options.Title = fmt.Sprintf("昨日晚报 - %s", dateStr)
	}

	now := time.Now()
	dateStr := now.Format("2006年01月02日")
	exportTime := now.Format("2006-01-02 15:04:05")
	issueNo := fmt.Sprintf("%03d", now.YearDay())
	volNo := fmt.Sprintf("%02d", now.Year()%100)

	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + html.EscapeString(options.Title) + `</title>
    <style>
        @page {
            size: A4;
            margin: 15mm;
        }
        body {
            font-family: "Georgia", "Times New Roman", "Noto Serif SC", "Songti SC", serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 40px 20px;
            background: #fdfaf3;
            color: #1a1a1a;
            line-height: 1.75;
            font-feature-settings: "liga" 1, "onum" 1;
        }
        .newspaper-header {
            border: 3px double #1a1a1a;
            padding: 24px 28px;
            margin-bottom: 30px;
            position: relative;
            background: #fdfaf3;
        }
        .newspaper-header::before,
        .newspaper-header::after {
            content: "";
            position: absolute;
            left: 10px;
            right: 10px;
            height: 1px;
            background: #1a1a1a;
        }
        .newspaper-header::before { top: 6px; }
        .newspaper-header::after { bottom: 6px; }
        .header-top {
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-size: 11px;
            color: #6b6b6b;
            letter-spacing: 1px;
            font-variant: small-caps;
            text-transform: lowercase;
            border-bottom: 1px solid #2c2c2c;
            padding-bottom: 10px;
            margin-bottom: 16px;
            flex-wrap: wrap;
            gap: 10px;
        }
        .header-title {
            text-align: center;
            padding: 8px 0;
        }
        .header-title h1 {
            font-family: "Georgia", "Times New Roman", "Noto Serif SC", serif;
            font-size: 48px;
            font-weight: 900;
            font-style: italic;
            margin: 0;
            letter-spacing: 6px;
            line-height: 1;
            color: #1a1a1a;
        }
        .header-subtitle {
            font-size: 12px;
            letter-spacing: 4px;
            color: #6b6b6b;
            text-transform: uppercase;
            margin-top: 6px;
            font-variant: small-caps;
        }
        .header-bar {
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 16px;
            padding: 10px 20px;
            background: #1a1a1a;
            color: #fdfaf3;
            font-size: 11px;
            flex-wrap: wrap;
            letter-spacing: 2px;
        }
        .bar-item { font-weight: 400; font-variant: small-caps; }
        .bar-item span { color: #b8860b; font-weight: 700; }
        .bar-divider { color: #fdfaf3; opacity: 0.4; }
        .newspaper-section {
            margin-bottom: 30px;
            page-break-inside: avoid;
        }
        .section-header {
            display: flex;
            justify-content: space-between;
            align-items: baseline;
            border-bottom: 2px solid #1a1a1a;
            padding-bottom: 8px;
            margin-bottom: 16px;
        }
        .section-header h2 {
            font-family: "Georgia", "Times New Roman", "Noto Serif SC", serif;
            font-size: 24px;
            font-weight: 900;
            margin: 0;
            letter-spacing: 3px;
            color: #1a1a1a;
        }
        .section-count {
            font-size: 11px;
            color: #6b6b6b;
            font-variant: small-caps;
            letter-spacing: 1px;
        }
        .featured-article {
            border: 3px double #1a1a1a;
            padding: 24px;
            margin-bottom: 28px;
            background: linear-gradient(180deg, transparent 0%, rgba(139, 0, 0, 0.03) 100%), #fdfaf3;
            page-break-inside: avoid;
        }
        .featured-image {
            width: 100%;
            height: 280px;
            margin-bottom: 20px;
            position: relative;
            overflow: hidden;
            border: 3px double #1a1a1a;
        }
        .featured-image::after {
            content: "";
            position: absolute;
            inset: 0;
            background: repeating-linear-gradient(45deg, transparent, transparent 10px, rgba(255,255,255,0.03) 10px, rgba(255,255,255,0.03) 20px);
            pointer-events: none;
        }
        .featured-image-label {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            color: rgba(255,255,255,0.7);
            font-size: 18px;
            font-weight: 700;
            letter-spacing: 6px;
            text-transform: uppercase;
            font-variant: small-caps;
            text-shadow: 0 1px 3px rgba(0,0,0,0.5);
        }
        .featured-kicker {
            display: block;
            font-size: 12px;
            font-weight: 700;
            letter-spacing: 4px;
            text-transform: uppercase;
            color: #8b0000;
            margin-bottom: 10px;
            padding-bottom: 6px;
            border-bottom: 3px solid #8b0000;
            width: fit-content;
            font-variant: small-caps;
        }
        .featured-title {
            font-family: "Georgia", "Times New Roman", "Noto Serif SC", serif;
            font-size: 32px;
            font-weight: 900;
            font-style: italic;
            line-height: 1.2;
            margin: 10px 0;
            color: #1a1a1a;
        }
        .featured-title a {
            color: inherit;
            text-decoration: none;
        }
        .featured-summary {
            font-size: 15px;
            line-height: 1.8;
            color: #3a3a3a;
            column-count: 2;
            column-gap: 24px;
            text-align: justify;
            hyphens: auto;
            margin: 16px 0;
        }
        .featured-summary .drop-cap::first-letter {
            float: left;
            font-family: "Georgia", "Times New Roman", serif;
            font-size: 4.5em;
            line-height: 0.85;
            font-weight: 900;
            color: #8b0000;
            margin: 0.05em 0.1em 0 0;
            padding: 0.08em 0.12em;
            border: 2px solid #8b0000;
            background: #f4efe6;
        }
        .featured-meta {
            font-size: 11px;
            color: #6b6b6b;
            padding-top: 12px;
            border-top: 1px solid #2c2c2c;
            letter-spacing: 1px;
            font-variant: small-caps;
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
            gap: 8px;
        }
        .featured-meta .news-source {
            font-weight: 700;
            color: #8b0000;
        }
        .news-analysis {
            margin-bottom: 10px;
            display: flex;
            flex-wrap: wrap;
            gap: 6px;
        }
        .analysis-badge {
            display: inline-block;
            padding: 3px 10px;
            font-size: 10px;
            color: white;
            font-weight: 700;
            letter-spacing: 1px;
            text-transform: uppercase;
            border-radius: 0;
        }
        .analysis-badge.warning {
            background: #8b0000 !important;
        }
        .analysis-badge.warning-light {
            background: #b8860b !important;
        }
        .analysis-badge.info {
            background: #2c2c2c !important;
        }
        .news-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 24px;
        }
        .news-card {
            background: #fdfaf3;
            border: 1px solid #2c2c2c;
            padding: 18px;
            page-break-inside: avoid;
            transition: all 0.25s ease;
        }
        .news-card:hover {
            box-shadow: 4px 4px 0 #2c2c2c;
            transform: translate(-2px, -2px);
        }
        .news-image {
            width: 100%;
            height: 160px;
            margin-bottom: 14px;
            position: relative;
            overflow: hidden;
            border: 1px solid #2c2c2c;
        }
        .news-image::after {
            content: "";
            position: absolute;
            inset: 0;
            background: repeating-linear-gradient(45deg, transparent, transparent 10px, rgba(255,255,255,0.03) 10px, rgba(255,255,255,0.03) 20px);
            pointer-events: none;
        }
        .news-image-label {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            color: rgba(255,255,255,0.7);
            font-size: 14px;
            font-weight: 700;
            letter-spacing: 4px;
            text-transform: uppercase;
            font-variant: small-caps;
            text-shadow: 0 1px 3px rgba(0,0,0,0.5);
        }
        .news-kicker {
            display: block;
            font-size: 10px;
            font-weight: 700;
            letter-spacing: 3px;
            text-transform: uppercase;
            color: #8b0000;
            margin-bottom: 6px;
            padding-bottom: 3px;
            border-bottom: 2px solid #8b0000;
            width: fit-content;
            font-variant: small-caps;
        }
        .news-title {
            font-family: "Georgia", "Times New Roman", "Noto Serif SC", serif;
            font-size: 17px;
            font-weight: 900;
            line-height: 1.35;
            margin: 0 0 8px 0;
            color: #1a1a1a;
        }
        .news-title a {
            color: inherit;
            text-decoration: none;
        }
        .news-title a:hover {
            color: #8b0000;
            text-decoration: underline;
            text-decoration-thickness: 2px;
            text-underline-offset: 3px;
        }
        .news-summary {
            font-size: 13px;
            line-height: 1.75;
            color: #3a3a3a;
            margin: 0 0 12px 0;
            text-align: justify;
        }
        .news-meta {
            font-size: 10px;
            color: #6b6b6b;
            padding-top: 10px;
            border-top: 1px solid #2c2c2c;
            letter-spacing: 1px;
            font-variant: small-caps;
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
            gap: 6px;
        }
        .news-source {
            font-weight: 700;
            color: #8b0000;
        }
        .news-stance-tag {
            font-size: 10px;
            color: #8b0000;
            font-weight: 700;
            letter-spacing: 1px;
        }
        .newspaper-footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 3px double #1a1a1a;
            text-align: center;
            font-size: 11px;
            color: #6b6b6b;
            letter-spacing: 2px;
        }
        .newspaper-footer p {
            margin: 5px 0;
        }
        @media print {
            body { padding: 0; background: white; }
            .no-print { display: none; }
        }
        @media (max-width: 600px) {
            .news-grid { grid-template-columns: 1fr; }
            .featured-summary { column-count: 1; }
        }
    </style>
</head>
<body>
    <div class="newspaper-header">
        <div class="header-top">
            <span>Vol. ` + volNo + ` · No. ` + issueNo + `</span>
            <span>` + dateStr + `</span>
            <span>Price: Free</span>
        </div>
        <div class="header-title">
            <h1>` + html.EscapeString(options.Title) + `</h1>
            <div class="header-subtitle">ShiBoSi Daily · 市舶司昨日晚报</div>
        </div>
        <div class="header-bar">
            <span class="bar-item">Date: <span>` + dateStr + `</span></span>
            <span class="bar-divider">|</span>
            <span class="bar-item">Count: <span>` + fmt.Sprintf("%d", len(newsList)) + `</span></span>
            <span class="bar-divider">|</span>
            <span class="bar-item">Exported: <span>` + exportTime + `</span></span>
        </div>
    </div>
`)

	categories := make(map[string][]News)
	for _, n := range newsList {
		cat := n.Category
		if cat == "" {
			cat = "综合"
		}
		categories[cat] = append(categories[cat], n)
	}

	catOrder := []string{"时政", "财经", "科技", "体育", "社会", "国际", "军事", "教育", "健康", "娱乐", "汽车", "房产", "综合"}
	orderedCats := make([]string, 0)
	for _, c := range catOrder {
		if _, ok := categories[c]; ok {
			orderedCats = append(orderedCats, c)
		}
	}
	for c := range categories {
		found := false
		for _, oc := range orderedCats {
			if oc == c {
				found = true
				break
			}
		}
		if !found {
			orderedCats = append(orderedCats, c)
		}
	}

	articleIndex := 0

	for _, cat := range orderedCats {
		list := categories[cat]
		if len(list) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf(`    <div class="newspaper-section">
        <div class="section-header">
            <h2>%s</h2>
            <span class="section-count">%d 条</span>
        </div>
`, html.EscapeString(cat), len(list)))

		isFirstCategory := articleIndex == 0
		hasFeatured := isFirstCategory && len(list) > 2

		if hasFeatured {
			featuredNews := list[0]
			featuredTitle := html.EscapeString(featuredNews.Title)
			featuredSummary := html.EscapeString(featuredNews.Summary)
			if len(featuredSummary) > 400 {
				featuredSummary = featuredSummary[:400] + "..."
			}
			featuredSource := html.EscapeString(featuredNews.Source)
			featuredPubTime := featuredNews.PublishedAt
			featuredGradient := getGradient(articleIndex, featuredNews.Category)

			featuredBanner := getStanceBadgeHTML(featuredNews.PoliticalStance)
			featuredReliability := getReliabilityBadgeHTML(featuredNews)
			featuredConfidence := getConfidenceFromTags(featuredNews.AnalysisTags)
			if featuredConfidence != "" {
				featuredBanner += fmt.Sprintf(`<span class="analysis-badge info">置信度: %s</span>`, featuredConfidence)
			}
			featuredBanner += featuredReliability

			var featuredAnalysisBlock string
			if featuredNews.AnalysisSummary != "" {
				featuredAnalysisBlock = fmt.Sprintf(`<div class="news-analysis" style="margin-top:12px;"><span class="analysis-badge info" style="background:#2c2c2c;">📊 交叉分析</span></div>
                <p style="font-size:12px;color:#6b6b6b;margin:6px 0 0 0;font-style:italic;">%s</p>`, html.EscapeString(featuredNews.AnalysisSummary))
			}

			var featuredTitleHTML string
			if featuredNews.URL != "" {
				featuredTitleHTML = fmt.Sprintf(`<h2 class="featured-title"><a href="%s" target="_blank">%s</a></h2>`, html.EscapeString(featuredNews.URL), featuredTitle)
			} else {
				featuredTitleHTML = fmt.Sprintf(`<h2 class="featured-title">%s</h2>`, featuredTitle)
			}

			dropCapSummary := ""
			if len(featuredSummary) > 0 {
				firstChar := string([]rune(featuredSummary)[0])
				restChars := string([]rune(featuredSummary)[1:])
				dropCapSummary = fmt.Sprintf(`<p class="featured-summary"><span class="drop-cap">%s</span>%s</p>`, firstChar, restChars)
			}

			sb.WriteString(fmt.Sprintf(`        <div class="featured-article">
            <div class="featured-image" style="background:%s;">
                <span class="featured-image-label">%s</span>
            </div>
            <div class="news-analysis">
                %s
            </div>
            <span class="featured-kicker">%s</span>
            %s
            %s
            <div class="featured-meta">
                <span>来源：<span class="news-source">%s</span></span>
                <span>时间：%s</span>
            </div>
        </div>
`, featuredGradient, html.EscapeString(cat), featuredBanner, html.EscapeString(cat), featuredTitleHTML, dropCapSummary, featuredAnalysisBlock, featuredSource, featuredPubTime))

			list = list[1:]
		}

		sb.WriteString(`        <div class="news-grid">
`)

		for i, n := range list {
			if i >= 49 {
				break
			}

			title := html.EscapeString(n.Title)
			summary := html.EscapeString(n.Summary)
			if len(summary) > 250 {
				summary = summary[:250] + "..."
			}
			source := html.EscapeString(n.Source)
			pubTime := n.PublishedAt
			gradient := getGradient(articleIndex+i+1, n.Category)

			badge := getStanceBadgeHTML(n.PoliticalStance)
			reliabilityBadge := getReliabilityBadgeHTML(n)
			confidence := getConfidenceFromTags(n.AnalysisTags)
			if confidence != "" {
				badge += fmt.Sprintf(`<span class="analysis-badge info">置信度: %s</span>`, confidence)
			}
			badge += reliabilityBadge

			var titleHTML string
			if n.URL != "" {
				titleHTML = fmt.Sprintf(`<h3 class="news-title"><a href="%s" target="_blank">%s</a></h3>`, html.EscapeString(n.URL), title)
			} else {
				titleHTML = fmt.Sprintf(`<h3 class="news-title">%s</h3>`, title)
			}

			stanceTag := ""
			if label, ok := stanceLabelCN[n.PoliticalStance]; ok && n.PoliticalStance != "neutral" {
				stanceTag = fmt.Sprintf(`<span class="news-stance-tag">%s</span>`, label)
			}

			sb.WriteString(fmt.Sprintf(`            <div class="news-card">
                <div class="news-image" style="background:%s;">
                    <span class="news-image-label">%s</span>
                </div>
                <div class="news-analysis">
                    %s
                </div>
                <span class="news-kicker">%s</span>
                %s
                <p class="news-summary">%s</p>
                <div class="news-meta">
                    <span class="news-source">%s</span>
                    <span>🕐 %s</span>
                    %s
                </div>
            </div>
`, gradient, html.EscapeString(n.Category), badge, html.EscapeString(n.Category), titleHTML, summary, source, pubTime, stanceTag))
		}

		sb.WriteString(`        </div>
    </div>
`)
		articleIndex++
	}

	sb.WriteString(fmt.Sprintf(`    <div class="newspaper-footer">
        <p>本文由市舶司昨日晚报自动生成</p>
        <p>Generated by ShiBoSi Newspaper · %s</p>
    </div>
</body>
</html>`, exportTime))

	filename := fmt.Sprintf("newspaper_%s.html", time.Now().Format("20060102_%150405"))
	return filename, sb.String(), nil
}

func SaveExportToFile(filename, content string) (string, error) {
	exportDir := filepath.Join("..", "..", "db", "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", err
	}

	filePath := filepath.Join(exportDir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

func GenerateNewspaperPDF(newsList []News, options ExportOptions) (string, string, error) {
	filename, htmlContent, err := exportHTML(newsList, options)
	if err != nil {
		return "", "", err
	}

	return filename, htmlContent, nil
}
