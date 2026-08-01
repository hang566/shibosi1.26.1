// Package service 业务逻辑服务层
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"admin-core/internal/dao"
	"admin-core/internal/filemgr"
	"admin-core/internal/middleware"
	"admin-core/internal/model"
	"admin-core/internal/runtime"
	"admin-core/internal/terminal"
	"admin-core/internal/ws"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// FirewallService 防火墙服务（调用系统 iptables/firewalld）
type FirewallService struct {
	db   *gorm.DB
	da   *dao.DAO
	bf   *middleware.SSHBruteForce
	path string // 系统防火墙命令路径
}

func NewFirewallService(db *gorm.DB, da *dao.DAO, bf *middleware.SSHBruteForce) *FirewallService {
	return &FirewallService{db: db, da: da, bf: bf, path: detectFirewallCmd()}
}

func detectFirewallCmd() string {
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		return "firewalld"
	}
	if _, err := exec.LookPath("iptables"); err == nil {
		return "iptables"
	}
	return ""
}

// ListRules 列出防火墙规则
func (s *FirewallService) ListRules() ([]model.FirewallRule, error) {
	return s.da.ListFirewallRules()
}

// CreateRule 创建规则
func (s *FirewallService) CreateRule(rule *model.FirewallRule) error {
	if err := s.da.CreateFirewallRule(rule); err != nil {
		return err
	}
	return s.applyRule(rule)
}

// DeleteRule 删除规则
func (s *FirewallService) DeleteRule(id int64) error {
	rule, err := s.da.GetFirewallRule(id)
	if err != nil {
		return err
	}
	if err := s.removeRule(rule); err != nil {
		return err
	}
	return s.da.DeleteFirewallRule(id)
}

// ToggleRule 启用/禁用
func (s *FirewallService) ToggleRule(id int64, enable bool) error {
	rule, err := s.da.GetFirewallRule(id)
	if err != nil {
		return err
	}
	if enable {
		if err := s.applyRule(rule); err != nil {
			return err
		}
	} else {
		if err := s.removeRule(rule); err != nil {
			return err
		}
	}
	rule.Enabled = enable
	return s.da.UpdateFirewallRule(rule)
}

