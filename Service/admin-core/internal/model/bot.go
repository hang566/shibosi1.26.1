package model

import "time"

// Bot 算法机器人模型
type Bot struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"size:128;not null;index"`
	DisplayName  string    `json:"display_name" gorm:"size:256"`
	Type         string    `json:"type" gorm:"size:32;index"` // crawler | analyzer | scheduler | notifier | security | ai_agent
	Status       string    `json:"status" gorm:"size:16;default:stopped;index"` // stopped | running | error | pending
	Schedule     string    `json:"schedule" gorm:"size:64"` // 调度策略
	Config       string    `json:"config" gorm:"type:text"` // JSON配置
	Description  string    `json:"description" gorm:"type:text"`
	Icon         string    `json:"icon" gorm:"size:64"`
	Priority     int       `json:"priority" gorm:"default:5"` // 优先级 1-10
	Concurrency  int       `json:"concurrency" gorm:"default:1"` // 并发数
	Timeout      int       `json:"timeout" gorm:"default:30"` // 超时秒数
	RetryCount   int       `json:"retry_count" gorm:"default:3"` // 重试次数
	LastRun      *time.Time `json:"last_run"`
	LastStatus   string    `json:"last_status" gorm:"size:16"`
	LastError    string    `json:"last_error" gorm:"type:text"`
	RunCount     int64     `json:"run_count" gorm:"default:0"`
	SuccessCount int64     `json:"success_count" gorm:"default:0"`
	FailCount    int64     `json:"fail_count" gorm:"default:0"`
	SuccessRate  float64   `json:"success_rate" gorm:"type:float"`
	CPUUsage     float64   `json:"cpu_usage" gorm:"type:float"`
	MemoryUsage  int64     `json:"memory_usage"`
	NetworkIO    int64     `json:"network_io"`
	Enabled      bool      `json:"enabled" gorm:"default:true;index"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Bot) TableName() string { return "bots" }

// BotLog 机器人运行日志
type BotLog struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	BotID     int64     `json:"bot_id" gorm:"index;not null"`
	BotName   string    `json:"bot_name" gorm:"size:128;index"`
	Level     string    `json:"level" gorm:"size:16;index"` // DEBUG | INFO | WARNING | ERROR | FATAL
	Message   string    `json:"message" gorm:"type:text"`
	Detail    string    `json:"detail" gorm:"type:text"`
	Duration  int64     `json:"duration"` // 毫秒
	Status    string    `json:"status" gorm:"size:16"` // success | failed | timeout
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (BotLog) TableName() string { return "bot_logs" }

// BotConfig 机器人配置模板
type BotConfig struct {
	ID          int64                  `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string                 `json:"key" gorm:"size:128;uniqueIndex;not null"`
	Value       string                 `json:"value" gorm:"type:text"`
	Type        string                 `json:"type" gorm:"size:32"` // string | number | boolean | json
	Description string                 `json:"description" gorm:"size:512"`
	Category    string                 `json:"category" gorm:"size:64;index"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

func (BotConfig) TableName() string { return "bot_configs" }

// BotType 机器人类型常量
const (
	BotTypeCrawler   = "crawler"   // 爬虫
	BotTypeAnalyzer  = "analyzer"  // 分析器
	BotTypeScheduler = "scheduler" // 调度器
	BotTypeNotifier  = "notifier"  // 通知器
	BotTypeSecurity  = "security"  // 安全机器人
	BotTypeAIAgent   = "ai_agent"  // AI代理
)

// BotStatus 机器人状态常量
const (
	BotStatusStopped = "stopped"
	BotStatusRunning = "running"
	BotStatusError   = "error"
	BotStatusPending = "pending"
)

// BotLogLevel 日志级别常量
const (
	BotLogDebug   = "DEBUG"
	BotLogInfo    = "INFO"
	BotLogWarning = "WARNING"
	BotLogError   = "ERROR"
	BotLogFatal   = "FATAL"
)
