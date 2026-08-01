package model

import "time"

// FirewallRule 防火墙规则
type FirewallRule struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Action    string    `json:"action" gorm:"size:16;not null;index"` // allow | deny
	Protocol  string    `json:"protocol" gorm:"size:16;not null"`     // tcp | udp | icmp | all
	Port      string    `json:"port" gorm:"size:32"`                  // 单端口或区间: 80 / 80-90 / 空表示不限
	Source    string    `json:"source" gorm:"size:64"`                // CIDR, 空=所有
	Comment   string    `json:"comment" gorm:"size:256"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
}

func (FirewallRule) TableName() string { return "firewall_rules" }

// SSHBlock SSH 防爆破黑名单
type SSHBlock struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	IP         string    `json:"ip" gorm:"uniqueIndex;size:64"`
	Failed    int       `json:"failed" gorm:"default:0"`
	LastTry   time.Time `json:"last_try"`
	BlockedAt time.Time `json:"blocked_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Reason    string    `json:"reason" gorm:"size:256"`
}

func (SSHBlock) TableName() string { return "ssh_blocks" }

// FileNode 文件/目录节点
type FileNode struct {
	Path     string    `json:"path" gorm:"primaryKey;size:1024"`
	IsDir    bool      `json:"is_dir"`
	Name     string    `json:"name" gorm:"size:256"`
	Size     int64     `json:"size"`
	Mode     string    `json:"mode" gorm:"size:16"`
	Modified time.Time `json:"modified"`
	MimeType string    `json:"mime_type" gorm:"size:128"`
	Children []*FileNode `json:"children,omitempty" gorm:"-"`
}

// Software 软件商店
type Software struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"uniqueIndex;size:64;not null"`
	DisplayName string    `json:"display_name" gorm:"size:128"`
	Category    string    `json:"category" gorm:"size:32;index"` // web | db | cache | proxy | monitor
	Version     string    `json:"version" gorm:"size:32"`
	Description string    `json:"description" gorm:"type:text"`
	Icon        string    `json:"icon" gorm:"size:64"`
	Installed   bool      `json:"installed" gorm:"default:false;index"`
	Status      string    `json:"status" gorm:"size:16;default:stopped"` // stopped | running | installed | error
	InstallPath string    `json:"install_path" gorm:"size:512"`
	ConfigPath  string    `json:"config_path" gorm:"size:512"`
	Port        int       `json:"port"`
	InstallLog  string    `json:"install_log" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Software) TableName() string { return "softwares" }

// Crontab 计划任务
type Crontab struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:128;not null"`
	Expression  string    `json:"expression" gorm:"size:64;not null"` // cron 表达式
	Description string    `json:"description" gorm:"size:256"`
	Type        string    `json:"type" gorm:"size:32;index"` // shell | backup-db | backup-site | api
	Command     string    `json:"command" gorm:"type:text"`
	Target      string    `json:"target" gorm:"size:256"` // 数据库/站点/URL
	StorageType string    `json:"storage_type" gorm:"size:16"` // local | oss | ftp
	StoragePath string    `json:"storage_path" gorm:"size:512"`
	Enabled     bool      `json:"enabled" gorm:"default:true;index"`
	LastRun     *time.Time `json:"last_run"`
	NextRun     *time.Time `json:"next_run"`
	RunCount    int       `json:"run_count" gorm:"default:0"`
	LastStatus  string    `json:"last_status" gorm:"size:16"` // success | failed
	LastError   string    `json:"last_error" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Crontab) TableName() string { return "crontabs" }

// CrontabLog 任务执行日志
type CrontabLog struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID    int64     `json:"task_id" gorm:"index;not null"`
	Status    string    `json:"status" gorm:"size:16"`
	Output    string    `json:"output" gorm:"type:text"`
	Error     string    `json:"error" gorm:"type:text"`
	Duration  int64     `json:"duration"` // 毫秒
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

func (CrontabLog) TableName() string { return "crontab_logs" }

// TerminalSession 终端会话
type TerminalSession struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	UserID    int64     `json:"user_id" gorm:"index"`
	User      string    `json:"user" gorm:"size:64"`
	Type      string    `json:"type" gorm:"size:16"` // shell | ssh
	Host      string    `json:"host" gorm:"size:128"`
	Port      int       `json:"port"`
	Status    string    `json:"status" gorm:"size:16;default:running"`
	CreatedAt time.Time `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
}

func (TerminalSession) TableName() string { return "terminal_sessions" }

// LogFile 日志文件元信息
type LogFile struct {
	Path      string    `json:"path" gorm:"primaryKey;size:512"`
	Name      string    `json:"name" gorm:"size:256"`
	Size      int64     `json:"size"`
	Lines     int64     `json:"lines"`
	Modified  time.Time `json:"modified"`
	Level     string    `json:"level" gorm:"size:16"` // info | warn | error
}

// TaskProgress 通用任务进度（WebSocket 推送用）
type TaskProgress struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Percent   int    `json:"percent"`
	Status    string `json:"status"` // pending | running | success | failed
	Message   string `json:"message"`
	Logs      []string `json:"logs"`
}
