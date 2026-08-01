// Package terminal 终端会话管理（WebSocket Shell 服务端）
package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Manager 终端会话管理器
type Manager struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	defaultShell string
}

// Session 单个终端会话
type Session struct {
	ID           string
	UserID       int64
	User         string
	Type         string
	Host         string
	Port         int
	WS           *websocket.Conn
	PTY          PTY
	Status       string
	Cancel       context.CancelFunc
	StartedAt    time.Time
	lastActivity time.Time
	mu           sync.Mutex
}

// PTY 伪终端接口
type PTY interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
	Close() error
	SetSize(cols, rows uint16) error
}

// LocalPTY 本地伪终端实现
type LocalPTY struct {
	Shell   *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stdinR  io.ReadCloser
	stdoutW io.WriteCloser
	mu      sync.Mutex
}

func NewLocalPTY(shell string) (*LocalPTY, error) {
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "LANG=C.UTF-8")
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	cmd.Stderr = stdoutW
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &LocalPTY{
		Shell: cmd, stdin: stdinW, stdout: stdoutR,
		stdinR: stdinR, stdoutW: stdoutW,
	}, nil
}

func (p *LocalPTY) Read(buf []byte) (int, error) { return p.stdout.Read(buf) }
func (p *LocalPTY) Write(buf []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stdin.Write(buf)
}
func (p *LocalPTY) Close() error {
	p.Shell.Process.Kill()
	p.stdinR.Close()
	p.stdoutW.Close()
	return nil
}
func (p *LocalPTY) SetSize(cols, rows uint16) error { return nil }

func defaultShellPath() string {
	if p, err := exec.LookPath("bash"); err == nil && p != "" {
		return p
	}
	if p, err := exec.LookPath("sh"); err == nil && p != "" {
		return p
	}
	return "/bin/sh"
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session), defaultShell: defaultShellPath()}
}

// CreateShellSession 创建本地 Shell 会话
func (m *Manager) CreateShellSession(userID int64, user string, wsConn *websocket.Conn) (*Session, error) {
	pty, err := NewLocalPTY(m.defaultShell)
	if err != nil {
		return nil, err
	}
	_, cancel := context.WithCancel(context.Background())
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	s := &Session{
		ID: id, UserID: userID, User: user,
		Type: "shell", Host: "localhost",
		WS: wsConn, PTY: pty, Status: "running",
		Cancel: cancel, StartedAt: time.Now(), lastActivity: time.Now(),
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()

	// 输出 -> WS
	go func() {
		defer m.CloseSession(id)
		buf := make([]byte, 4096)
		for {
			n, err := pty.Read(buf)
			if n > 0 {
				data, _ := json.Marshal(map[string]interface{}{"type": "output", "data": string(buf[:n])})
				if err2 := wsConn.WriteMessage(websocket.TextMessage, data); err2 != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// 客户端消息 -> PTY
	go func() {
		for {
			msgType, data, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.TextMessage {
				handleClientMessage(s, data)
			}
		}
	}()

	// 空闲超时
	go func() {
		t := time.NewTicker(1 * time.Minute)
		defer t.Stop()
		for range t.C {
			s.mu.Lock()
			last := s.lastActivity
			s.mu.Unlock()
			if time.Since(last) > 15*time.Minute {
				m.CloseSession(id)
				return
			}
		}
	}()

	return s, nil
}

func handleClientMessage(s *Session, data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		s.PTY.Write(data)
		return
	}
	switch msg["type"] {
	case "input":
		if v, ok := msg["data"].(string); ok {
			s.PTY.Write([]byte(v))
			s.mu.Lock()
			s.lastActivity = time.Now()
			s.mu.Unlock()
		}
	case "resize":
		if cols, ok1 := msg["cols"].(float64); ok1 {
			if rows, ok2 := msg["rows"].(float64); ok2 {
				s.PTY.SetSize(uint16(cols), uint16(rows))
			}
		}
	case "close":
		s.Cancel()
	}
}

// CloseSession 关闭会话
func (m *Manager) CloseSession(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	s.Status = "closed"
	s.Cancel()
	s.PTY.Close()
	s.WS.Close()
}

// ListSessions 列出活动会话
func (m *Manager) ListSessions() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]map[string]interface{}, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, map[string]interface{}{
			"id":      s.ID,
			"user":    s.User,
			"type":    s.Type,
			"host":    s.Host,
			"port":    s.Port,
			"status":  s.Status,
			"started": s.StartedAt.Format(time.RFC3339),
		})
	}
	return list
}

// CloseAll 关闭所有会话
func (m *Manager) CloseAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		m.CloseSession(id)
	}
}
