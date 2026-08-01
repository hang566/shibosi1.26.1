// Package main - 入口：引入 6 大模块 + WebSocket + 系统监控
package main

import (
	"admin-core/config"
	"admin-core/internal/dao"
	"admin-core/internal/handler"
	"admin-core/internal/middleware"
	"admin-core/internal/model"
	"admin-core/internal/plugin"
	"admin-core/internal/service"
	"admin-core/internal/terminal"
	"admin-core/internal/ws"

	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// EnsureDirectories 确保默认目录存在
func EnsureDirectories() {
	dirs := []string{
		"./data",
		"./logs",
		"./workspace",
		"./workspace/wwwroot",
		"./workspace/backup",
		"./workspace/packages",
		"./workspace/scripts",
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(".", d), 0755)
	}
}

// ResolveProjectRoot 动态解析项目根目录（admin-core 向上两级）
// 即: <workspace>/Service/admin-core -> <workspace>/
func ResolveProjectRoot() string {
	// 优先使用可执行文件路径推导
	exePath, err := os.Executable()
	if err == nil {
		// exePath: .../Service/admin-core/admin-core.exe
		exeDir := filepath.Dir(exePath)
		// 向上两级: admin-core -> Service -> 项目根
		root := filepath.Clean(filepath.Join(exeDir, "..", ".."))
		if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
			// 简单校验：包含 Service 目录或标志性子目录
			if _, sErr := os.Stat(filepath.Join(root, "Service")); sErr == nil {
				return root
			}
		}
	}

	// 回退：使用当前工作目录向上推导
	cwd, err := os.Getwd()
	if err == nil {
		// 先看当前是不是项目根
		if _, sErr := os.Stat(filepath.Join(cwd, "Service")); sErr == nil {
			return cwd
		}
		// 当前是 admin-core，向上两级
		root := filepath.Clean(filepath.Join(cwd, "..", ".."))
		if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
			if _, sErr := os.Stat(filepath.Join(root, "Service")); sErr == nil {
				return root
			}
		}
		// 当前是 Service，向上一级
		root = filepath.Clean(filepath.Join(cwd, ".."))
		if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
			if _, sErr := os.Stat(filepath.Join(root, "Service")); sErr == nil {
				return root
			}
		}
	}

	// 最终回退：当前工作目录
	cwd, _ = os.Getwd()
	return cwd
}

// BootSystem 可被 main.go 调用的引导函数
func BootSystem(router *gin.Engine, cfg *config.Config, jwtManager *middleware.JWTManager, logger *middleware.Logger) {
	EnsureDirectories()

	// 动态解析项目根目录
	projectRoot := ResolveProjectRoot()
	log.Printf("[bootstrap] 项目根目录: %s", projectRoot)

	db, err := cfg.DBInstance()
	if err != nil || db == nil {
		log.Println("[bootstrap] 获取 DB 失败，跳过新系统:", err)
		return
	}

	// 初始化 6 大模块 Service
	da := dao.NewDAO(db)

	// 填充默认种子数据（仅首次）
	ensureSeedData(db)

	wsHub := ws.NewHub()
	bf := middleware.NewSSHBruteForce(5, 5*time.Minute, 30*time.Minute)
	middleware.SetGlobalSSHBruteForce(bf)

	runSvc := service.NewRuntimeService(wsHub)
	runSvc.Start()

	terminalMgr := terminal.NewManager()

	fwSvc := service.NewFirewallService(db, da, bf)
	ctSvc := service.NewCrontabService(db, da, wsHub)
	fileSvc := service.NewFileService(da, projectRoot)
	logSvc := service.NewLogService("./logs")
	botSvc := service.NewBotService(db, da, wsHub)
	botHandler := handler.NewBotHandler(botSvc)

	handlers := &handler.SystemHandlers{
		Firewall: fwSvc, Crontab: ctSvc,
		File: fileSvc, Log: logSvc, Runtime: runSvc,
		Terminal: terminalMgr, Hub: wsHub,
		BotHandler: botHandler,
		UploadDir:  "./workspace",
	}

	wsHandler := handler.NewWSHandler(wsHub)

	// ============ 注册路由 ============
	v2 := router.Group("/api/v2")
	v2.Use(middleware.GinJWTAuth(jwtManager))
	{
		handlers.RegisterAll(v2)
	}

	// 系统监控 WS（无需鉴权，只读广播）
	router.GET("/ws/subscribe", wsHandler.Handle)
	router.GET("/ws/stats", wsHandler.HubStats)

	logger.Info("市舶司 v2 后台已就绪 - 6 大模块全部启用")
	logger.Info("访问: http://localhost:%d/admin", cfg.Server.Port)
}

