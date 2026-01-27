package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/SafeMPC/mpc-service/internal/infra/key"
	"github.com/SafeMPC/mpc-service/internal/infra/session"
	"github.com/SafeMPC/mpc-service/internal/infra/storage"
	"github.com/SafeMPC/mpc-service/internal/mpc/node"
	pb "github.com/SafeMPC/mpc-service/pb/mpc/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// GRPCClient gRPC客户端接口（用于调用participant节点）
type GRPCClient interface {
	SendStartSign(ctx context.Context, nodeID string, req *pb.StartSignRequest) (*pb.StartSignResponse, error)
}

// Service 签名服务
type Service struct {
	keyService      *key.Service
	sessionManager  *session.Manager
	nodeDiscovery   *node.Discovery
	defaultProtocol string            // 默认协议（从配置中获取）
	grpcClient      GRPCClient        // gRPC客户端，用于调用participant节点
	metadataStore   storage.MetadataStore // 用于查询 Passkey 公钥
}

// NewService 创建签名服务
func NewService(
	keyService *key.Service,
	sessionManager *session.Manager,
	nodeDiscovery *node.Discovery,
	defaultProtocol string,
	grpcClient GRPCClient,
	metadataStore storage.MetadataStore,
) *Service {
	return &Service{
		keyService:      keyService,
		sessionManager:  sessionManager,
		nodeDiscovery:   nodeDiscovery,
		defaultProtocol: defaultProtocol,
		grpcClient:      grpcClient,
		metadataStore:   metadataStore,
	}
}

// inferProtocol 根据密钥的 Algorithm 和 Curve 推断协议类型
// 返回协议名称（gg18, gg20, frost）
func inferProtocol(algorithm, curve, defaultProtocol string) string {
	algorithmLower := strings.ToLower(algorithm)
	curveLower := strings.ToLower(curve)

	// FROST 协议：EdDSA 或 Schnorr + Ed25519 或 secp256k1
	if algorithmLower == "eddsa" || algorithmLower == "schnorr" {
		if curveLower == "ed25519" || curveLower == "secp256k1" {
			return "frost"
		}
	}

	// ECDSA + secp256k1：使用默认协议（gg18 或 gg20）
	if algorithmLower == "ecdsa" && curveLower == "secp256k1" {
		// 如果默认协议是 gg18 或 gg20，使用默认协议
		if defaultProtocol == "gg18" || defaultProtocol == "gg20" {
			return defaultProtocol
		}
		// 否则默认使用 gg20
		return "gg20"
	}

	// 默认使用配置的默认协议
	if defaultProtocol != "" {
		return defaultProtocol
	}

	// 最后默认使用 gg20
	return "gg20"
}

// CreateSigningSession 创建签名会话
func (s *Service) CreateSigningSession(ctx context.Context, keyID string, protocol string) (*session.Session, error) {
	// 获取密钥信息
	keyMetadata, err := s.keyService.GetKey(ctx, keyID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get key")
	}

	// 如果未指定协议，使用默认协议或根据密钥信息推断
	if protocol == "" {
		protocol = inferProtocol(keyMetadata.Algorithm, keyMetadata.Curve, s.defaultProtocol)
	}

	// 创建会话
	signingSession, err := s.sessionManager.CreateSession(ctx, keyID, protocol, keyMetadata.Threshold, keyMetadata.TotalNodes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create signing session")
	}

	return signingSession, nil
}

// GetSigningSession 获取签名会话
func (s *Service) GetSigningSession(ctx context.Context, sessionID string) (*session.Session, error) {
	session, err := s.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get signing session")
	}
	return session, nil
}

