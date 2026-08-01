// Package runtime 提供系统运行时信息采集
// 为了兼容无 gopsutil 环境，这里实现纯标准库版本
package runtime

import (
	"os"
	"runtime"
	"sync"
	"time"
)

// Stat 系统实时指标快照
type Stat struct {
	Time     int64      `json:"time"`
	CPU      float64    `json:"cpu"`
	CPUCores int        `json:"cpu_cores"`
	Mem      MemStat    `json:"mem"`
	Disks    []DiskStat `json:"disks"`
	Net      NetStat    `json:"net"`
	Host     HostStat   `json:"host"`
	Procs    int        `json:"procs"`
	Load     [3]float64 `json:"load"`
}

type MemStat struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Percent float64 `json:"percent"`
}

type DiskStat struct {
	Mount   string  `json:"mount"`
	Fstype  string  `json:"fstype"`
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Percent float64 `json:"percent"`
}

type NetStat struct {
	BytesSent  uint64   `json:"bytes_sent"`
	BytesRecv  uint64   `json:"bytes_recv"`
	Interfaces []string `json:"interfaces"`
}

type HostStat struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Platform string `json:"platform"`
	Uptime   uint64 `json:"uptime"`
	BootTime int64  `json:"boot_time"`
}

// Monitor 系统指标监控器
type Monitor struct {
	mu      sync.RWMutex
	history []Stat
	maxHist int
	done    chan struct{}
}

// NewMonitor 创建监控器
func NewMonitor() *Monitor {
	return &Monitor{history: make([]Stat, 0, 120), maxHist: 120, done: make(chan struct{})}
}

// Collect 采集一次系统状态
func Collect() (*Stat, error) {
	stat := &Stat{
		Time:     time.Now().UnixMilli(),
		CPUCores: runtime.NumCPU(),
		Host:     HostStat{OS: runtime.GOOS, Platform: runtime.GOARCH},
	}

	// 内存（使用 runtime.MemStats）
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stat.Mem = MemStat{
		Total:   m.Sys,
		Used:    m.Alloc,
		Percent: float64(m.Alloc) / float64(m.Sys+1) * 100,
	}

	// 磁盘（读取 /proc/mounts + syscall.Statfs）
	stat.Disks = readDisks()

	// Hostname
	if h, err := os.Hostname(); err == nil {
		stat.Host.Hostname = h
	}

	// Uptime from /proc/uptime (Linux)
	stat.Host.Uptime = readUptime()

	// Net (简单统计 /proc/net/dev)
	stat.Net = readNet()

	return stat, nil
}

func (m *Monitor) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if stat, err := Collect(); err == nil {
					m.mu.Lock()
					m.history = append(m.history, *stat)
					if len(m.history) > m.maxHist {
						m.history = m.history[len(m.history)-m.maxHist:]
					}
					m.mu.Unlock()
				}
			case <-m.done:
				return
			}
		}
	}()
}

func (m *Monitor) Stop() {
	select {
	case <-m.done:
	default:
		close(m.done)
	}
}

// Snapshot 获取最近 N 条历史
func (m *Monitor) Snapshot(n int) []Stat {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n <= 0 || n > len(m.history) {
		n = len(m.history)
	}
	out := make([]Stat, n)
	copy(out, m.history[len(m.history)-n:])
	return out
}

// Latest 获取最新一条
func (m *Monitor) Latest() *Stat {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.history) == 0 {
		return nil
	}
	s := m.history[len(m.history)-1]
	return &s
}
