package service

import (
	"admin-core/internal/dao"
	"admin-core/internal/model"
	"admin-core/internal/ws"
	"context"
	"encoding/json"
	"fmt"
	stdLog "log"
	"math/rand"
	"sync"
	"time"

	"gorm.io/gorm"
)

// BotService 算法机器人服务
type BotService struct {
	db      *gorm.DB
	da      *dao.DAO
	wsHub   *ws.Hub
	running map[int64]*botRuntime
	mu      sync.RWMutex
}

type botRuntime struct {
	bot     *model.Bot
	cancel  context.CancelFunc
	running bool
}

func NewBotService(db *gorm.DB, da *dao.DAO, hub *ws.Hub) *BotService {
	return &BotService{
		db:      db,
		da:      da,
		wsHub:   hub,
		running: make(map[int64]*botRuntime),
	}
}

// List 列出所有机器人
func (s *BotService) List() ([]model.Bot, error) {
	list, err := s.da.ListBots()
	if err == nil && len(list) > 0 {
		return list, nil
	}
	// 如果数据库为空，返回预定义机器人目录
	return defaultBotCatalog(), nil
}

// Get 获取单个机器人
func (s *BotService) Get(id int64) (*model.Bot, error) {
	return s.da.GetBot(id)
}

// Create 创建机器人
func (s *BotService) Create(bot *model.Bot) error {
	if bot.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	now := time.Now()
	bot.CreatedAt = now
	bot.UpdatedAt = now
	if bot.Status == "" {
		bot.Status = model.BotStatusStopped
	}
	if bot.Concurrency <= 0 {
		bot.Concurrency = 1
	}
	if bot.Timeout <= 0 {
		bot.Timeout = 30
	}
	if bot.RetryCount <= 0 {
		bot.RetryCount = 3
	}
	return s.da.CreateBot(bot)
}

// Update 更新机器人
func (s *BotService) Update(bot *model.Bot) error {
	bot.UpdatedAt = time.Now()
	return s.da.UpdateBot(bot)
}

// Delete 删除机器人
func (s *BotService) Delete(id int64) error {
	s.Stop(id) // 先停止
	return s.da.DeleteBot(id)
}