func (s *FirewallService) applyRule(rule *model.FirewallRule) error {
	if s.path == "" {
		// 无系统防火墙，仅在数据库中标记
		return nil
	}
	var cmd *exec.Cmd
	switch s.path {
	case "firewalld":
		args := []string{"--permanent", "--add-rich-rule",
			fmt.Sprintf("rule family='ipv4' source address='%s' port protocol='%s' port='%s' accept",
				rule.Source, rule.Protocol, rule.Port)}
		if rule.Action == "deny" {
			args[len(args)-1] = "reject"
		}
		cmd = exec.Command("firewall-cmd", args...)
	default:
		action := "A"
		if rule.Action == "deny" {
			action = "I"
		}
		args := []string{"-" + action, "INPUT", "-p", rule.Protocol, "--dport", rule.Port, "-j", rule.Action}
		if rule.Source != "" {
			args = append(args, "-s", rule.Source)
		}
		cmd = exec.Command("iptables", args...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[firewall] apply error: %s, out: %s", err, string(out))
		return err
	}
	return nil
}

func (s *FirewallService) removeRule(rule *model.FirewallRule) error {
	if s.path == "" {
		return nil
	}
	var cmd *exec.Cmd
	switch s.path {
	case "firewalld":
		args := []string{"--permanent", "--remove-rich-rule",
			fmt.Sprintf("rule family='ipv4' source address='%s' port protocol='%s' port='%s' accept",
				rule.Source, rule.Protocol, rule.Port)}
		cmd = exec.Command("firewall-cmd", args...)
	default:
		action := "D"
		args := []string{"-" + action, "INPUT", "-p", rule.Protocol, "--dport", rule.Port, "-j", rule.Action}
		if rule.Source != "" {
			args = append(args, "-s", rule.Source)
		}
		cmd = exec.Command("iptables", args...)
	}
	cmd.Run()
	return nil
}

// ListBlocked 查看 SSH 黑名单
func (s *FirewallService) ListBlocked() []middleware.BlockedIP {
	if s.bf == nil {
		return nil
	}
	return s.bf.GetBlockedList()
}

// UnblockIP 解封
func (s *FirewallService) UnblockIP(ip string) bool {
	if s.bf == nil {
		return false
	}
	return s.bf.Unblock(ip)
}

// CrontabService 计划任务服务
type CrontabService struct {
	db     *gorm.DB
	da     *dao.DAO
	cron   *cron.Cron
	wsHub  *ws.Hub
	runner *TaskRunner
}

// TaskRunner 任务执行器
type TaskRunner struct {
	wsHub *ws.Hub
}

func NewTaskRunner(hub *ws.Hub) *TaskRunner {
	return &TaskRunner{wsHub: hub}
}

// Run 执行一个 Crontab 任务
func (r *TaskRunner) Run(task *model.Crontab) (*model.CrontabLog, error) {
	started := time.Now()
	log := &model.CrontabLog{
		TaskID: task.ID, Status: "running", StartedAt: started,
	}
	if r.wsHub != nil {
		r.wsHub.Publish(fmt.Sprintf("task:%d", task.ID), "start", nil)
	}

	var cmd *exec.Cmd
	switch task.Type {
	case "shell":
		cmd = exec.Command("bash", "-c", task.Command)
	case "backup-db":
		cmd = exec.Command("mysqldump", "-u", "root", "-p", task.Command, task.Target)
	case "api":
		cmd = exec.Command("curl", "-fsSL", task.Command)
	default:
		cmd = exec.Command("bash", "-c", task.Command)
	}
	out, err := cmd.CombinedOutput()
	ended := time.Now()
	log.Duration = ended.Sub(started).Milliseconds()
	log.EndedAt = ended
	if err != nil {
		log.Status = "failed"
		log.Error = err.Error()
	} else {
		log.Status = "success"
		log.Output = string(out)
	}
	if r.wsHub != nil {
		r.wsHub.Publish(fmt.Sprintf("task:%d", task.ID), "end", map[string]interface{}{
			"status": log.Status,
			"log":    log.Output,
			"error":  log.Error,
		})
	}
	return log, nil
}

func NewCrontabService(db *gorm.DB, da *dao.DAO, hub *ws.Hub) *CrontabService {
	s := &CrontabService{
		db: da.DB, da: da, cron: cron.New(cron.WithSeconds()), wsHub: hub,
		runner: NewTaskRunner(hub),
	}
	s.cron.Start()
	s.reloadTasks()
	return s
}

func (s *CrontabService) reloadTasks() {
	tasks, err := s.da.ListCrontabs()
	if err != nil {
		return
	}
	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		task := t
		_, err := s.cron.AddFunc(task.Expression, func() {
			log, _ := s.runner.Run(&task)
			s.da.CreateCrontabLog(log)
			task.LastRun = &log.StartedAt
			task.LastStatus = log.Status
			task.RunCount++
			nextTime, _ := s.nextTime(task.Expression)
			if nextTime != nil {
				task.NextRun = nextTime
			}
			s.da.UpdateCrontab(&task)
		})
		if err != nil {
			log.Printf("[crontab] add func failed: %v expr=%s", err, task.Expression)
		}
	}
}

// List / Create / Update / Delete
func (s *CrontabService) List() ([]model.Crontab, error) { return s.da.ListCrontabs() }
func (s *CrontabService) Create(t *model.Crontab) error {
	err := s.da.CreateCrontab(t)
	if err == nil {
		s.reloadTasks()
	}
	return err
}
func (s *CrontabService) Update(t *model.Crontab) error {
	err := s.da.UpdateCrontab(t)
	if err == nil {
		s.reloadTasks()
	}
	return err
}
func (s *CrontabService) Delete(id int64) error { return s.da.DeleteCrontab(id) }
func (s *CrontabService) ListLogs(taskID int64) ([]model.CrontabLog, error) {
	return s.da.ListCrontabLogs(taskID)
}

// Trigger 手动触发
func (s *CrontabService) Trigger(id int64) error {
	task, err := s.da.GetCrontab(id)
	if err != nil {
		return err
	}
	go func() {
		log, _ := s.runner.Run(task)
		s.da.CreateCrontabLog(log)
	}()
	return nil
}

func (s *CrontabService) nextTime(expr string) (*time.Time, error) {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(expr)
	if err != nil {
		return nil, err
	}
	t := sched.Next(time.Now())
	return &t, nil
}

// ToChinese 将 cron 表达式翻译为中文
func ToChinese(expr string) string {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(expr)
	if err != nil {
		return "无效表达式"
	}
	// 简化版：识别常见模式
	switch expr {
	case "0 0 3 * * mon":
		return "每周一 凌晨 3:00"
	case "0 0 2 * * *":
		return "每天 凌晨 2:00"
	case "0 0 0 * * 0":
		return "每周日 凌晨 0:00"
	case "0 0 0 1 * *":
		return "每月 1 日 凌晨 0:00"
	}
	next := sched.Next(time.Now())
	return fmt.Sprintf("下次执行: %s", next.Format("2006-01-02 15:04"))
}

