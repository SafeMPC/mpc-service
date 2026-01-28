package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

// WebAuthnRegisterBeginResponse WebAuthn 注册开始响应
type WebAuthnRegisterBeginResponse struct {
	Options     interface{} `json:"options"`
	SessionData string      `json:"session_data"`
}

// WebAuthnRegisterFinishPayload WebAuthn 注册完成请求
type WebAuthnRegisterFinishPayload struct {
	UserID      string      `json:"user_id"`
	UserName    string      `json:"user_name"`
	SessionData string      `json:"session_data"`
	Response    interface{} `json:"response"`
}

// WebAuthnLoginBeginResponse WebAuthn 登录开始响应
type WebAuthnLoginBeginResponse struct {
	Options     interface{} `json:"options"`
	SessionData string      `json:"session_data"`
}

// WebAuthnLoginFinishPayload WebAuthn 登录完成请求
type WebAuthnLoginFinishPayload struct {
	UserID      string      `json:"user_id"`
	SessionData string      `json:"session_data"`
	Response    interface{} `json:"response"`
}

// WebAuthnLoginResponse WebAuthn 登录响应
type WebAuthnLoginResponse struct {
	Success     bool   `json:"success"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// TestWebAuthnRegistration 测试 WebAuthn 注册
func (c *TestClient) TestWebAuthnRegistration(ctx context.Context, userID string) error {
	// 1. 开始注册
	log.Debug().Str("user_id", userID).Msg("Starting WebAuthn registration...")

	beginReq := map[string]interface{}{
		"user_id":     userID,
		"user_name":   userID + "@example.com",
		"display_name": "Test User",
	}

	resp, err := c.makeRequest(ctx, "POST", "/api/v1/auth/webauthn/register/begin", beginReq, "")
	if err != nil {
		return fmt.Errorf("register begin failed: %w", err)
	}

	var beginResp WebAuthnRegisterBeginResponse
	if err := c.parseResponse(resp, &beginResp); err != nil {
		return fmt.Errorf("parse register begin response failed: %w", err)
	}

	log.Debug().Str("session_data", beginResp.SessionData).Msg("Register begin successful")

	// 2. 使用工具生成真实的 WebAuthn 凭证响应
	log.Debug().Msg("Generating WebAuthn credential using tool...")
	credential, err := generateWebAuthnCredential(beginResp.SessionData, "register")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to generate credential using tool, using mock data")
		// 回退到模拟数据
		credential = map[string]interface{}{
			"id":       base64.RawURLEncoding.EncodeToString([]byte("mock-credential-id")),
			"rawId":    base64.RawURLEncoding.EncodeToString([]byte("mock-credential-id")),
			"type":     "public-key",
			"response": map[string]interface{}{
				"clientDataJSON":    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.create","challenge":"`+beginResp.SessionData+`","origin":"http://localhost:8080"}`)),
				"attestationObject": base64.RawURLEncoding.EncodeToString([]byte("mock-attestation-object")),
			},
		}
	}
	credentialResponse := credential

	// 3. 完成注册
	finishReq := WebAuthnRegisterFinishPayload{
		UserID:      userID,
		UserName:    userID + "@example.com",
		SessionData: beginResp.SessionData,
		Response:    credentialResponse,
	}

	resp, err = c.makeRequest(ctx, "POST", "/api/v1/auth/webauthn/register/finish", finishReq, "")
	if err != nil {
		return fmt.Errorf("register finish failed: %w", err)
	}

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("register finish failed with status %d: %s", resp.StatusCode, string(body))
	}

	var finishResp map[string]interface{}
	if err := c.parseResponse(resp, &finishResp); err != nil {
		return fmt.Errorf("parse register finish response failed: %w", err)
	}

	log.Info().Interface("response", finishResp).Msg("WebAuthn registration completed")
	return nil
}

// TestWebAuthnLogin 测试 WebAuthn 登录
func (c *TestClient) TestWebAuthnLogin(ctx context.Context, userID string) (string, error) {
	// 1. 开始登录
	log.Debug().Str("user_id", userID).Msg("Starting WebAuthn login...")

	beginReq := map[string]interface{}{
		"user_id": userID,
	}

	resp, err := c.makeRequest(ctx, "POST", "/api/v1/auth/webauthn/login/begin", beginReq, "")
	if err != nil {
		return "", fmt.Errorf("login begin failed: %w", err)
	}

	var beginResp WebAuthnLoginBeginResponse
	if err := c.parseResponse(resp, &beginResp); err != nil {
		return "", fmt.Errorf("parse login begin response failed: %w", err)
	}

	log.Debug().Str("session_data", beginResp.SessionData).Msg("Login begin successful")

	// 2. 使用工具生成真实的 WebAuthn 登录断言
	// 注意：登录需要 credential ID 和私钥，这些应该在注册时保存
	// 为了简化，我们使用工具生成新的凭证（实际应该使用已注册的凭证）
	log.Debug().Msg("Generating WebAuthn assertion using tool...")
	
	// TODO: 在实际使用中，应该从之前的注册中获取 credential ID 和私钥
	// 这里我们生成一个新的凭证用于测试
	credential, err := generateWebAuthnCredential(beginResp.SessionData, "login")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to generate assertion using tool, using mock data")
		// 回退到模拟数据
		credential = map[string]interface{}{
			"id":       base64.RawURLEncoding.EncodeToString([]byte("mock-credential-id")),
			"rawId":    base64.RawURLEncoding.EncodeToString([]byte("mock-credential-id")),
			"type":     "public-key",
			"response": map[string]interface{}{
				"clientDataJSON":    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get","challenge":"`+beginResp.SessionData+`","origin":"http://localhost:8080"}`)),
				"authenticatorData": base64.RawURLEncoding.EncodeToString([]byte("mock-authenticator-data")),
				"signature":          base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
				"userHandle":        base64.RawURLEncoding.EncodeToString([]byte(userID)),
			},
		}
	}
	assertionResponse := credential

	// 3. 完成登录
	finishReq := WebAuthnLoginFinishPayload{
		UserID:      userID,
		SessionData: beginResp.SessionData,
		Response:    assertionResponse,
	}

	resp, err = c.makeRequest(ctx, "POST", "/api/v1/auth/webauthn/login/finish", finishReq, "")
	if err != nil {
		return "", fmt.Errorf("login finish failed: %w", err)
	}

	var loginResp WebAuthnLoginResponse
	if err := c.parseResponse(resp, &loginResp); err != nil {
		return "", fmt.Errorf("parse login finish response failed: %w", err)
	}

	if !loginResp.Success {
		return "", fmt.Errorf("login failed: success is false")
	}

	log.Info().Str("token", loginResp.AccessToken[:20]+"...").Msg("WebAuthn login completed")
	return loginResp.AccessToken, nil
}