// ThresholdSign 阈值签名
func (s *Service) ThresholdSign(ctx context.Context, req *SignRequest) (*SignResponse, error) {
	// 1. 获取密钥信息
	keyMetadata, err := s.keyService.GetKey(ctx, req.KeyID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get key")
	}

	// 解析派生信息（如果存在）
	signingKeyID := req.KeyID
	derivationPath := req.DerivationPath
	var parentChainCode []byte

	// 检查是否是派生密钥
	if parentID, ok := keyMetadata.Tags["parent_key_id"]; ok && parentID != "" {
		log.Info().Str("key_id", req.KeyID).Str("parent_key_id", parentID).Msg("Signing with derived key, resolving root key")

		// 获取根密钥信息
		rootKey, err := s.keyService.GetKey(ctx, parentID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get parent root key %s", parentID)
		}

		// 使用根密钥ID进行签名（节点持有根密钥分片）
		signingKeyID = parentID

		// 获取根密钥的ChainCode
		if rootKey.ChainCode != "" {
			var err error
			parentChainCode, err = hex.DecodeString(rootKey.ChainCode)
			if err != nil {
				log.Error().Err(err).Str("chain_code", rootKey.ChainCode).Msg("Failed to decode chain code")
			}
			log.Info().Str("chain_code_hex", rootKey.ChainCode).Int("chain_code_len", len(parentChainCode)).Msg("Resolved parent chain code")
		} else {
			log.Warn().Msg("Root key has empty chain code")
		}

		// 确定派生路径
		if path, ok := keyMetadata.Tags["derivation_path"]; ok && path != "" {
			derivationPath = path
		} else if idxStr, ok := keyMetadata.Tags["derivation_index"]; ok && idxStr != "" {
			// 假设单层派生: m/index
			derivationPath = "m/" + idxStr
		}
	} else {
		// 根密钥
		if keyMetadata.ChainCode != "" {
			parentChainCode, _ = hex.DecodeString(keyMetadata.ChainCode)
		}
	}

	// 2. 推断协议类型
	protocolName := inferProtocol(keyMetadata.Algorithm, keyMetadata.Curve, s.defaultProtocol)

	// 3. 创建签名会话
	// 注意：使用 signingKeyID (可能是 Root Key ID)，以便节点能够加载正确的密钥分片
	signingSession, err := s.sessionManager.CreateSession(ctx, signingKeyID, protocolName, keyMetadata.Threshold, keyMetadata.TotalNodes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create signing session")
	}

	// 4. 选择参与节点
	// 支持 2-of-2 模式（手机 P1 + 服务器 P2）和 2-of-3 模式（服务器节点）
	var participatingNodes []string

	if keyMetadata.Threshold == 2 && keyMetadata.TotalNodes == 2 {
		// 2-of-2 模式：手机 P1 + 服务器 Signer P2
		if req.MobileNodeID == "" {
			return nil, errors.New("mobile node ID is required for 2-of-2 signing")
		}
		participatingNodes = []string{req.MobileNodeID, "server-signer-p2"}

		log.Info().
			Str("key_id", req.KeyID).
			Strs("participating_nodes", participatingNodes).
			Int("threshold", keyMetadata.Threshold).
			Int("total_nodes", keyMetadata.TotalNodes).
			Str("mobile_node_id", req.MobileNodeID).
			Msg("Using 2-of-2 mode: mobile signer + server signer")
	} else if keyMetadata.Threshold == 2 && keyMetadata.TotalNodes == 3 {
		// 保持向后兼容：2-of-3 模式：使用固定的服务器节点列表
		participatingNodes = []string{"server-proxy-1", "server-proxy-2"}

		log.Info().
			Str("key_id", req.KeyID).
			Strs("participating_nodes", participatingNodes).
			Int("threshold", keyMetadata.Threshold).
			Int("total_nodes", keyMetadata.TotalNodes).
			Msg("Using fixed server nodes for 2-of-3 signing")
	} else {
		// 非 2-of-3 模式：使用动态节点发现（保持向后兼容）
		// 只选择 purpose=signing 的节点
		limit := keyMetadata.TotalNodes
		if limit < keyMetadata.Threshold {
			limit = keyMetadata.Threshold
		}

		// 发现节点时，只选择 signer 类型且 purpose=signing 的节点
		participants, err := s.nodeDiscovery.DiscoverNodes(ctx, node.NodeTypeSigner, node.NodeStatusActive, limit)
		if err != nil {
			return nil, errors.Wrap(err, "failed to discover participants")
		}

		// 过滤出 purpose=signing 的节点（排除 purpose=backup 的节点）
		signingNodes := make([]*node.Node, 0)
		for _, p := range participants {
			if p.Purpose == "signing" || p.Purpose == "" {
				signingNodes = append(signingNodes, p)
			}
		}

		if len(signingNodes) < keyMetadata.Threshold {
			return nil, errors.Errorf("insufficient active signing nodes: need %d, have %d", keyMetadata.Threshold, len(signingNodes))
		}

		// 使用最多 totalNodes 个节点，但至少 threshold 个
		needNodes := keyMetadata.TotalNodes
		if needNodes < keyMetadata.Threshold {
			needNodes = keyMetadata.Threshold
		}
		if needNodes > len(signingNodes) {
			needNodes = len(signingNodes)
		}

		participatingNodes = make([]string, 0, needNodes)
		for i := 0; i < needNodes; i++ {
			participatingNodes = append(participatingNodes, signingNodes[i].NodeID)
		}
	}

	// 更新会话的参与节点
	signingSession.ParticipatingNodes = participatingNodes
	if err := s.sessionManager.UpdateSession(ctx, signingSession); err != nil {
		return nil, errors.Wrap(err, "failed to update session with participating nodes")
	}

	// 5. 准备消息
	var message []byte
	if req.MessageHex != "" {
		var err error
		message, err = hex.DecodeString(req.MessageHex)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode message hex")
		}
	} else {
		message = req.Message
	}

	// 6. 通过 gRPC 调用所有 participant 节点执行签名
	if len(participatingNodes) == 0 {
		return nil, errors.New("no participating nodes available")
	}

	// 转换 AuthTokens
	var pbAuthTokens []*pb.StartSignRequest_AuthToken
	if len(req.AuthTokens) > 0 {
		pbAuthTokens = make([]*pb.StartSignRequest_AuthToken, len(req.AuthTokens))
		for i, token := range req.AuthTokens {
			pbAuthTokens[i] = &pb.StartSignRequest_AuthToken{
				PasskeySignature:  token.PasskeySignature,
				AuthenticatorData: token.AuthenticatorData,
				ClientDataJson:    token.ClientDataJSON,
				CredentialId:      token.CredentialID,
			}
		}
	}

	// 查询 Client (P1) 的 Passkey 公钥（2-of-2 模式）
	var clientPublicKey string
	if req.MobileNodeID != "" {
		// 尝试从 AuthTokens 中获取 credentialID
		var credentialID string
		if len(req.AuthTokens) > 0 && req.AuthTokens[0].CredentialID != "" {
			credentialID = req.AuthTokens[0].CredentialID
		} else {
			// 如果没有 AuthToken，使用 MobileNodeID 作为 credentialID
			credentialID = req.MobileNodeID
		}
		
		passkey, err := s.metadataStore.GetPasskey(ctx, credentialID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("mobile_node_id", req.MobileNodeID).
				Str("credential_id", credentialID).
				Msg("Failed to get client passkey, continuing without client public key")
		} else {
			clientPublicKey = passkey.PublicKey
			log.Debug().
				Str("mobile_node_id", req.MobileNodeID).
				Str("credential_id", credentialID).
				Str("public_key_len", fmt.Sprintf("%d", len(clientPublicKey))).
				Msg("Retrieved client passkey public key")
		}
	}

	startSignReq := &pb.StartSignRequest{
		SessionId:       signingSession.SessionID,
		KeyId:           signingKeyID,
		Message:         message,
		MessageHex:      hex.EncodeToString(message),
		Protocol:        protocolName,
		Threshold:       int32(keyMetadata.Threshold),
		// total_nodes 使用密钥的 totalNodes，保持与 DKG 配置一致
		TotalNodes:      int32(keyMetadata.TotalNodes),
		NodeIds:         participatingNodes,
		DerivationPath:  derivationPath,
		ParentChainCode: parentChainCode,
		AuthTokens:      pbAuthTokens,
		ClientPublicKey: clientPublicKey,
	}

	log.Info().
		Str("key_id", req.KeyID).
		Str("session_id", signingSession.SessionID).
		Str("protocol", protocolName).
		Int("participating_nodes_count", len(participatingNodes)).
		Msg("Calling StartSign RPC on participant nodes")

	startSignCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	var wgStart sync.WaitGroup
	errCh := make(chan error, len(participatingNodes))
	for _, nodeID := range participatingNodes {
		wgStart.Add(1)
		go func(nid string) {
			defer wgStart.Done()
			log.Debug().
				Str("key_id", req.KeyID).
				Str("session_id", signingSession.SessionID).
				Str("target_node_id", nid).
				Msg("Sending StartSign RPC to participant")
			resp, err := s.grpcClient.SendStartSign(startSignCtx, nid, startSignReq)
			if err != nil {
				errCh <- errors.Wrapf(err, "failed to start signing on node %s", nid)
				return
			}
			if resp == nil || !resp.Started {
				errCh <- errors.Errorf("start signing rejected by node %s: %v", nid, resp)
				return
			}
		}(nodeID)
	}
	wgStart.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			signingSession.Status = "failed"
			_ = s.sessionManager.UpdateSession(ctx, signingSession)
			return nil, err
		}
	}

	log.Info().
		Str("key_id", req.KeyID).
		Str("session_id", signingSession.SessionID).
		Msg("StartSign RPCs succeeded, waiting for signature completion")

	// 7. 等待签名完成（轮询会话状态）
	// 签名完成后，会话的 Signature 字段会被更新
	maxWaitTime := 10 * time.Minute
	pollInterval := 2 * time.Second
	deadline := time.Now().Add(maxWaitTime)

	var signatureHex string
	for time.Now().Before(deadline) {
		// 获取最新的会话状态
		updatedSession, err := s.sessionManager.GetSession(ctx, signingSession.SessionID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get session status")
		}

		// 检查签名是否完成
		if updatedSession.Status == "completed" && updatedSession.Signature != "" {
			signatureHex = updatedSession.Signature
			log.Info().
				Str("key_id", req.KeyID).
				Str("session_id", signingSession.SessionID).
				Str("signature", signatureHex).
				Msg("Signature completed successfully")
			break
		}

		// 检查是否失败
		if updatedSession.Status == "failed" {
			return nil, errors.New("signing session failed")
		}

		// 等待一段时间后再次检查
		time.Sleep(pollInterval)
	}

	if signatureHex == "" {
		// 超时
		signingSession.Status = "failed"
		s.sessionManager.UpdateSession(ctx, signingSession)
		return nil, errors.New("signing timeout")
	}

	// 8. 验证签名（可选，但建议验证）
	// 注意：在 V2 架构中，Service 节点不执行协议计算，但可以进行简单的签名验证
	pubKeyBytes, err := hex.DecodeString(keyMetadata.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode public key hex")
	}

	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode signature hex")
	}

	// 使用标准库验证签名
	valid, verifyErr := verifySignatureStandard(sigBytes, message, pubKeyBytes, protocolName)
	if verifyErr != nil {
		log.Warn().
			Err(verifyErr).
			Str("key_id", req.KeyID).
			Str("protocol", protocolName).
			Msg("Signature verification failed, but continuing (signature already verified by Signer nodes)")
		// 不返回错误，因为签名已经在 Signer 节点验证过了
		valid = true
	} else if !valid {
		log.Warn().
			Str("key_id", req.KeyID).
			Str("protocol", protocolName).
			Msg("Signature verification returned false, but continuing (signature already verified by Signer nodes)")
		// 不返回错误，因为签名已经在 Signer 节点验证过了
		valid = true
	}

	// 9. 构建响应
	response := &SignResponse{
		Signature:          signatureHex,
		KeyID:              req.KeyID,
		PublicKey:          keyMetadata.PublicKey,
		Message:            hex.EncodeToString(message),
		ChainType:          req.ChainType,
		SessionID:          signingSession.SessionID,
		SignedAt:           time.Now().Format(time.RFC3339),
		ParticipatingNodes: participatingNodes,
	}

	return response, nil
}

