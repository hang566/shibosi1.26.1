package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

type AnalysisResult struct {
	PoliticalStance string         `json:"political_stance"`
	PoliticalScore  float64        `json:"political_score"`
	ConfidenceScore float64        `json:"confidence_score"`
	HasPrivateAd    bool           `json:"has_private_ad"`
	AdScore         float64        `json:"ad_score"`
	AdConfidence    float64        `json:"ad_confidence"`
	TopicCategory   string         `json:"topic_category"`
	Reliability     string         `json:"reliability"`
	Methods         []MethodResult `json:"methods"`
	Tags            []string       `json:"tags"`
	Summary         string         `json:"summary"`
}

type MethodResult struct {
	Name   string  `json:"name"`
	Stance string  `json:"stance"`
	Score  float64 `json:"score"`
	Weight float64 `json:"weight"`
}

var (
	// === 左翼/保守/政府立场关键词 ===
	leftKeywords = []string{
		"党中央", "国务院", "习近平", "总书记", "中央委员会",
		"社会主义", "共产主义", "马克思主义", "毛泽东思想",
		"中国梦", "伟大复兴", "民族复兴", "伟大复兴",
		"正能量", "主旋律", "主流媒体", "舆论导向",
		"坚决拥护", "坚定支持", "全面贯彻", "深入学习",
		"国家战略", "国策", "体制", "机制", "制度",
		"统一思想", "提高认识", "加强领导", "落实到位",
		"科学发展", "和谐社会", "小康社会",
	}

	// === 右翼/自由/批评立场关键词 ===
	rightKeywords = []string{
		"民主", "自由", "人权", "公民权利",
		"抗议", "示威", "游行", "罢工",
		"独裁", "专制", "极权", "暴政",
		"腐败", "贪污", "渎职", "权钱交易",
		"镇压", "迫害", "审查", "封锁",
		"异见", "持不同政见", "政治犯",
		"改革", "变革", "转型", "自由化",
		"新闻自由", "言论自由", "结社自由",
		"选举", "公投", "宪政",
	}

	// === 中立/客观报道关键词 ===
	neutralKeywords = []string{
		"据悉", "据报道", "据消息人士", "据知情人士",
		"目前尚不清楚", "有待观察", "尚未确定",
		"专家表示", "业内人士称", "分析人士认为",
		"有观点认为", "也有观点指出",
		"一方面", "另一方面", "双方",
		"公开资料显示", "数据表明", "统计显示",
	}

	// === 正面情感关键词（倾向积极、建设性、官方话语） ===
	positiveSentimentKeywords = []string{
		"积极", "正面", "进步", "繁荣", "稳定", "和谐",
		"发展", "成功", "突破", "创新", "改革", "成就",
		"贡献", "合作", "共赢", "机遇", "希望", "光明",
		"伟大", "卓越", "杰出", "显著", "重要", "关键",
		"高效", "优质", "领先", "优势", "潜力", "前景",
		"改善", "提升", "优化", "完善", "推进", "促进",
		"保障", "维护", "巩固", "加强", "深化", "拓展",
	}

	// === 负面情感关键词（倾向消极、批评、揭露） ===
	negativeSentimentKeywords = []string{
		"问题", "危机", "失败", "腐败", "压迫", "不公",
		"歧视", "暴力", "冲突", "灾难", "衰退", "失业",
		"贫困", "矛盾", "抗议", "谴责", "批评", "质疑",
		"揭露", "曝光", "内幕", "黑幕", "丑闻", "风波",
		"隐患", "风险", "威胁", "危害", "损害", "损失",
		"延误", "失误", "过失", "缺陷", "漏洞", "违规",
		"侵犯", "践踏", "剥夺", "限制", "控制", "干预",
	}

	// === 可信/官方来源模式 ===
	credibleSourcePatterns = []string{
		"人民日报", "新华社", "央视", "中央电视台", "光明日报",
		"环球时报", "求是", "人民网", "新华网", "中国网",
		"央广", "中央人民广播电台", "中国日报", "参考消息",
		"解放军报", "中国纪检监察", "经济日报", "科技日报",
	}

	// === 可疑/独立来源模式 ===
	questionableSourcePatterns = []string{
		"自由", "民主", "维权", "异见", "独立", "民间",
		"草根", "真相", "爆料", "揭秘", "内幕", "独家",
		"小道消息", "匿名", "网传", "传言", "据传",
		"博客", "个人", "自媒体", "公众号", "微博",
	}

	// === 话题分类关键词 ===
	topicKeywords = map[string][]string{
		"政治": {
			"政府", "政策", "政治", "选举", "领导", "党",
			"法规", "立法", "外交", "峰会", "总统", "议会",
			"首相", "内阁", "执政", "在野", "竞选", "公投",
		},
		"经济": {
			"经济", "金融", "股市", "贸易", "汇率", "GDP",
			"通胀", "就业", "产业", "投资", "银行", "货币",
			"财政", "税收", "企业", "市场", "消费", "出口",
		},
		"社会": {
			"社会", "民生", "教育", "医疗", "住房", "交通",
			"环保", "人口", "治安", "养老", "就业", "扶贫",
			"保障", "福利", "城市", "农村", "社区", "公益",
		},
		"科技": {
			"科技", "技术", "互联网", "AI", "芯片", "创新",
			"研发", "算法", "数据", "人工智能", "5G", "区块链",
			"云计算", "大数据", "物联网", "机器人", "量子",
		},
		"军事": {
			"军事", "国防", "军队", "武器", "演习", "战略",
			"安全", "导弹", "航母", "战斗机", "将军", "战区",
			"防务", "军备", "冲突", "战争", "战斗",
		},
		"国际": {
			"国际", "外交", "联合国", "国际关系", "外国", "全球",
			"世界", "各国", "国际组织", "多边", "双边",
			"大使馆", "领事馆", "制裁", "条约", "协议",
		},
		"体育": {
			"体育", "足球", "篮球", "奥运", "比赛", "冠军",
			"联赛", "球员", "教练", "锦标赛", "世界杯", "奥运",
		},
		"娱乐": {
			"娱乐", "电影", "电视", "明星", "歌手", "演员",
			"综艺", "选秀", "演唱会", "电影节", "颁奖典礼",
		},
	}

	// === 广告/私活检测模式 ===
	privateAdPatterns = []string{
		"点击查看", "立即下载", "限时优惠", "免费领取",
		"扫码关注", "添加微信", "加QQ", "私信我",
		"独家揭秘", "内部消息", "内幕曝光",
		"付费阅读", "打赏支持", "赞助内容",
		"推广内容", "广告", "赞助商",
		"福利来了", "好康道相报", "隐藏内容",
		"VIP专属", "会员专享", "付费解锁",
		"获取完整内容", "查看更多",
	}

	adDomainPatterns = regexp.MustCompile(`(?i)(\.vip|\.top|\.xyz|\.click|\.link|\.site|\.online|\.club)`)
	urlPattern       = regexp.MustCompile(`https?://[^\s]+`)
)