// FileService 文件管理服务
type FileService struct {
	fm *filemgr.Manager
	da *dao.DAO
}

func NewFileService(da *dao.DAO, baseDir string) *FileService {
	return &FileService{fm: filemgr.New(baseDir), da: da}
}

func (s *FileService) List(path string, depth int) ([]*filemgr.Node, error) {
	return s.fm.List(path, depth)
}
func (s *FileService) Read(path string) (string, error)         { return s.fm.Read(path) }
func (s *FileService) Write(path, content string) error         { return s.fm.Write(path, content) }
func (s *FileService) CreateDir(path string) error              { return s.fm.CreateDir(path) }
func (s *FileService) Rename(o, n string) error                 { return s.fm.Rename(o, n) }
func (s *FileService) Move(o, n string) error                   { return s.fm.Move(o, n) }
func (s *FileService) Copy(o, n string) error                   { return s.fm.Copy(o, n) }
func (s *FileService) Delete(path string) error                 { return s.fm.Delete(path) }
func (s *FileService) Chmod(path, mode string) error            { return s.fm.Chmod(path, mode) }
func (s *FileService) Zip(src, dest string) (string, error)     { return s.fm.Zip(src, dest) }
func (s *FileService) TarGz(src, dest string) (string, error)   { return s.fm.TarGz(src, dest) }
func (s *FileService) Extract(archive, dest string) error       { return s.fm.Extract(archive, dest) }
func (s *FileService) Search(k string) ([]*filemgr.Node, error) { return s.fm.Search(k) }
func (s *FileService) BaseDir() string                          { return s.fm.BaseDir }

// LogService 日志服务（实时 tail -f）
type LogService struct {
	logDir string
}

func NewLogService(logDir string) *LogService {
	return &LogService{logDir: logDir}
}

// ListLogs 列出日志文件
func (ls *LogService) ListLogs() ([]model.LogFile, error) {
	entries, err := os.ReadDir(ls.logDir)
	if err != nil {
		return nil, err
	}
	var list []model.LogFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 4 || name[len(name)-4:] != ".log" {
			continue
		}
		fi, _ := e.Info()
		list = append(list, model.LogFile{
			Path:     ls.logDir + "/" + name,
			Name:     name,
			Size:     fi.Size(),
			Modified: fi.ModTime(),
		})
	}
	return list, nil
}

// Tail 启动实时追踪，通过 wsHub 推送
func (ls *LogService) Tail(filename string, lines int, hub *ws.Hub) error {
	path := ls.logDir + "/" + filename
	ctx := context.Background()
	go func() {
		cmd := exec.CommandContext(ctx, "tail", "-f", "-n", fmt.Sprintf("%d", lines), path)
		out, _ := cmd.StdoutPipe()
		cmd.Start()
		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			default:
			}
			n, _ := out.Read(buf)
			if n > 0 {
				hub.Publish("log:"+filename, "data", string(buf[:n]))
			}
		}
	}()
	return nil
}

// TerminalService 终端服务
type TerminalService struct {
	mgr *terminal.Manager
	da  *dao.DAO
}

func NewTerminalService(da *dao.DAO) *TerminalService {
	return &TerminalService{mgr: terminal.NewManager(), da: da}
}

func (s *TerminalService) CreateShell(userID int64, user string, wsConn interface{}) (string, error) {
	// 简化：仅返回会话，实际创建在 handler 中完成
	_ = wsConn
	_ = terminal.Session{}
	return "", fmt.Errorf("use TerminalManager directly via handler")
}

// RuntimeService 系统运行时服务
type RuntimeService struct {
	mon  *runtime.Monitor
	hub  *ws.Hub
	once sync.Once
}

func NewRuntimeService(hub *ws.Hub) *RuntimeService {
	return &RuntimeService{mon: runtime.NewMonitor(), hub: hub}
}

func (s *RuntimeService) Start() {
	s.once.Do(func() {
		s.mon.Start(2 * time.Second)
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for range t.C {
				if stat := s.mon.Latest(); stat != nil {
					s.hub.Publish("system:stat", "update", stat)
				}
			}
		}()
	})
}

func (s *RuntimeService) Snapshot(n int) []runtime.Stat {
	if n <= 0 {
		n = 60
	}
	return s.mon.Snapshot(n)
}

func (s *RuntimeService) Latest() *runtime.Stat { return s.mon.Latest() }

// LogTaskStart / LogTaskEnd helpers (未用)
var _ = json.Marshal
