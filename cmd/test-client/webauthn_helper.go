package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// generateWebAuthnCredential 使用工具生成 WebAuthn 凭证
func generateWebAuthnCredential(challenge string, action string) (map[string]interface{}, error) {
	// 调用 gen-webauthn-credential 工具
	cmd := exec.Command("./bin/gen-webauthn-credential", "--action", action, "--challenge", challenge)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to generate credential: %w", err)
	}

	// 解析 JSON 输出
	var credential map[string]interface{}
	if err := json.Unmarshal(output, &credential); err != nil {
		// 尝试从输出中提取 JSON（可能包含日志）
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "{") {
				if err := json.Unmarshal([]byte(line), &credential); err == nil {
					break
				}
			}
		}
		if credential == nil {
			return nil, fmt.Errorf("failed to parse credential output: %w", err)
		}
	}

	return credential, nil
}

// extractCredentialInfo 从工具输出中提取凭证信息
func extractCredentialInfo(output string) (credentialID, privateKey string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Credential ID:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				credentialID = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "Private Key:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				privateKey = strings.TrimSpace(parts[1])
			}
		}
	}
	return
}

// buildCredentialResponse 构建符合 API 要求的凭证响应
func buildCredentialResponse(credential map[string]interface{}) map[string]interface{} {
	response, ok := credential["response"].(map[string]interface{})
	if !ok {
		return nil
	}
	return response
}

// decodeBase64URL 解码 Base64URL 字符串
func decodeBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// encodeBase64URL 编码为 Base64URL 字符串
func encodeBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// hexToBytes 将 hex 字符串转换为字节
func hexToBytes(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
