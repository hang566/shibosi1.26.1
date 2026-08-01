// Package ws 提供 WebSocket Hub，用于广播实时系统状态、任务进度、日志流
package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Hub 维护所有 WebSocket 连接，支持按 topic 订阅
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*client]struct{} // topic -> clients
	broadcast chan *Event
	register   chan *subscription
	unregister chan *subscription
}

type client struct {
	send  chan []byte
	done  chan struct{}
}

type subscription struct {
	topic  string
	client *client
}

type Event struct {
	Topic   string      `json:"topic"`
	Data    interface{} `json:"data"`
	Time    int64       `json:"time"`
	Type    string      `json:"type"`
}

// NewHub 创建新的 Hub
func NewHub() *Hub {
	h := &Hub{
		clients:   make(map[string]map[*client]struct{}),
		broadcast: make(chan *Event, 256),
		register:  make(chan *subscription, 32),
		unregister: make(chan *subscription, 32),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case sub := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[sub.topic]; !ok {
				h.clients[sub.topic] = make(map[*client]struct{})
			}
			h.clients[sub.topic][sub.client] = struct{}{}
			h.mu.Unlock()
		case sub := <-h.unregister:
			h.mu.Lock()
			if m, ok := h.clients[sub.topic]; ok {
				delete(m, sub.client)
				if len(m) == 0 {
					delete(h.clients, sub.topic)
				}
			}
			h.mu.Unlock()
			close(sub.client.send)
		case ev := <-h.broadcast:
			h.mu.RLock()
			var targets []*client
			if ev.Topic == "*" {
				for _, m := range h.clients {
					for c := range m {
						targets = append(targets, c)
					}
				}
			} else if m, ok := h.clients[ev.Topic]; ok {
				for c := range m {
					targets = append(targets, c)
				}
			}
			h.mu.RUnlock()

			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			for _, c := range targets {
				select {
				case c.send <- payload:
				default:
					// 缓冲区满，丢弃
				}
			}
		}
	}
}

// Subscribe 订阅某个 topic，返回 client 和取消函数
func (h *Hub) Subscribe(topic string) (*client, func()) {
	c := &client{send: make(chan []byte, 256), done: make(chan struct{})}
	h.register <- &subscription{topic: topic, client: c}
	unsub := func() {
		h.unregister <- &subscription{topic: topic, client: c}
	}
	return c, unsub
}

// Publish 向指定 topic 广播事件
func (h *Hub) Publish(topic string, evType string, data interface{}) {
	h.broadcast <- &Event{Topic: topic, Type: evType, Data: data, Time: time.Now().UnixMilli()}
}

// PublishAll 向所有连接广播
func (h *Hub) PublishAll(evType string, data interface{}) {
	h.broadcast <- &Event{Topic: "*", Type: evType, Data: data, Time: time.Now().UnixMilli()}
}

// ClientRead 从 client 读取消息（非阻塞，带超时）
func (c *client) Read(timeout time.Duration) ([]byte, bool) {
	select {
	case msg, ok := <-c.send:
		return msg, ok
	case <-time.After(timeout):
		return nil, false
	case <-c.done:
		return nil, false
	}
}

// ClientDone 关闭信号
func (c *client) Done() <-chan struct{} { return c.done }

// CloseClient 主动关闭
func (c *client) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

// ClientCount 返回当前连接数（调试）
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, m := range h.clients {
		n += len(m)
	}
	return n
}

// LogError 错误日志辅助
func logError(err error) {
	if err != nil {
		log.Println("[ws] error:", err)
	}
}