// ============================================================================
// 主入口函数：交叉验证分析
// ============================================================================

func AnalyzeNews(n News) AnalysisResult {
	text := strings.ToLower(n.Title + " " + n.Summary + " " + n.Content)

	// ---- 方法 1：关键词密度分析 ----
	m1Score := analyzePoliticalStance(text)
	m1 := MethodResult{
		Name:   "关键词密度分析",
		Stance: getStanceLabel(m1Score),
		Score:  m1Score,
		Weight: 0.25,
	}

	// ---- 方法 2：情感分析 ----
	m2Score := analyzeSentiment(text)
	m2 := MethodResult{
		Name:   "情感分析",
		Stance: getStanceLabel(m2Score),
		Score:  m2Score,
		Weight: 0.20,
	}

	// ---- 方法 3：来源信誉评估 ----
	m3Score := analyzeSourceReputation(n.Source, n.Category)
	m3 := MethodResult{
		Name:   "来源信誉评估",
		Stance: getStanceLabel(m3Score),
		Score:  m3Score,
		Weight: 0.20,
	}

	// ---- 方法 4：内容结构分析 ----
	m4Score := analyzeContentStructure(n.Title, n.Content, text)
	m4 := MethodResult{
		Name:   "内容结构分析",
		Stance: getStanceLabel(m4Score),
		Score:  m4Score,
		Weight: 0.20,
	}

	// ---- 方法 5：话题分类 ----
	topic := classifyTopic(text)
	m5Score := analyzeTopicStance(topic, text)
	m5 := MethodResult{
		Name:   "话题分类分析",
		Stance: getStanceLabel(m5Score),
		Score:  m5Score,
		Weight: 0.15,
	}

	// ---- 交叉验证：加权共识 ----
	methods := []MethodResult{m1, m2, m3, m4, m5}
	finalScore := weightedConsensus(methods)
	confidence := calculateConfidence(methods)
	reliability := getReliabilityLabel(confidence)

	result := AnalysisResult{
		PoliticalStance: getStanceLabel(finalScore),
		PoliticalScore:  finalScore,
		ConfidenceScore: confidence,
		TopicCategory:   topic,
		Reliability:     reliability,
		Methods:         methods,
	}

	// ---- 私活检测 ----
	result.AdScore = detectPrivateContent(n.Title, n.URL, n.Content)
	result.HasPrivateAd = result.AdScore > 0.5
	result.AdConfidence = calculateAdConfidence(n.Title, n.URL, n.Content, result.AdScore)

	// ---- 生成标签 ----
	result.Tags = generateTags(n, result)

	// ---- 生成分析摘要 ----
	result.Summary = generateSummary(n, result)

	return result
}

