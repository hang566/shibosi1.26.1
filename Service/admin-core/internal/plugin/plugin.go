// Package plugin 软件商店插件化架构
package plugin

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Installer 安装器接口
type Installer interface {
	Name() string
	Info() SoftwareInfo
	Install(ctx context.Context, logger LogWriter) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() (string, error) // running | stopped | installed
}

// LogWriter 日志流接口
type LogWriter interface {
	Write(line string)
}

// SoftwareInfo 软件元信息
type SoftwareInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Port        int    `json:"port"`
	InstallPath string `json:"install_path"`
	ConfigPath  string `json:"config_path"`
}

// Registry 软件注册表
type Registry struct {
	mu   sync.RWMutex
	list map[string]Installer
}

func NewRegistry() *Registry {
	return &Registry{list: make(map[string]Installer)}
}

func (r *Registry) Register(ins Installer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list[ins.Name()] = ins
}

func (r *Registry) Get(name string) (Installer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ins, ok := r.list[name]
	return ins, ok
}

func (r *Registry) All() []SoftwareInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]SoftwareInfo, 0, len(r.list))
	for _, v := range r.list {
		list = append(list, v.Info())
	}
	return list
}

// ShellInstaller 通用 Shell 脚本安装器
type ShellInstaller struct {
	info   SoftwareInfo
	Script string // shell 脚本路径或内联脚本
	Env    []string
	cmd    *exec.Cmd
	mu     sync.Mutex
}

func NewShellInstaller(info SoftwareInfo, script string) *ShellInstaller {
	return &ShellInstaller{info: info, Script: script}
}

func (s *ShellInstaller) Name() string       { return s.info.Name }
func (s *ShellInstaller) Info() SoftwareInfo { return s.info }

func (s *ShellInstaller) Install(ctx context.Context, logger LogWriter) error {
	var cmd *exec.Cmd
	if strings.HasSuffix(s.Script, ".sh") || strings.HasSuffix(s.Script, ".bat") {
		if _, err := os.Stat(s.Script); err != nil {
			return fmt.Errorf("script not found: %w", err)
		}
		if strings.HasSuffix(s.Script, ".bat") {
			cmd = exec.CommandContext(ctx, "cmd", "/c", s.Script)
		} else {
			cmd = exec.CommandContext(ctx, "bash", "-c", "chmod +x "+s.Script+" && "+s.Script)
		}
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", s.Script)
	}
	cmd.Env = append(os.Environ(), s.Env...)
	cmd.Dir = s.info.InstallPath
	if err := os.MkdirAll(cmd.Dir, 0755); err != nil {
		return err
	}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	s.cmd = cmd
	s.mu.Unlock()

	// 实时日志输出
	go pipeLogs(stdout, logger, "stdout")
	go pipeLogs(stderr, logger, "stderr")

	err := cmd.Wait()
	s.mu.Lock()
	s.cmd = nil
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("install failed: %w", err)
	}
	logger.Write(fmt.Sprintf("[%s] 安装完成", s.info.DisplayName))
	return nil
}

func (s *ShellInstaller) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil {
		return fmt.Errorf("already running")
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", s.Script+" start")
	cmd.Dir = s.info.InstallPath
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	go func() {
		cmd.Wait()
		s.mu.Lock()
		s.cmd = nil
		s.mu.Unlock()
	}()
	return nil
}

func (s *ShellInstaller) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *ShellInstaller) Status() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return "running", nil
	}
	return "stopped", nil
}

func pipeLogs(r io.ReadCloser, logger LogWriter, tag string) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			line := strings.TrimSpace(string(buf[:n]))
			if line != "" {
				logger.Write(fmt.Sprintf("[%s] %s", tag, line))
			}
		}
		if err != nil {
			break
		}
	}
}

// StreamLogger 流式日志写入器（收集到字符串切片 + 回调）
type StreamLogger struct {
	mu     sync.Mutex
	Lines  []string
	OnLine func(string)
	Done   chan struct{}
}