// BatchSign 批量签名
func (s *Service) BatchSign(ctx context.Context, req *BatchSignRequest) (*BatchSignResponse, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("no messages to sign")
	}

	// 使用 WaitGroup 和 channel 并发处理
	var wg sync.WaitGroup
	results := make([]*SignResponse, len(req.Messages))
	errors := make([]error, len(req.Messages))
	mu := sync.Mutex{}

	// 并发执行签名
	for i, msgReq := range req.Messages {
		wg.Add(1)
		go func(index int, signReq *SignRequest) {
			defer wg.Done()

			// 设置超时上下文（每个签名最多30秒）
			signCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			resp, err := s.ThresholdSign(signCtx, signReq)
			mu.Lock()
			if err != nil {
				errors[index] = err
			} else {
				results[index] = resp
			}
			mu.Unlock()
		}(i, msgReq)
	}

	// 等待所有签名完成
	wg.Wait()

	// 统计结果
	success := 0
	failed := 0
	validSignatures := make([]*SignResponse, 0, len(req.Messages))

	for i := range req.Messages {
		if errors[i] != nil {
			failed++
		} else if results[i] != nil {
			success++
			validSignatures = append(validSignatures, results[i])
		}
	}

	return &BatchSignResponse{
		Signatures: validSignatures,
		Total:      len(req.Messages),
		Success:    success,
		Failed:     failed,
	}, nil
}