// Start 启动机器人
func (s *BotService) Start(id int64) error {
	bot, err := s.da.GetBot(id)
	if err != nil {
		return fmt.Errorf("机器人不存在")
	}
	if bot.Status == model.BotStatusRunning {
		return fmt.Errorf("机器人已在运行")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[id] = &botRuntime{bot: bot, cancel: cancel, running: true}
	s.mu.Unlock()

	// 更新状态
	bot.Status = model.BotStatusRunning
	bot.UpdatedAt = time.Now()
	s.da.UpdateBot(bot)

	// 记录日志
	s.createLog(bot, model.BotLogInfo, "机器人启动", "开始执行任务", 0, "success")

	// 广播状态变更
	if s.wsHub != nil {
		s.wsHub.Publish("bot:status", "update", map[string]interface{}{
			"id":     id,
			"status": "running",
			"time":   time.Now(),
		})
	}

	// 启动后台任务
	go s.runBotLoop(ctx, bot)

	return nil
}

// Stop 停止机器人
func (s *BotService) Stop(id int64) error {
	s.mu.Lock()
	rt, exists := s.running[id]
	if exists {
		rt.cancel()
		delete(s.running, id)
	}
	s.mu.Unlock()

	bot, err := s.da.GetBot(id)
	if err != nil {
		return nil // 机器人可能已被删除
	}

	bot.Status = model.BotStatusStopped
	bot.UpdatedAt = time.Now()
	s.da.UpdateBot(bot)

	s.createLog(bot, model.BotLogInfo, "机器人停止", "手动停止", 0, "success")

	if s.wsHub != nil {
		s.wsHub.Publish("bot:status", "update", map[string]interface{}{
			"id":     id,
			"status": "stopped",
			"time":   time.Now(),
		})
	}

	return nil
}

// StopAll 停止所有机器人
func (s *BotService) StopAll() {
	s.mu.RLock()
	ids := make([]int64, 0, len(s.running))
	for id := range s.running {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	for _, id := range ids {
		s.Stop(id)
	}
}

// StartAll 启动所有已启用的机器人
func (s *BotService) StartAll() {
	list, err := s.da.ListBots()
	if err != nil {
		return
	}
	for _, bot := range list {
		if bot.Enabled && bot.Status != model.BotStatusRunning {
			s.Start(bot.ID)
		}
	}
}

// Trigger 手动触发机器人执行一次
func (s *BotService) Trigger(id int64) error {
	bot, err := s.da.GetBot(id)
	if err != nil {
		return fmt.Errorf("机器人不存在")
	}

	ctx := context.Background()
	go s.runBotOnce(ctx, bot)
	return nil
}

// AutoConfig 自动配置优化
func (s *BotService) AutoConfig(mode string) map[string]interface{} {
	result := map[string]interface{}{
		"mode":          mode,
		"time":          time.Now().Format("2006-01-02 15:04:05"),
		"status":        "success",
		"optimizations": []string{},
	}

	list, _ := s.da.ListBots()
	optimizations := make([]string, 0)

	for _, bot := range list {
		switch mode {
		case "full", "crawler", "analyzer", "schedule":
			if mode == "crawler" && bot.Type != model.BotTypeCrawler {
				continue
			}
			if mode == "analyzer" && bot.Type != model.BotTypeAnalyzer {
				continue
			}

			// 根据类型优化配置
			updated := false
			switch bot.Type {
			case model.BotTypeCrawler:
				if bot.Concurrency < 3 {
					bot.Concurrency = 3
					updated = true
					optimizations = append(optimizations, fmt.Sprintf("爬虫 %s: 并发数调整为 3", bot.DisplayName))
				}
				if bot.Timeout < 60 {
					bot.Timeout = 60
					updated = true
					optimizations = append(optimizations, fmt.Sprintf("爬虫 %s: 超时调整为 60s", bot.DisplayName))
				}
			case model.BotTypeAnalyzer:
				if bot.Concurrency < 2 {
					bot.Concurrency = 2
					updated = true
					optimizations = append(optimizations, fmt.Sprintf("分析器 %s: 并发数调整为 2", bot.DisplayName))
				}
			case model.BotTypeSecurity:
				if bot.RetryCount > 5 {
					bot.RetryCount = 5
					updated = true
					optimizations = append(optimizations, fmt.Sprintf("安全机器人 %s: 重试次数优化", bot.DisplayName))
				}
			}

			if mode == "schedule" {
				bot.Priority = int((bot.ID % 10) + 1)
				updated = true
				optimizations = append(optimizations, fmt.Sprintf("机器人 %s: 优先级调整为 %d", bot.DisplayName, bot.Priority))
			}

			if updated {
				bot.UpdatedAt = time.Now()
				s.da.UpdateBot(&bot)
			}
		}
	}

	result["optimizations"] = optimizations
	result["optimized_count"] = len(optimizations)
	return result
}

// GetStats 获取机器人统计
func (s *BotService) GetStats() (map[string]interface{}, error) {
	stats, err := s.da.GetBotStats()
	if err != nil {
		return nil, err
	}
	logStats, _ := s.da.GetBotLogStats()
	stats["logs"] = logStats
	stats["timestamp"] = time.Now().Format("2006-01-02 15:04:05")
	return stats, nil
}

// ListLogs 获取机器人日志
func (s *BotService) ListLogs(botID int64, limit int) ([]model.BotLog, error) {
	return s.da.ListBotLogs(botID, limit)
}

// CleanLogs 清理日志
func (s *BotService) CleanLogs(botID int64, days int) error {
	return s.da.CleanBotLogs(botID, days)
}

// ListConfigs 获取配置列表
func (s *BotService) ListConfigs() ([]model.BotConfig, error) {
	return s.da.ListBotConfigs()
}

// SaveConfig 保存配置
func (s *BotService) SaveConfig(cfg *model.BotConfig) error {
	now := time.Now()
	if cfg.ID == 0 {
		cfg.CreatedAt = now
		return s.da.CreateBotConfig(cfg)
	}
	cfg.UpdatedAt = now
	return s.da.UpdateBotConfig(cfg)
}

// DeleteConfig 删除配置
func (s *BotService) DeleteConfig(id int64) error {
	return s.da.DeleteBotConfig(id)
}

// ============ 内部方法 ============

func (s *BotService) runBotLoop(ctx context.Context, bot *model.Bot) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runBotOnce(ctx, bot)
		}
	}
}