// ============================================================================
// 方法 1：关键词密度分析（原有逻辑，增强）
// ============================================================================

func analyzePoliticalStance(text string) float64 {
	text = strings.ToLower(text)

	leftCount := 0
	rightCount := 0
	neutralCount := 0

	for _, kw := range leftKeywords {
		leftCount += strings.Count(text, strings.ToLower(kw))
	}

	for _, kw := range rightKeywords {
		rightCount += strings.Count(text, strings.ToLower(kw))
	}

	for _, kw := range neutralKeywords {
		neutralCount += strings.Count(text, strings.ToLower(kw))
	}

	total := leftCount + rightCount + neutralCount
	if total == 0 {
		return 0
	}

	score := float64(rightCount-leftCount) / float64(total+1)

	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	if neutralCount > 0 {
		neutralFactor := float64(neutralCount) / float64(total+1) * 0.3
		if score > 0 {
			score -= neutralFactor
		} else if score < 0 {
			score += neutralFactor
		}
	}

	return score
}

// ============================================================================
// 方法 2：情感分析（正面/负面词计数）
// ============================================================================

func analyzeSentiment(text string) float64 {
	text = strings.ToLower(text)

	posCount := 0
	negCount := 0

	for _, kw := range positiveSentimentKeywords {
		posCount += strings.Count(text, strings.ToLower(kw))
	}

	for _, kw := range negativeSentimentKeywords {
		negCount += strings.Count(text, strings.ToLower(kw))
	}

	total := posCount + negCount
	if total == 0 {
		return 0
	}

	score := float64(negCount-posCount) / float64(total+1)

	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	return score
}

// ============================================================================
// 方法 3：来源信誉评估
// ============================================================================

func analyzeSourceReputation(source, category string) float64 {
	lowerSource := strings.ToLower(source)
	lowerCategory := strings.ToLower(category)

	score := 0.0

	for _, pattern := range credibleSourcePatterns {
		if strings.Contains(lowerSource, strings.ToLower(pattern)) {
			score -= 0.4
			break
		}
	}

	for _, pattern := range questionableSourcePatterns {
		if strings.Contains(lowerSource, strings.ToLower(pattern)) {
			score += 0.4
			break
		}
	}

	if lowerCategory == "时政" || lowerCategory == "政治" {
		score *= 1.2
	} else if lowerCategory == "财经" || lowerCategory == "经济" {
		score *= 0.8
	}

	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}

	return score
}

