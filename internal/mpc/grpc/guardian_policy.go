package grpc

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	pb "github.com/kashguard/go-mpc-infra/internal/pb/mpc/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// checkGuardianPolicy 执行鉴权代理策略检查
// 验证规则：
// 1. 获取钱包的签名策略（单人/团队，阈值）
// 2. 验证所有 AuthToken 的签名有效性
// 3. 统计有效签名数量是否满足阈值
func (s *GRPCServer) checkGuardianPolicy(ctx context.Context, req *pb.StartSignRequest) error {
	// 1. 加载策略
	// 如果没有配置策略，默认回退到 "单人模式" (threshold=1)
	policy, err := s.metadataStore.GetSigningPolicy(ctx, req.KeyId)
	if err != nil {
		// 如果找不到策略，但在 Guardian 模式下，这是一个安全风险
		// 我们可以选择报错，或者允许默认行为（仅当明确允许时）
		// 这里为了安全，如果找不到策略则默认需要至少 1 个签名
		log.Warn().Err(err).Str("key_id", req.KeyId).Msg("Failed to load signing policy, defaulting to single signature requirement")
		policy = nil
	}

	minSignatures := 1
	if policy != nil {
		minSignatures = policy.MinSignatures
	}

	// 2. 加载允许的公钥列表
	allowedKeys, err := s.metadataStore.ListUserAuthKeys(ctx, req.KeyId)
	if err != nil {
		return errors.Wrap(err, "failed to list user auth keys")
	}

	// 构建公钥白名单 map 用于快速查找
	allowedKeysMap := make(map[string]bool)
	for _, key := range allowedKeys {
		// 统一转为小写 hex
		allowedKeysMap[strings.ToLower(key.PublicKeyHex)] = true
	}

	// 3. 验证签名
	validSigCount := 0
	visitedKeys := make(map[string]bool)

	// 准备待签名的消息
	// 注意：APP 端签名的内容必须与这里一致。
	// 通常是对 MessageHex 进行签名，或者是对 Message 的哈希
	// 这里假设是对 MessageHex (交易哈希) 的签名
	msgToVerify := req.Message
	if len(msgToVerify) == 0 && req.MessageHex != "" {
		var decodeErr error
		msgToVerify, decodeErr = hex.DecodeString(req.MessageHex)
		if decodeErr != nil {
			return errors.Wrap(decodeErr, "invalid message hex")
		}
	}

	for _, token := range req.GetAuthTokens() {
		pubKeyHex := strings.ToLower(token.GetPublicKey())

		// A. 公钥白名单检查
		if !allowedKeysMap[pubKeyHex] {
			log.Warn().Str("public_key", pubKeyHex).Msg("Guardian: Public key not in whitelist")
			continue
		}

		// B. 防重放/去重 (同一个公钥只算一次)
		if visitedKeys[pubKeyHex] {
			continue
		}

		// C. 验签
		// 目前支持 Ed25519 和 Secp256k1
		// 尝试推断密钥类型或尝试两种验证
		isValid := false

		// 尝试 Ed25519
		if len(pubKeyHex) == 64 { // 32 bytes hex
			isValid = verifyEd25519(pubKeyHex, msgToVerify, token.GetSignature())
		} else {
			// 尝试 Secp256k1 (压缩 33 bytes -> 66 hex, 未压缩 65 bytes -> 130 hex)
			isValid = verifySecp256k1(pubKeyHex, msgToVerify, token.GetSignature())
		}

		if isValid {
			validSigCount++
			visitedKeys[pubKeyHex] = true
			log.Debug().Str("public_key", pubKeyHex).Msg("Guardian: Valid signature found")
		} else {
			log.Warn().Str("public_key", pubKeyHex).Msg("Guardian: Invalid signature")
		}
	}

	// 4. 阈值检查
	if validSigCount < minSignatures {
		return fmt.Errorf("insufficient valid signatures: need %d, got %d", minSignatures, validSigCount)
	}

	return nil
}

// verifyEd25519 验证 Ed25519 签名
func verifyEd25519(pubKeyHex string, msg []byte, sig []byte) bool {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}

	// Ed25519 签名通常是 64 字节
	if len(sig) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(pubKeyBytes, msg, sig)
}

// verifySecp256k1 验证 Secp256k1 签名 (Ethereum style)
func verifySecp256k1(pubKeyHex string, msg []byte, sig []byte) bool {
	// go-ethereum 的 VerifySignature 需要 65 字节的签名 (R, S, V)
	// 如果传入的是 64 字节 (R, S)，需要自行处理

	// 1. 解析公钥
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false
	}

	// 2. 验证
	// 注意：Crypto.VerifySignature 期望的是 msg 的哈希（32字节），而不是原始消息
	// 但在这个上下文中，req.Message 通常已经是交易的哈希
	// 如果不是，APP 端和这里需要统一约定是否再做一次 Keccak256

	// 尝试作为 Ecrecover 恢复
	// 为了简化，我们使用 VerifySignature，它需要未压缩公钥
	// 如果是压缩公钥，需要先解压
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		return false
	}

	// 将公钥转为 bytes (未压缩)
	pubBytes := crypto.FromECDSAPub(pubKey)

	// 移除 V (如果存在) 并标准化为 64 字节
	sigNoRecovery := sig
	if len(sig) == 65 {
		sigNoRecovery = sig[:64]
	}

	return crypto.VerifySignature(pubBytes, msg, sigNoRecovery)
}