func (s *BotService) runBotOnce(ctx context.Context, bot *model.Bot) {
	startTime := time.Now()

	// 模拟执行
	time.Sleep(time.Duration(500+rand.Intn(1500)) * time.Millisecond)

	select {
	case <-ctx.Done():
		return
	default:
	}

	// 随机成功/失败
	success := rand.Float64() > 0.15
	duration := time.Since(startTime).Milliseconds()

	// 更新统计
	bot, err := s.da.GetBot(bot.ID)
	if err == nil {
		bot.RunCount++
		if success {
			bot.SuccessCount++
		} else {
			bot.FailCount++
		}
		total := bot.SuccessCount + bot.FailCount
		if total > 0 {
			bot.SuccessRate = float64(bot.SuccessCount) / float64(total) * 100
		}
		bot.LastRun = &startTime
		if success {
			bot.LastStatus = "success"
			bot.LastError = ""
		} else {
			bot.LastStatus = "failed"
			bot.LastError = "执行超时或外部服务不可用"
		}
		bot.CPUUsage = 10 + rand.Float64()*40
		bot.MemoryUsage = 50 + int64(rand.Intn(200))
		bot.NetworkIO = 1000 + int64(rand.Intn(5000))
		bot.UpdatedAt = time.Now()
		s.da.UpdateBot(bot)
	}

	// 记录日志
	level := model.BotLogInfo
	if !success {
		level = model.BotLogError
	}
	s.createLog(bot, level, s.getBotAction(bot), s.getBotDetail(bot, success), duration, map[bool]string{true: "success", false: "failed"}[success])

	// 推送更新
	if s.wsHub != nil {
		s.wsHub.Publish("bot:update", "status", map[string]interface{}{
			"id":            bot.ID,
			"run_count":     bot.RunCount,
			"success_count": bot.SuccessCount,
			"fail_count":    bot.FailCount,
			"success_rate":  bot.SuccessRate,
			"cpu_usage":     bot.CPUUsage,
			"memory_usage":  bot.MemoryUsage,
			"last_status":   bot.LastStatus,
			"last_run":      bot.LastRun,
		})
	}
}

func (s *BotService) createLog(bot *model.Bot, level, message, detail string, duration int64, status string) {
	botLog := &model.BotLog{
		BotID:     bot.ID,
		BotName:   bot.DisplayName,
		Level:     level,
		Message:   message,
		Detail:    detail,
		Duration:  duration,
		Status:    status,
		CreatedAt: time.Now(),
	}
	if err := s.da.CreateBotLog(botLog); err != nil {
		stdLog.Printf("[bot] create log error: %v", err)
	}

	// 广播日志
	if s.wsHub != nil {
		s.wsHub.Publish("bot:log", "new", botLog)
	}
}

func (s *BotService) getBotAction(bot *model.Bot) string {
	actions := map[string][]string{
		model.BotTypeCrawler:   {"抓取数据", "解析页面", "下载资源"},
		model.BotTypeAnalyzer:  {"分析数据", "生成报告", "运行算法"},
		model.BotTypeScheduler: {"调度任务", "检查状态", "分发任务"},
		model.BotTypeNotifier:  {"发送通知", "推送消息", "触发告警"},
		model.BotTypeSecurity:  {"扫描端口", "检测攻击", "封禁IP"},
		model.BotTypeAIAgent:   {"处理请求", "生成响应", "学习模型"},
	}
	if actions[bot.Type] != nil {
		list := actions[bot.Type]
		return list[rand.Intn(len(list))]
	}
	return "执行任务"
}

func (s *BotService) getBotDetail(bot *model.Bot, success bool) string {
	if success {
		details := map[string][]string{
			model.BotTypeCrawler:   {"成功抓取 100 条数据", "解析 50 个页面", "下载完成"},
			model.BotTypeAnalyzer:  {"分析完成，生成报告", "算法执行成功", "数据处理完毕"},
			model.BotTypeScheduler: {"任务调度完成", "检查所有节点正常", "分发 10 个任务"},
			model.BotTypeNotifier:  {"发送 5 条通知", "推送成功", "告警触发"},
			model.BotTypeSecurity:  {"扫描 1000 个端口", "检测到 0 个异常", "封禁 0 个 IP"},
			model.BotTypeAIAgent:   {"处理 20 个请求", "生成响应", "模型推理完成"},
		}
		if details[bot.Type] != nil {
			list := details[bot.Type]
			return list[rand.Intn(len(list))]
		}
		return "执行成功"
	} else {
		return "执行失败：外部服务暂时不可用或超时"
	}
}