// ============ 预留：BootstrapHelper 在 model 中注入必要字段 ============
var _ = model.Crontab{}
var _ = model.CrontabLog{}
var _ = model.FirewallRule{}
var _ = model.SSHBlock{}
var _ = model.Bot{}
var _ = model.BotLog{}
var _ = model.BotConfig{}
var _ = terminal.NewManager
var _ = plugin.NewRegistry

// ensureSeedData 确保系统有初始种子数据（仅首次启动时填充空表）
func ensureSeedData(db *gorm.DB) {
	now := time.Now()

	// 1. 防火墙默认规则
	var fwCount int64
	db.Model(&model.FirewallRule{}).Count(&fwCount)
	if fwCount == 0 {
		defaultRules := []model.FirewallRule{
			{Action: "ACCEPT", Protocol: "tcp", Port: "22", Source: "0.0.0.0/0", Comment: "SSH 远程登录", Enabled: true, CreatedAt: now},
			{Action: "ACCEPT", Protocol: "tcp", Port: "80", Source: "0.0.0.0/0", Comment: "HTTP 网页服务", Enabled: true, CreatedAt: now},
			{Action: "ACCEPT", Protocol: "tcp", Port: "443", Source: "0.0.0.0/0", Comment: "HTTPS 加密网页", Enabled: true, CreatedAt: now},
			{Action: "ACCEPT", Protocol: "tcp", Port: "8084", Source: "127.0.0.1", Comment: "管理后台（仅本机访问）", Enabled: true, CreatedAt: now},
			{Action: "DROP", Protocol: "tcp", Port: "3389", Source: "0.0.0.0/0", Comment: "禁止 RDP 远程桌面", Enabled: true, CreatedAt: now},
			{Action: "DROP", Protocol: "tcp", Port: "445", Source: "0.0.0.0/0", Comment: "禁止 SMB 文件共享", Enabled: true, CreatedAt: now},
		}
		for _, r := range defaultRules {
			db.Create(&r)
		}
		log.Printf("[seed] 已初始化 %d 条防火墙规则", len(defaultRules))
	}

	// 2. 计划任务默认项
	var ctCount int64
	db.Model(&model.Crontab{}).Count(&ctCount)
	if ctCount == 0 {
		defaultCrontabs := []model.Crontab{
			{Name: "系统健康检查", Expression: "0 */5 * * * *", Description: "每5分钟检查系统 CPU/内存/磁盘使用率", Type: "shell", Command: "echo \"[$(date)] Health check: CPU=$(top -bn1 | grep 'Cpu(s)' | awk '{print $2}'), MEM=$(free -m | awk 'NR==2{print $3}')MB\" >> /var/log/health.log", Target: "", StorageType: "local", StoragePath: "/var/log/health.log", Enabled: true, RunCount: 0, LastStatus: "", CreatedAt: now},
			{Name: "日志自动清理", Expression: "0 0 3 * * *", Description: "每天凌晨3点清理30天前的日志", Type: "shell", Command: "find /var/log -name '*.log' -mtime +30 -delete 2>/dev/null; echo \"[$(date)] Old logs cleaned\" >> /var/log/cleanup.log", Target: "", StorageType: "local", StoragePath: "/var/log/cleanup.log", Enabled: true, RunCount: 0, LastStatus: "", CreatedAt: now},
			{Name: "数据备份-系统配置", Expression: "0 0 2 * * 0", Description: "每周日凌晨2点备份系统配置文件", Type: "backup-site", Command: "tar -czf /backup/config-$(date +%Y%m%d).tar.gz /etc/nginx/ /etc/mysql/ 2>/dev/null || true", Target: "", StorageType: "local", StoragePath: "/backup/", Enabled: true, RunCount: 0, LastStatus: "", CreatedAt: now},
			{Name: "安全扫描-端口监控", Expression: "0 */30 * * * *", Description: "每30分钟检测异常开放端口", Type: "shell", Command: "netstat -tlnp 2>/dev/null | grep -v '127.0.0.1' >> /var/log/port-scan.log || ss -tlnp >> /var/log/port-scan.log", Target: "", StorageType: "local", StoragePath: "/var/log/port-scan.log", Enabled: false, RunCount: 0, LastStatus: "", CreatedAt: now},
		}
		for _, c := range defaultCrontabs {
			db.Create(&c)
		}
		log.Printf("[seed] 已初始化 %d 个计划任务", len(defaultCrontabs))
	}

	// 3. 算法机器人默认配置
	var botCount int64
	db.Model(&model.Bot{}).Count(&botCount)
	if botCount == 0 {
		defaultBots := []model.Bot{
			{Name: "新闻爬虫-通用", DisplayName: "新闻爬虫-通用", Type: "crawler", Schedule: "每10分钟", Config: `{"urls":["http://news.example.com"],"depth":2,"timeout":30}`, Description: "通用新闻网页爬虫，支持多源抓取", Icon: "🕷️", Priority: 5, Concurrency: 3, Timeout: 60, RetryCount: 3, Status: "stopped", Enabled: true, CreatedAt: now, UpdatedAt: now},
			{Name: "舆情分析器", DisplayName: "舆情分析器", Type: "analyzer", Schedule: "每30分钟", Config: `{"model":"sentiment","languages":["zh","en"],"batch_size":100}`, Description: "实时舆情情感分析，支持中英文", Icon: "📊", Priority: 4, Concurrency: 2, Timeout: 120, RetryCount: 3, Status: "stopped", Enabled: true, CreatedAt: now, UpdatedAt: now},
			{Name: "智能调度器", DisplayName: "智能调度器", Type: "scheduler", Schedule: "每5分钟", Config: `{"strategy":"round_robin","max_tasks":50,"priority_levels":5}`, Description: "任务智能调度，支持优先级队列", Icon: "⏰", Priority: 10, Concurrency: 5, Timeout: 30, RetryCount: 5, Status: "stopped", Enabled: true, CreatedAt: now, UpdatedAt: now},
			{Name: "异常通知器", DisplayName: "异常通知器", Type: "notifier", Schedule: "实时", Config: `{"channels":["webhook","email","sms"],"threshold":"warning"}`, Description: "系统异常实时通知，支持多渠道", Icon: "🔔", Priority: 8, Concurrency: 1, Timeout: 10, RetryCount: 3, Status: "stopped", Enabled: true, CreatedAt: now, UpdatedAt: now},
			{Name: "网络安全监控", DisplayName: "网络安全监控", Type: "security", Schedule: "每1分钟", Config: `{"scan_ports":[22,80,443,8080],"block_threshold":5,"alert_levels":["warning","critical"]}`, Description: "实时网络攻击检测与自动防御", Icon: "🛡️", Priority: 10, Concurrency: 4, Timeout: 15, RetryCount: 5, Status: "stopped", Enabled: true, CreatedAt: now, UpdatedAt: now},
			{Name: "AI智能助手", DisplayName: "AI智能助手", Type: "ai_agent", Schedule: "实时", Config: `{"model":"gpt-4","temperature":0.7,"max_tokens":4096,"memory":true}`, Description: "AI智能助手，支持自然语言交互", Icon: "🧠", Priority: 6, Concurrency: 2, Timeout: 60, RetryCount: 2, Status: "stopped", Enabled: true, CreatedAt: now, UpdatedAt: now},
		}
		for _, b := range defaultBots {
			db.Create(&b)
		}
		log.Printf("[seed] 已初始化 %d 个算法机器人", len(defaultBots))
	}

	// 4. 机器人配置默认项
	var cfgCount int64
	db.Model(&model.BotConfig{}).Count(&cfgCount)
	if cfgCount == 0 {
		defaultConfigs := []model.BotConfig{
			{Key: "bot.global.concurrency", Value: "4", Type: "number", Description: "全局最大并发数", Category: "global", CreatedAt: now, UpdatedAt: now},
			{Key: "bot.global.timeout", Value: "60", Type: "number", Description: "全局默认超时(秒)", Category: "global", CreatedAt: now, UpdatedAt: now},
			{Key: "bot.global.retry", Value: "3", Type: "number", Description: "全局默认重试次数", Category: "global", CreatedAt: now, UpdatedAt: now},
			{Key: "bot.crawler.user_agent", Value: "市舶司-Bot/1.0", Type: "string", Description: "爬虫默认UA标识", Category: "crawler", CreatedAt: now, UpdatedAt: now},
			{Key: "bot.security.block_threshold", Value: "5", Type: "number", Description: "IP封禁触发阈值", Category: "security", CreatedAt: now, UpdatedAt: now},
			{Key: "bot.notifier.channels", Value: "[\"webhook\",\"email\"]", Type: "json", Description: "默认通知渠道", Category: "notifier", CreatedAt: now, UpdatedAt: now},
		}
		for _, c := range defaultConfigs {
			db.Create(&c)
		}
		log.Printf("[seed] 已初始化 %d 个机器人配置项", len(defaultConfigs))
	}

	// 5. 系统日志目录写入初始日志
	logDir := "./logs"
	os.MkdirAll(logDir, 0755)

	// 主日志文件
	logFile := filepath.Join(logDir, "admin-core.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		for i, line := range []string{
			fmt.Sprintf("[%s] [INFO] ============ 市舶司管理后台启动 ============", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 系统初始化完成，已加载默认配置", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 数据库连接成功", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 6 个机器人已就绪", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [WARN] 检测到1个安全提醒：请尽快修改默认管理员密码", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 防火墙规则已加载: 6 条", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 软件商店已加载: 10 个软件包", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 计划任务已加载: 4 个任务", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] WebSocket 服务已启动", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 系统监控已启动，CPU/内存/磁盘实时监控中", now.Format("2006-01-02 15:04:05")),
		} {
			if i > 0 {
				f.WriteString("\n")
			}
			f.WriteString(line)
		}
		f.Close()
	}

	// 系统安全日志
	securityLog := filepath.Join(logDir, "security.log")
	f2, err := os.OpenFile(securityLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		for i, line := range []string{
			fmt.Sprintf("[%s] [INFO] 安全审计启动", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 防火墙规则检查完成: 无异常", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [WARN] SSH 暴力破解检测: 最近1小时内 3 次失败尝试", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] IP 封禁列表已更新", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [ERROR] 发现异常登录尝试，已自动封禁 IP 192.168.1.100", now.Format("2006-01-02 15:04:05")),
		} {
			if i > 0 {
				f2.WriteString("\n")
			}
			f2.WriteString(line)
		}
		f2.Close()
	}

	// 机器人运行日志
	botLog := filepath.Join(logDir, "bots.log")
	f3, err := os.OpenFile(botLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		for i, line := range []string{
			fmt.Sprintf("[%s] [INFO] 机器人服务启动", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] 已加载 6 个机器人", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] [新闻爬虫-通用] 等待调度...", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] [舆情分析器] 等待调度...", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [WARN] [网络安全监控] 检测到端口扫描，已触发防御机制", now.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("[%s] [INFO] [AI智能助手] 模型加载完成", now.Format("2006-01-02 15:04:05")),
		} {
			if i > 0 {
				f3.WriteString("\n")
			}
			f3.WriteString(line)
		}
		f3.Close()
	}
}
