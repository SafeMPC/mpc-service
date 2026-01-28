package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// WebSocketMessage WebSocket 消息格式
type WebSocketMessage struct {
	Type           string `json:"type"`
	SessionID      string `json:"session_id"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Data           string `json:"data,omitempty"`
	Round          int32  `json:"round,omitempty"`
	Timestamp      string `json:"timestamp,omitempty"`
	ClientSignature string `json:"client_signature,omitempty"`
}

// WebSocketStatusUpdate 状态更新消息
type WebSocketStatusUpdate struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Round     int32  `json:"round,omitempty"`
	Message   string `json:"message,omitempty"`
}

// WebSocketClient WebSocket 客户端
type WebSocketClient struct {
	conn   *websocket.Conn
	url    string
	token  string
	sessionID string
}

// NewWebSocketClient 创建新的 WebSocket 客户端
func NewWebSocketClient(wsURL, token, sessionID string) *WebSocketClient {
	return &WebSocketClient{
		url:       wsURL,
		token:     token,
		sessionID: sessionID,
	}
}

// Connect 连接到 WebSocket 服务器
func (c *WebSocketClient) Connect(ctx context.Context) error {
	// 构建 WebSocket URL
	// WebSocket 路由在 APIV1Auth 组下，路径是 /api/v1/auth/ws
	url := fmt.Sprintf("%s/api/v1/auth/ws?token=%s&session_id=%s", c.url, c.token, c.sessionID)

	log.Debug().Str("url", url).Msg("Connecting to WebSocket...")

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.conn = conn
	log.Info().Msg("WebSocket connected successfully")
	return nil
}

// Close 关闭 WebSocket 连接
func (c *WebSocketClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendMessage 发送协议消息
func (c *WebSocketClient) SendMessage(ctx context.Context, msg *WebSocketMessage) error {
	if c.conn == nil {
		return fmt.Errorf("WebSocket not connected")
	}

	// 设置写入超时
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Debug().
		Str("type", msg.Type).
		Str("session_id", msg.SessionID).
		Int32("round", msg.Round).
		Msg("Message sent via WebSocket")

	return nil
}

// ReceiveMessage 接收消息（阻塞直到收到消息或超时）
func (c *WebSocketClient) ReceiveMessage(ctx context.Context, timeout time.Duration) (*WebSocketMessage, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("WebSocket not connected")
	}

	// 设置读取超时
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	// 读取消息
	var rawMsg json.RawMessage
	if err := c.conn.ReadJSON(&rawMsg); err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// 尝试解析为不同类型的消息
	var msg WebSocketMessage
	if err := json.Unmarshal(rawMsg, &msg); err != nil {
		// 如果不是协议消息，尝试解析为状态更新
		var statusUpdate WebSocketStatusUpdate
		if err := json.Unmarshal(rawMsg, &statusUpdate); err == nil {
			log.Debug().
				Str("type", statusUpdate.Type).
				Str("session_id", statusUpdate.SessionID).
				Str("status", statusUpdate.Status).
				Msg("Received status update")
			// 转换为通用消息格式
			msg.Type = statusUpdate.Type
			msg.SessionID = statusUpdate.SessionID
		} else {
			return nil, fmt.Errorf("failed to parse message: %w", err)
		}
	}

	log.Debug().
		Str("type", msg.Type).
		Str("session_id", msg.SessionID).
		Msg("Message received via WebSocket")

	return &msg, nil
}

// WaitForCompletion 等待会话完成（通过状态更新）
func (c *WebSocketClient) WaitForCompletion(ctx context.Context, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining > 30*time.Second {
			remaining = 30 * time.Second
		}

		msg, err := c.ReceiveMessage(ctx, remaining)
		if err != nil {
			// 检查是否是超时错误
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return "", err
			}
			// 超时，继续等待
			continue
		}

		// 检查是否是完成消息
		if msg.Type == "status_update" {
			// 解析状态更新
			var statusUpdate WebSocketStatusUpdate
			msgBytes, _ := json.Marshal(msg)
			json.Unmarshal(msgBytes, &statusUpdate)

			if statusUpdate.Status == "completed" {
				log.Info().Str("session_id", statusUpdate.SessionID).Msg("Session completed")
				return statusUpdate.SessionID, nil
			} else if statusUpdate.Status == "failed" {
				return "", fmt.Errorf("session failed: %s", statusUpdate.Message)
			}
		} else if msg.Type == "dkg_completed" || msg.Type == "sign_completed" {
			log.Info().Str("session_id", msg.SessionID).Msg("Session completed")
			return msg.SessionID, nil
		}
	}

	return "", fmt.Errorf("timeout waiting for session completion")
}