func NewStreamLogger() *StreamLogger {
	return &StreamLogger{Lines: make([]string, 0), Done: make(chan struct{})}
}

func (l *StreamLogger) Write(line string) {
	l.mu.Lock()
	l.Lines = append(l.Lines, fmt.Sprintf("%s %s", time.Now().Format("15:04:05"), line))
	if l.OnLine != nil {
		l.OnLine(line)
	}
	l.mu.Unlock()
}

func (l *StreamLogger) Snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.Lines))
	copy(out, l.Lines)
	return out
}

func (l *StreamLogger) Close() {
	select {
	case <-l.Done:
	default:
		close(l.Done)
	}
}

// NginxInstaller 预置 Nginx 安装器示例
func NewNginxInstaller(installPath string) *ShellInstaller {
	script := fmt.Sprintf(`
set -e
if command -v apt >/dev/null 2>&1; then
  apt-get update && apt-get install -y nginx
elif command -v yum >/dev/null 2>&1; then
  yum install -y nginx
elif command -v brew >/dev/null 2>&1; then
  brew install nginx
else
  echo "不支持的包管理器"
  exit 1
fi
nginx -v
`)
	return NewShellInstaller(SoftwareInfo{
		Name: "nginx", DisplayName: "Nginx", Category: "proxy", Version: "latest",
		Description: "高性能 HTTP 与反向代理服务器", Icon: "🌐",
		Port: 80, InstallPath: installPath,
		ConfigPath: "/etc/nginx/nginx.conf",
	}, script)
}

// MySQLInstaller 预置 MySQL/MariaDB 安装器
func NewMySQLInstaller(installPath string) *ShellInstaller {
	script := `
set -e
if command -v apt >/dev/null 2>&1; then
  apt-get update && apt-get install -y mariadb-server
elif command -v yum >/dev/null 2>&1; then
  yum install -y mariadb-server
else
  echo "不支持的包管理器"
  exit 1
fi
mysql --version || mariadb --version
`
	return NewShellInstaller(SoftwareInfo{
		Name: "mysql", DisplayName: "MySQL/MariaDB", Category: "db", Version: "latest",
		Description: "关系型数据库服务", Icon: "🐬", Port: 3306,
		InstallPath: installPath, ConfigPath: "/etc/mysql/my.cnf",
	}, script)
}

// RedisInstaller 预置 Redis 安装器
func NewRedisInstaller(installPath string) *ShellInstaller {
	script := `
set -e
if command -v apt >/dev/null 2>&1; then
  apt-get update && apt-get install -y redis-server
elif command -v yum >/dev/null 2>&1; then
  yum install -y redis
elif command -v brew >/dev/null 2>&1; then
  brew install redis
else
  echo "不支持的包管理器"
  exit 1
fi
redis-server --version
`
	return NewShellInstaller(SoftwareInfo{
		Name: "redis", DisplayName: "Redis", Category: "cache", Version: "latest",
		Description: "高性能键值缓存与消息队列", Icon: "📦", Port: 6379,
		InstallPath: installPath, ConfigPath: "/etc/redis/redis.conf",
	}, script)
}

// PM2Installer 预置 Node.js 进程管理器
func NewPM2Installer(installPath string) *ShellInstaller {
	script := `
set -e
if ! command -v node >/dev/null 2>&1; then
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt-get install -y nodejs
fi
npm install -g pm2
pm2 --version
`
	return NewShellInstaller(SoftwareInfo{
		Name: "pm2", DisplayName: "PM2", Category: "proxy", Version: "latest",
		Description: "Node.js 进程管理器", Icon: "⚙️", Port: 0,
		InstallPath: installPath, ConfigPath: "~/.pm2",
	}, script)
}

// LoggerToLog 默认日志（输出到 log）
type LoggerToLog struct{}

func (LoggerToLog) Write(line string) { log.Println(line) }
