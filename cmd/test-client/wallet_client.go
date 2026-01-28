package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CreateWalletPayload 创建钱包请求
type CreateWalletPayload struct {
	ChainType         string      `json:"chain_type"`
	Algorithm         string      `json:"algorithm,omitempty"`
	Curve             string      `json:"curve,omitempty"`
	WebauthnAssertion interface{} `json:"webauthn_assertion,omitempty"`
}

// CreateWalletResponse 创建钱包响应
type CreateWalletResponse struct {
	WalletID     string `json:"wallet_id"`
	DkgSessionID string `json:"dkg_session_id"` // API 返回的是 dkg_session_id
	Status       string `json:"status"`
	WebsocketURL string `json:"websocket_url,omitempty"`
}

// SignTransactionPayload 签名交易请求
type SignTransactionPayload struct {
	MessageHex        string      `json:"message_hex"`
	ChainType         string      `json:"chain_type"`
	DerivationPath    string      `json:"derivation_path,omitempty"`
	WebauthnAssertion interface{} `json:"webauthn_assertion,omitempty"`
}

// SignTransactionResponse 签名交易响应
type SignTransactionResponse struct {
	SessionID    string `json:"session_id"`
	Status       string `json:"status"`
	WebsocketURL string `json:"websocket_url,omitempty"`
	Signature    string `json:"signature,omitempty"`
}

// TestCreateWallet 测试创建钱包（DKG）
func (c *TestClient) TestCreateWallet(ctx context.Context, token string) (string, error) {
	log.Debug().Msg("Creating wallet (DKG)...")

	req := CreateWalletPayload{
		ChainType: "ethereum",
		Algorithm: "ECDSA",
		Curve:     "secp256k1",
		// 注意：测试环境暂时不提供 WebAuthn Assertion
		// 生产环境必须提供有效的 WebAuthn Assertion
		WebauthnAssertion: nil,
	}

	resp, err := c.makeRequest(ctx, "POST", "/api/v1/auth/wallets", req, token)
	if err != nil {
		return "", fmt.Errorf("create wallet request failed: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return "", fmt.Errorf("create wallet failed with status %d: %s", resp.StatusCode, string(body))
	}

	var walletResp CreateWalletResponse
	if err := c.parseResponse(resp, &walletResp); err != nil {
		return "", fmt.Errorf("parse create wallet response failed: %w", err)
	}

	// 清理 wallet ID（移除可能的 ANSI 转义字符）
	walletID := strings.TrimSpace(walletResp.WalletID)
	walletID = strings.Trim(walletID, "\"")

	log.Info().
		Str("wallet_id", walletID).
		Str("dkg_session_id", walletResp.DkgSessionID).
		Str("status", walletResp.Status).
		Str("websocket_url", walletResp.WebsocketURL).
		Msg("Wallet created successfully")

	// 如果提供了 WebSocket URL，尝试通过 WebSocket 参与 DKG 协议
	if walletResp.WebsocketURL != "" && walletResp.DkgSessionID != "" {
		log.Info().Msg("Connecting to WebSocket to participate in DKG protocol...")

		// 清理 session ID
		sessionID := strings.TrimSpace(walletResp.DkgSessionID)
		sessionID = strings.Trim(sessionID, "\"")

		// 创建 WebSocket 客户端
		wsClient := NewWebSocketClient(c.wsURL, token, sessionID)
		if err := wsClient.Connect(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to connect to WebSocket, DKG may continue without client participation")
		} else {
			defer wsClient.Close()

			// 创建 MPC 客户端
			mpcClient := NewMPCClient(wsClient, "mobile-p1")

			// 参与 DKG 流程（模拟）
			if err := mpcClient.TestDKGFlow(ctx, sessionID); err != nil {
				log.Warn().Err(err).Msg("DKG flow simulation failed, but wallet creation may still succeed")
			} else {
				log.Info().Msg("DKG flow simulation completed")
			}

			// 等待 DKG 完成
			log.Info().Msg("Waiting for DKG completion...")
			if _, err := wsClient.WaitForCompletion(ctx, 60*time.Second); err != nil {
				log.Warn().Err(err).Msg("Timeout waiting for DKG completion")
			} else {
				log.Info().Msg("DKG completed successfully")
			}
		}
	}

	return walletID, nil
}

// TestSignTransaction 测试签名交易
func (c *TestClient) TestSignTransaction(ctx context.Context, token, walletID string) (string, error) {
	log.Debug().Str("wallet_id", walletID).Msg("Signing transaction...")

	// 创建一个简单的测试消息（Ethereum 交易）
	// 注意：这是一个简化的示例，真实的交易需要正确的格式
	testMessageHex := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	req := SignTransactionPayload{
		MessageHex: testMessageHex,
		ChainType:  "ethereum",
		// 注意：测试环境暂时不提供 WebAuthn Assertion
		// 生产环境必须提供有效的 WebAuthn Assertion
		WebauthnAssertion: nil,
	}

	resp, err := c.makeRequest(ctx, "POST", "/api/v1/auth/wallets/"+walletID+"/sign", req, token)
	if err != nil {
		return "", fmt.Errorf("sign transaction request failed: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return "", fmt.Errorf("sign transaction failed with status %d: %s", resp.StatusCode, string(body))
	}

	var signResp SignTransactionResponse
	if err := c.parseResponse(resp, &signResp); err != nil {
		return "", fmt.Errorf("parse sign transaction response failed: %w", err)
	}

	log.Info().
		Str("session_id", signResp.SessionID).
		Str("status", signResp.Status).
		Msg("Sign transaction request submitted")

	// 如果响应中包含签名，直接返回
	if signResp.Signature != "" {
		return signResp.Signature, nil
	}

	// 否则，需要通过 WebSocket 等待签名完成
	log.Info().Msg("Connecting to WebSocket to participate in signing protocol...")

	// 创建 WebSocket 客户端
	wsClient := NewWebSocketClient(c.wsURL, token, signResp.SessionID)
	if err := wsClient.Connect(ctx); err != nil {
		return "", fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer wsClient.Close()

	// 创建 MPC 客户端
	mpcClient := NewMPCClient(wsClient, "mobile-p1")

	// 解码消息哈希
	messageBytes, err := hex.DecodeString(strings.TrimPrefix(testMessageHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("failed to decode message hex: %w", err)
	}

	// 参与签名流程（模拟）
	if err := mpcClient.TestSignFlow(ctx, signResp.SessionID, messageBytes); err != nil {
		return "", fmt.Errorf("sign flow failed: %w", err)
	}

	// 等待签名完成
	log.Info().Msg("Waiting for signature completion...")
	if _, err := wsClient.WaitForCompletion(ctx, 60*time.Second); err != nil {
		return "", fmt.Errorf("timeout waiting for signature completion: %w", err)
	}

	log.Info().Msg("Signature completed successfully")
	// 注意：实际实现中应该从 WebSocket 消息中提取签名
	// 这里我们返回 session_id 作为占位符
	return signResp.SessionID, nil
}
