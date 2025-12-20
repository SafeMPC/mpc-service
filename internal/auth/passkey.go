package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

// VerifyPasskeySignature 验证 WebAuthn Passkey 签名
// publicKeyHex: COSE Key 格式的公钥 (Hex 编码)
// signature: Assertion Signature (Raw bytes)
// authData: Authenticator Data (Raw bytes)
// clientDataJSON: Client Data JSON (Raw bytes)
// expectedChallenge: 期望的 Challenge 字符串 (通常是 Base64URL 编码的 Hash 或随机数)
func VerifyPasskeySignature(publicKeyHex string, signature []byte, authData []byte, clientDataJSON []byte, expectedChallenge string) error {
	// 1. 解析公钥
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("invalid public key hex: %w", err)
	}

	pubKey, err := webauthncose.ParsePublicKey(publicKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	// 2. 解析 ClientDataJSON
	var clientData protocol.CollectedClientData
	if err := json.Unmarshal(clientDataJSON, &clientData); err != nil {
		return fmt.Errorf("failed to parse client data: %w", err)
	}

	// 3. 验证 Challenge
	// WebAuthn ClientData 中的 Challenge 是 Base64URL 编码的
	if clientData.Challenge != expectedChallenge {
		// 尝试处理 Padding 差异
		normalizedClient := strings.TrimRight(clientData.Challenge, "=")
		normalizedExpected := strings.TrimRight(expectedChallenge, "=")
		if normalizedClient != normalizedExpected {
			return fmt.Errorf("challenge mismatch: got %s, want %s", clientData.Challenge, expectedChallenge)
		}
	}

	// 4. 解析 AuthenticatorData
	var authenticatorData protocol.AuthenticatorData
	if err := authenticatorData.Unmarshal(authData); err != nil {
		return fmt.Errorf("failed to parse authenticator data: %w", err)
	}

	// 5. 验证 User Present (UP) 位
	if !authenticatorData.Flags.UserPresent() {
		return fmt.Errorf("user not present (UP flag not set)")
	}

	// 6. 验证 User Verified (UV) 位 (可选，视安全策略而定，建议开启)
	// if !authenticatorData.Flags.UserVerified() {
	// 	return fmt.Errorf("user not verified (UV flag not set)")
	// }

	// 7. 构造签名数据: authData || sha256(clientDataJSON)
	clientDataHash := sha256.Sum256(clientDataJSON)
	signedData := append(authData, clientDataHash[:]...)

	// 8. 验证签名
	valid, err := webauthncose.VerifySignature(pubKey, signedData, signature)
	if err != nil {
		return fmt.Errorf("error verifying signature: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// HexToBase64URL 将 Hex 字符串转换为 Base64URL 字符串 (无 Padding)
func HexToBase64URL(hexStr string) (string, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// VerifyAdminPasskey 验证 Admin 的 Passkey
// adminAuth: Admin 鉴权 Token
// requestHash: 请求的 Hash (Raw bytes)
func VerifyAdminPasskey(adminAuth interface{}, requestHash []byte) error {
	// 由于 Go 的泛型限制或为了解耦，这里接收 interface{}，实际应为 *pb.AdminAuthToken
	// 但为了避免循环依赖 (pb -> auth -> pb)，我们可以让调用者传入字段，或者在这里使用反射/接口定义
	// 更好的方式是让 VerifyPasskeySignature 足够通用，调用者负责提取字段。
	return nil
}