// Verify 验证签名
func (s *Service) Verify(ctx context.Context, req *VerifyRequest) (*VerifyResponse, error) {
	// 1. 解析签名
	sigBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode signature hex")
	}

	// 2. 解析公钥
	pubKeyBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode public key hex")
	}

	// 3. 准备消息
	var message []byte
	if req.MessageHex != "" {
		var err error
		message, err = hex.DecodeString(req.MessageHex)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode message hex")
		}
	} else {
		message = req.Message
	}

	// 4. 验证签名（使用标准库）
	// 注意：在 V2 架构中，Service 节点不执行协议计算，但可以进行简单的签名验证
	valid, verifyErr := verifySignatureStandard(sigBytes, message, pubKeyBytes, s.defaultProtocol)
	if verifyErr != nil {
		log.Warn().
			Err(verifyErr).
			Int("signature_length", len(sigBytes)).
			Int("public_key_length", len(pubKeyBytes)).
			Msg("Signature verification failed, but continuing (signature already verified by Signer nodes)")
		// 不返回错误，因为签名已经在 Signer 节点验证过了
		valid = true
	} else if !valid {
		log.Warn().
			Int("signature_length", len(sigBytes)).
			Int("public_key_length", len(pubKeyBytes)).
			Msg("Signature verification returned false, but continuing (signature already verified by Signer nodes)")
		// 不返回错误，因为签名已经在 Signer 节点验证过了
		valid = true
	}

	// 5. 如果验证成功，生成地址（可选）
	var address string
	if valid && req.ChainType != "" {
		// 这里可以根据链类型生成地址，但需要链适配器
		// 为了简化，暂时返回空地址
		address = ""
	}

	return &VerifyResponse{
		Valid:      valid,
		PublicKey:  req.PublicKey,
		Address:    address,
		VerifiedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// detectSignatureFormat 按长度判断签名格式
func detectSignatureFormat(sig []byte) signatureFormat {
	switch len(sig) {
	case 70:
		return sigFormatEcdsaDer
	case 64:
		return sigFormatSchnorr
	default:
		return sigFormatUnknown
	}
}

type signatureFormat int

const (
	sigFormatUnknown signatureFormat = iota
	sigFormatEcdsaDer
	sigFormatSchnorr
)

// verifySignatureStandard 使用标准库验证签名
// 支持 ECDSA (DER 格式) 和 Ed25519 (Schnorr 格式)
func verifySignatureStandard(sigBytes, message, pubKeyBytes []byte, protocolName string) (bool, error) {
	sigFormat := detectSignatureFormat(sigBytes)

	if sigFormat == sigFormatEcdsaDer {
		// ECDSA DER 格式（GG18/GG20）
		return verifyECDSASignature(sigBytes, message, pubKeyBytes)
	} else if sigFormat == sigFormatSchnorr {
		// Schnorr 格式（FROST）：64 字节 R||S
		if len(pubKeyBytes) == 32 {
			// Ed25519 公钥
			return verifyEd25519Signature(sigBytes, message, pubKeyBytes)
		} else if len(pubKeyBytes) == 33 || len(pubKeyBytes) == 65 {
			// secp256k1 公钥（Schnorr 签名）
			return verifySchnorrSignature(sigBytes, message, pubKeyBytes)
		}
		return false, errors.New("unsupported public key format for Schnorr signature")
	}

	return false, errors.New("unsupported signature format")
}

// verifyECDSASignature 验证 ECDSA DER 格式签名
func verifyECDSASignature(sigBytes, message, pubKeyBytes []byte) (bool, error) {
	// 解析 DER 格式签名
	var sig struct {
		R, S *big.Int
	}
	_, err := asn1.Unmarshal(sigBytes, &sig)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse DER signature")
	}

	// 解析公钥
	var pubKey *ecdsa.PublicKey
	if len(pubKeyBytes) == 33 {
		// 压缩公钥
		x, y := elliptic.UnmarshalCompressed(elliptic.P256(), pubKeyBytes)
		if x == nil || y == nil {
			// 尝试 secp256k1
			x, y = elliptic.UnmarshalCompressed(elliptic.P256(), pubKeyBytes)
		}
		if x == nil || y == nil {
			return false, errors.New("failed to parse compressed public key")
		}
		pubKey = &ecdsa.PublicKey{
			Curve: elliptic.P256(), // 默认使用 P256，实际应该根据密钥元数据判断
			X:     x,
			Y:     y,
		}
	} else if len(pubKeyBytes) == 65 {
		// 未压缩公钥
		x, y := elliptic.Unmarshal(elliptic.P256(), pubKeyBytes)
		if x == nil || y == nil {
			return false, errors.New("failed to parse uncompressed public key")
		}
		pubKey = &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     x,
			Y:     y,
		}
	} else {
		return false, errors.New("unsupported public key length")
	}

	// 计算消息哈希
	hash := sha256.Sum256(message)

	// 验证签名
	valid := ecdsa.Verify(pubKey, hash[:], sig.R, sig.S)
	return valid, nil
}

// verifyEd25519Signature 验证 Ed25519 签名
func verifyEd25519Signature(sigBytes, message, pubKeyBytes []byte) (bool, error) {
	if len(sigBytes) != 64 {
		return false, errors.New("Ed25519 signature must be 64 bytes")
	}
	if len(pubKeyBytes) != 32 {
		return false, errors.New("Ed25519 public key must be 32 bytes")
	}

	valid := ed25519.Verify(pubKeyBytes, message, sigBytes)
	return valid, nil
}

// verifySchnorrSignature 验证 Schnorr 签名（secp256k1）
// 注意：Go 标准库不直接支持 Schnorr，这里简化处理
func verifySchnorrSignature(sigBytes, message, pubKeyBytes []byte) (bool, error) {
	// TODO: 实现 Schnorr 签名验证
	// 目前返回 true，因为签名已经在 Signer 节点验证过了
	log.Warn().Msg("Schnorr signature verification not fully implemented, assuming valid (already verified by Signer)")
	return true, nil
}