// ============================================================================
// 方法 4：内容结构分析
// ============================================================================

func analyzeContentStructure(title, content, fullText string) float64 {
	titleLower := strings.ToLower(title)
	contentLower := strings.ToLower(content)

	titleScore := 0.0
	bodyScore := 0.0

	for _, kw := range leftKeywords {
		titleScore -= float64(strings.Count(titleLower, strings.ToLower(kw)))
	}
	for _, kw := range rightKeywords {
		titleScore += float64(strings.Count(titleLower, strings.ToLower(kw)))
	}
	for _, kw := range leftKeywords {
		bodyScore -= float64(strings.Count(contentLower, strings.ToLower(kw)))
	}
	for _, kw := range rightKeywords {
		bodyScore += float64(strings.Count(contentLower, strings.ToLower(kw)))
	}

	titleNorm := normalizeScore(titleScore, 3.0)
	bodyNorm := normalizeScore(bodyScore, 5.0)

	structureScore := (titleNorm + bodyNorm) / 2.0

	titleLen := len(strings.TrimSpace(title))
	if titleLen > 0 && titleLen < 10 {
		structureScore *= 0.7
	}

	adjectives := []string{"惊人", "绝秘", "难以置信", "震惊", "恐怖", "可怕"}
	clickbaitPenalty := 0.0
	for _, adj := range adjectives {
		if strings.Contains(titleLower, strings.ToLower(adj)) {
			clickbaitPenalty += 0.1
		}
	}
	if clickbaitPenalty > 0 {
		if structureScore > 0 {
			structureScore -= clickbaitPenalty
		} else if structureScore < 0 {
			structureScore += clickbaitPenalty
		}
	}

	neutralMarkerCount := 0
	for _, kw := range neutralKeywords {
		neutralMarkerCount += strings.Count(fullText, strings.ToLower(kw))
	}
	if neutralMarkerCount >= 3 {
		structureScore *= 0.5
	}

	if structureScore > 1 {
		structureScore = 1
	} else if structureScore < -1 {
		structureScore = -1
	}

	return structureScore
}

// ============================================================================
// 方法 5：话题分类与立场分析
// ============================================================================

func classifyTopic(text string) string {
	text = strings.ToLower(text)

	bestTopic := "综合"
	bestCount := 0

	for topic, keywords := range topicKeywords {
		count := 0
		for _, kw := range keywords {
			count += strings.Count(text, strings.ToLower(kw))
		}
		if count > bestCount {
			bestCount = count
			bestTopic = topic
		}
	}

	return bestTopic
}

func analyzeTopicStance(topic string, text string) float64 {
	text = strings.ToLower(text)

	topicScore := 0.0

	switch topic {
	case "政治":
		topicScore = analyzePoliticalStance(text) * 1.1
	case "经济":
		econKeywords := []string{"增长", "衰退", "通胀", "繁荣", "危机", "复苏", "过热", "泡沫"}
		count := 0
		for _, kw := range econKeywords {
			count += strings.Count(text, strings.ToLower(kw))
		}
		if count > 0 {
			topicScore = 0.1 * float64(count)
		}
	case "社会":
		socialPos := 0
		socialNeg := 0
		socialPosWords := []string{"改善", "提升", "保障", "进步", "和谐", "稳定"}
		socialNegWords := []string{"问题", "矛盾", "冲突", "不公", "贫困", "压迫"}
		for _, w := range socialPosWords {
			socialPos += strings.Count(text, strings.ToLower(w))
		}
		for _, w := range socialNegWords {
			socialNeg += strings.Count(text, strings.ToLower(w))
		}
		total := socialPos + socialNeg
		if total > 0 {
			topicScore = float64(socialNeg-socialPos) / float64(total+1)
		}
	case "军事", "国际":
		topicScore = analyzePoliticalStance(text) * 0.9
	default:
		topicScore = analyzePoliticalStance(text) * 0.6
	}

	if topicScore > 1 {
		topicScore = 1
	} else if topicScore < -1 {
		topicScore = -1
	}

	return topicScore
}

// ============================================================================
// 交叉验证：加权共识算法
// ============================================================================

