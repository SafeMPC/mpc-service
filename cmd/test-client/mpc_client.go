package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// MPCClient MPC 协议客户端（模拟实现）
type MPCClient struct {
	wsClient *WebSocketClient
	nodeID   string
}

// NewMPCClient 创建新的 MPC 客户端
func NewMPCClient(wsClient *WebSocketClient, nodeID string) *MPCClient {
	return &MPCClient{
		wsClient: wsClient,
		nodeID:   nodeID,
	}
}

// SendProtocolMessage 发送协议消息
func (c *MPCClient) SendProtocolMessage(ctx context.Context, sessionID string, messageData []byte, round int32, toNodeID string, clientSignature []byte) error {
	msg := &WebSocketMessage{
		Type:      "protocol_message",
		SessionID: sessionID,
		From:      c.nodeID,
		To:        toNodeID,
		Data:      hex.EncodeToString(messageData),
		Round:     round,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if len(clientSignature) > 0 {
		msg.ClientSignature = hex.EncodeToString(clientSignature)
	}

	return c.wsClient.SendMessage(ctx, msg)
}

// ReceiveProtocolMessage 接收协议消息
func (c *MPCClient) ReceiveProtocolMessage(ctx context.Context, timeout time.Duration) (*WebSocketMessage, error) {
	return c.wsClient.ReceiveMessage(ctx, timeout)
}

// SimulateDKGMessage 模拟 DKG 协议消息（用于测试）
func (c *MPCClient) SimulateDKGMessage(sessionID string, round int32) []byte {
	// 模拟一个简单的协议消息
	// 注意：真实的实现应该使用 tss-lib 生成消息
	message := fmt.Sprintf("DKG|%s|%d|%s", sessionID, round, c.nodeID)
	return []byte(message)
}

// SimulateSignMessage 模拟签名协议消息（用于测试）
func (c *MPCClient) SimulateSignMessage(sessionID string, round int32, messageHash []byte) []byte {
	// 模拟一个简单的协议消息
	// 注意：真实的实现应该使用 tss-lib 生成消息
	message := fmt.Sprintf("SIGN|%s|%d|%s|%x", sessionID, round, c.nodeID, messageHash)
	return []byte(message)
}

// SimulateClientSignature 模拟 Client 签名（用于测试）
func (c *MPCClient) SimulateClientSignature(sessionID, fromNodeID, toNodeID string, messageData []byte, round int32) []byte {
	// 模拟一个简单的签名
	// 注意：真实的实现应该使用 Passkey 私钥签名
	signatureData := fmt.Sprintf("%s|%s|%s|%x|%d", sessionID, fromNodeID, toNodeID, messageData, round)
	return []byte(signatureData)
}

// TestDKGFlow 测试 DKG 流程（模拟）
func (c *MPCClient) TestDKGFlow(ctx context.Context, sessionID string) error {
	log.Info().Str("session_id", sessionID).Msg("Starting DKG flow simulation...")

	// 模拟 DKG 协议的几轮消息交换
	for round := int32(1); round <= 4; round++ {
		log.Debug().Int32("round", round).Msg("DKG round")

		// 生成模拟消息
		messageData := c.SimulateDKGMessage(sessionID, round)

		// 生成模拟签名
		clientSignature := c.SimulateClientSignature(sessionID, c.nodeID, "server-signer-p2", messageData, round)

		// 发送消息
		if err := c.SendProtocolMessage(ctx, sessionID, messageData, round, "server-signer-p2", clientSignature); err != nil {
			return fmt.Errorf("failed to send DKG message round %d: %w", round, err)
		}

		// 等待回复（可选）
		if round < 4 {
			reply, err := c.ReceiveProtocolMessage(ctx, 5*time.Second)
			if err != nil {
				log.Warn().Err(err).Msg("No reply received, continuing...")
			} else {
				log.Debug().
					Str("type", reply.Type).
					Int32("round", reply.Round).
					Msg("Received DKG reply")
			}
		}

		// 短暂延迟
		time.Sleep(500 * time.Millisecond)
	}

	log.Info().Str("session_id", sessionID).Msg("DKG flow simulation completed")
	return nil
}

// TestSignFlow 测试签名流程（模拟）
func (c *MPCClient) TestSignFlow(ctx context.Context, sessionID string, messageHash []byte) error {
	log.Info().Str("session_id", sessionID).Msg("Starting sign flow simulation...")

	// 模拟签名协议的几轮消息交换
	for round := int32(1); round <= 6; round++ {
		log.Debug().Int32("round", round).Msg("Sign round")

		// 生成模拟消息
		messageData := c.SimulateSignMessage(sessionID, round, messageHash)

		// 生成模拟签名
		clientSignature := c.SimulateClientSignature(sessionID, c.nodeID, "server-signer-p2", messageData, round)

		// 发送消息
		if err := c.SendProtocolMessage(ctx, sessionID, messageData, round, "server-signer-p2", clientSignature); err != nil {
			return fmt.Errorf("failed to send sign message round %d: %w", round, err)
		}

		// 等待回复（可选）
		if round < 6 {
			reply, err := c.ReceiveProtocolMessage(ctx, 5*time.Second)
			if err != nil {
				log.Warn().Err(err).Msg("No reply received, continuing...")
			} else {
				log.Debug().
					Str("type", reply.Type).
					Int32("round", reply.Round).
					Msg("Received sign reply")
			}
		}

		// 短暂延迟
		time.Sleep(500 * time.Millisecond)
	}

	log.Info().Str("session_id", sessionID).Msg("Sign flow simulation completed")
	return nil
}