// defaultBotCatalog 预定义机器人目录
func defaultBotCatalog() []model.Bot {
	now := time.Now()
	bots := []model.Bot{
		{
			Name: "新闻爬虫-通用", DisplayName: "新闻爬虫-通用",
			Type: model.BotTypeCrawler, Schedule: "每10分钟",
			Config:      `{"urls":["http://news.example.com"],"depth":2,"timeout":30}`,
			Description: "通用新闻网页爬虫，支持多源抓取", Icon: "🕷️",
			Priority: 5, Concurrency: 3, Timeout: 60, RetryCount: 3,
			Status: model.BotStatusStopped, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "舆情分析器", DisplayName: "舆情分析器",
			Type: model.BotTypeAnalyzer, Schedule: "每30分钟",
			Config:      `{"model":"sentiment","languages":["zh","en"],"batch_size":100}`,
			Description: "实时舆情情感分析，支持中英文", Icon: "📊",
			Priority: 4, Concurrency: 2, Timeout: 120, RetryCount: 3,
			Status: model.BotStatusStopped, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "智能调度器", DisplayName: "智能调度器",
			Type: model.BotTypeScheduler, Schedule: "每5分钟",
			Config:      `{"strategy":"round_robin","max_tasks":50,"priority_levels":5}`,
			Description: "任务智能调度，支持优先级队列", Icon: "⏰",
			Priority: 10, Concurrency: 5, Timeout: 30, RetryCount: 5,
			Status: model.BotStatusStopped, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "异常通知器", DisplayName: "异常通知器",
			Type: model.BotTypeNotifier, Schedule: "实时",
			Config:      `{"channels":["webhook","email","sms"],"threshold":"warning"}`,
			Description: "系统异常实时通知，支持多渠道", Icon: "🔔",
			Priority: 8, Concurrency: 1, Timeout: 10, RetryCount: 3,
			Status: model.BotStatusStopped, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "网络安全监控", DisplayName: "网络安全监控",
			Type: model.BotTypeSecurity, Schedule: "每1分钟",
			Config:      `{"scan_ports":[22,80,443,8080],"block_threshold":5,"alert_levels":["warning","critical"]}`,
			Description: "实时网络攻击检测与自动防御", Icon: "🛡️",
			Priority: 10, Concurrency: 4, Timeout: 15, RetryCount: 5,
			Status: model.BotStatusStopped, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			Name: "AI智能助手", DisplayName: "AI智能助手",
			Type: model.BotTypeAIAgent, Schedule: "实时",
			Config:      `{"model":"gpt-4","temperature":0.7,"max_tokens":4096,"memory":true}`,
			Description: "AI智能助手，支持自然语言交互", Icon: "🧠",
			Priority: 6, Concurrency: 2, Timeout: 60, RetryCount: 2,
			Status: model.BotStatusStopped, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		},
	}
	for i := range bots {
		bots[i].SuccessRate = 95.0 + float64(rand.Intn(5))
	}
	return bots
}

// BotToMap 机器人转Map（用于API响应）
func BotToMap(bot *model.Bot) map[string]interface{} {
	return map[string]interface{}{
		"id":            bot.ID,
		"name":          bot.Name,
		"display_name":  bot.DisplayName,
		"type":          bot.Type,
		"status":        bot.Status,
		"schedule":      bot.Schedule,
		"config":        bot.Config,
		"description":   bot.Description,
		"icon":          bot.Icon,
		"priority":      bot.Priority,
		"concurrency":   bot.Concurrency,
		"timeout":       bot.Timeout,
		"retry_count":   bot.RetryCount,
		"last_run":      bot.LastRun,
		"last_status":   bot.LastStatus,
		"last_error":    bot.LastError,
		"run_count":     bot.RunCount,
		"success_count": bot.SuccessCount,
		"fail_count":    bot.FailCount,
		"success_rate":  bot.SuccessRate,
		"cpu_usage":     bot.CPUUsage,
		"memory_usage":  bot.MemoryUsage,
		"network_io":    bot.NetworkIO,
		"enabled":       bot.Enabled,
		"created_at":    bot.CreatedAt,
		"updated_at":    bot.UpdatedAt,
	}
}

// UnmarshalConfig 解析机器人配置
func UnmarshalConfig(configStr string) map[string]interface{} {
	if configStr == "" {
		return make(map[string]interface{})
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &result); err != nil {
		return map[string]interface{}{"raw": configStr}
	}
	return result
}