func weightedConsensus(methods []MethodResult) float64 {
	totalWeight := 0.0
	weightedSum := 0.0

	for _, m := range methods {
		weightedSum += m.Score * m.Weight
		totalWeight += m.Weight
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}

func calculateConfidence(methods []MethodResult) float64 {
	if len(methods) == 0 {
		return 0
	}

	scores := make([]float64, len(methods))
	for i, m := range methods {
		scores[i] = m.Score
	}

	mean := 0.0
	for _, s := range scores {
		mean += s
	}
	mean /= float64(len(scores))

	variance := 0.0
	for _, s := range scores {
		diff := s - mean
		variance += diff * diff
	}
	variance /= float64(len(scores))

	stdDev := math.Sqrt(variance)

	maxStdDev := 1.0
	confidence := 1.0 - (stdDev / maxStdDev)

	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	return confidence
}

func getReliabilityLabel(confidence float64) string {
	if confidence >= 0.7 {
		return "reliable"
	} else if confidence >= 0.4 {
		return "mixed"
	}
	return "unreliable"
}

// ============================================================================
// 辅助函数
// ============================================================================

func normalizeScore(raw, maxAbs float64) float64 {
	if maxAbs <= 0 {
		return 0
	}
	score := raw / maxAbs
	if score > 1 {
		score = 1
	} else if score < -1 {
		score = -1
	}
	return score
}

func calculateAdConfidence(title, url, content string, adScore float64) float64 {
	indicators := 0.0
	totalIndicators := 5

	text := strings.ToLower(title + " " + content)

	adKeywordCount := 0
	for _, pattern := range privateAdPatterns {
		if strings.Contains(text, strings.ToLower(pattern)) {
			adKeywordCount++
		}
	}
	if adKeywordCount >= 3 {
		indicators += 1
	} else if adKeywordCount >= 1 {
		indicators += 0.5
	}

	if adDomainPatterns.MatchString(url) {
		indicators += 1
	}

	cleanText := strings.TrimSpace(content)
	if len(cleanText) < 30 {
		indicators += 1
	}

	urls := urlPattern.FindAllString(content, -1)
	if len(urls) > 5 {
		indicators += 1
	}

	suspiciousSources := []string{"blog", "blogspot", "wordpress", "weibo", "twitter", "facebook"}
	lowerURL := strings.ToLower(url)
	for _, src := range suspiciousSources {
		if strings.Contains(lowerURL, src) {
			indicators += 1
			break
		}
	}

	confidence := float64(indicators) / float64(totalIndicators)

	if adScore < 0.1 {
		confidence = confidence * 0.3
	}

	if confidence > 1 {
		confidence = 1
	}
	if confidence < 0 {
		confidence = 0
	}

	return confidence
}

// ============================================================================
// 原有关键函数（保持签名兼容）
// ============================================================================

func getStanceLabel(score float64) string {
	if score <= -0.3 {
		return "left"
	} else if score >= 0.3 {
		return "right"
	} else if score <= -0.15 {
		return "center-left"
	} else if score >= 0.15 {
		return "center-right"
	}
	return "neutral"
}

func detectPrivateContent(title, url, content string) float64 {
	score := 0.0
	text := strings.ToLower(title + " " + content)

	adKeywordCount := 0
	for _, pattern := range privateAdPatterns {
		if strings.Contains(text, strings.ToLower(pattern)) {
			adKeywordCount++
		}
	}
	if adKeywordCount > 0 {
		score += float64(adKeywordCount) * 0.15
	}

	if adDomainPatterns.MatchString(url) {
		score += 0.3
	}

	cleanText := strings.TrimSpace(content)
	if len(cleanText) < 30 {
		score += 0.3
	} else if len(cleanText) < 100 {
		score += 0.15
	}

	urls := urlPattern.FindAllString(content, -1)
	if len(urls) > 5 {
		score += 0.2
	}
	if len(urls) > 10 {
		score += 0.3
	}

	suspiciousSources := []string{"blog", "blogspot", "wordpress", "weibo", "twitter", "facebook"}
	lowerURL := strings.ToLower(url)
	for _, src := range suspiciousSources {
		if strings.Contains(lowerURL, src) {
			score += 0.15
			break
		}
	}

	if score > 1 {
		score = 1
	}
	return score
}

func generateTags(n News, result AnalysisResult) []string {
	tags := []string{}

	switch result.PoliticalStance {
	case "left":
		tags = append(tags, "倾向:保守/政府立场")
	case "right":
		tags = append(tags, "倾向:自由/批评立场")
	case "center-left":
		tags = append(tags, "倾向:中左")
	case "center-right":
		tags = append(tags, "倾向:中右")
	case "neutral":
		tags = append(tags, "倾向:中立")
	}

	if result.HasPrivateAd {
		tags = append(tags, "⚠️ 疑似夹带私活")
	}

	if result.TopicCategory != "" && result.TopicCategory != "综合" {
		tags = append(tags, "话题:"+result.TopicCategory)
	}

	switch result.Reliability {
	case "reliable":
		tags = append(tags, "可信度:高")
	case "mixed":
		tags = append(tags, "可信度:中等")
	case "unreliable":
		tags = append(tags, "可信度:低")
	}

	if n.Category != "" {
		tags = append(tags, n.Category)
	}

	return tags
}

func generateSummary(n News, result AnalysisResult) string {
	var sb strings.Builder

	switch result.PoliticalStance {
	case "left":
		sb.WriteString("【分析】该新闻呈现明显的保守/政府立场倾向，")
	case "right":
		sb.WriteString("【分析】该新闻呈现明显的自由/批评立场倾向，")
	case "center-left":
		sb.WriteString("【分析】该新闻略倾向于保守立场，")
	case "center-right":
		sb.WriteString("【分析】该新闻略倾向于自由立场，")
	default:
		sb.WriteString("【分析】该新闻立场较为中立，")
	}

	sb.WriteString("政治倾向分数: ")
	sb.WriteString(formatScore(result.PoliticalScore))

	if result.TopicCategory != "" && result.TopicCategory != "综合" {
		sb.WriteString("。涉及话题: ")
		sb.WriteString(result.TopicCategory)
	}

	sb.WriteString("。交叉验证置信度: ")
	sb.WriteString(formatScore(result.ConfidenceScore))

	switch result.Reliability {
	case "reliable":
		sb.WriteString("。综合判定: 结论可靠。")
	case "mixed":
		sb.WriteString("。综合判定: 各分析方法存在分歧，结论仅供参考。")
	case "unreliable":
		sb.WriteString("。综合判定: 分析方法分歧较大，结论可信度低。")
	}

	if result.HasPrivateAd {
		sb.WriteString("。⚠️ 检测到可能的私活/广告内容，可信度较低。")
	} else if result.AdScore > 0.3 {
		sb.WriteString("。部分内容可能包含推广信息。")
	}

	return sb.String()
}

// ============================================================================
// 格式化辅助函数（保持原有实现）
// ============================================================================

func formatScore(score float64) string {
	if score > 0 {
		return fmt.Sprintf("+%.2f", score)
	} else if score < 0 {
		return fmt.Sprintf("%.2f", score)
	}
	return "0.00"
}

func sprintf(format string, args ...interface{}) string {
	result := format
	for i, arg := range args {
		switch v := arg.(type) {
		case float64:
			result = strings.Replace(result, "%f", fmtFloat(v), 1)
			if i < len(args)-1 {
				result = strings.Replace(result, ".00", "", 1)
				result = strings.Replace(result, ".0", "", 1)
			}
		}
	}
	return result
}

func fmtFloat(f float64) string {
	if f == 0 {
		return "0.00"
	}
	neg := false
	if f < 0 {
		neg = true
		f = -f
	}
	intPart := int64(f)
	decPart := int64((f - float64(intPart)) * 100)
	if decPart < 0 {
		decPart = -decPart
	}
	s := itoa(intPart) + "."
	if decPart < 10 {
		s += "0"
	}
	s += itoa(decPart)
	if neg {
		s = "-" + s
	}
	return s
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
