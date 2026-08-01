// Package handler - WebSocket 通用推送端点（订阅/广播）
package handler

import (
	"admin-core/internal/ws"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WSHandler WebSocket 管理器
type WSHandler struct {
	Hub *ws.Hub
}

// NewWSHandler 创建
func NewWSHandler(hub *ws.Hub) *WSHandler { return &WSHandler{Hub: hub} }

// Handle 通用订阅端点：客户端 connect 后发送 {"action":"subscribe","topic":"system:stat"}
func (h *WSHandler) Handle(c *gin.Context) {
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	// 心跳
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			if _, _, err := wsConn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			return
		}
		var req struct {
			Action string `json:"action"`
			Topic  string `json:"topic"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		switch req.Action {
		case "subscribe":
			if req.Topic == "" {
				continue
			}
			client, unsub := h.Hub.Subscribe(req.Topic)
			defer unsub()
			go func() {
				for {
					m, ok := client.Read(0)
					if !ok {
						return
					}
					wsConn.WriteMessage(websocket.TextMessage, m)
				}
			}()
			// 保持连接，直到 WS 断开
			<-done
			return
		case "ping":
			wsConn.WriteJSON(map[string]interface{}{"type": "pong"})
		}
	}
}

// HubStats 调试端点
func (h *WSHandler) HubStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"clients": h.Hub.ClientCount(),
	})
}

// WebSocketSSE 备用方案：SSE 推送（无需 WS 协议的前端可使用）
type WSSE struct {
	Hub *ws.Hub
}

func NewWSSE(hub *ws.Hub) *WSSE { return &WSSE{Hub: hub} }

func (s *WSSE) Handle(c *gin.Context) {
	topic := c.Query("topic")
	if topic == "" {
		c.JSON(400, gin.H{"error": "missing topic"})
		return
	}
	client, unsub := s.Hub.Subscribe(topic)
	defer unsub()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}

	log.Printf("[SSE] subscribe topic=%s", topic)
	for {
		m, ok := client.Read(5000)
		if ok {
			c.SSEvent("message", string(m))
			flusher.Flush()
		}
	}
}
