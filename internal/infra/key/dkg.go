package key

import (
	"context"
	"strings"
	"time"

	"github.com/SafeMPC/mpc-service/internal/infra/storage"
	"github.com/SafeMPC/mpc-service/internal/mpc/node"
	pb "github.com/SafeMPC/mpc-service/pb/mpc/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// inferProtocolForDKG 根据算法和曲线推断DKG应该使用的协议
// ECDSA + secp256k1 -> GG20 (默认) 或 GG18
// EdDSA/Schnorr + ed25519/secp256k1 -> FROST
func inferProtocolForDKG(algorithm, curve string) string {
	algorithmLower := strings.ToLower(algorithm)
	curveLower := strings.ToLower(curve)

	// FROST 协议：EdDSA 或 Schnorr + Ed25519 或 secp256k1
	if algorithmLower == "eddsa" || algorithmLower == "schnorr" {
		if curveLower == "ed25519" || curveLower == "secp256k1" {
			return "frost"
		}
	}

	// ECDSA + secp256k1：使用 GG20（默认）或 GG18
	if algorithmLower == "ecdsa" {
		if curveLower == "secp256k1" || curveLower == "secp256r1" {
			return "gg20" // 默认使用 GG20
		}
	}

	// 默认使用 GG20
	return "gg20"
}

// DKGService 分布式密钥生成服务
// 注意：在 V2 架构中，Service 节点不执行协议计算，只负责协调
type DKGService struct {
	metadataStore   storage.MetadataStore
	keyShareStorage storage.KeyShareStorage
	nodeManager     *node.Manager
	nodeDiscovery   *node.Discovery
	grpcClient      dkgGRPCClient
	// 同步模式配置：最大等待时间、轮询间隔
	MaxWaitTime  time.Duration
	PollInterval time.Duration
}

// dkgGRPCClient 最小化的 gRPC 客户端接口
type dkgGRPCClient interface {
	SendStartDKG(ctx context.Context, nodeID string, req *pb.StartDKGRequest) (*pb.StartDKGResponse, error)
}

// NewDKGService 创建DKG服务
// 注意：在 V2 架构中，Service 节点不执行协议计算，只负责协调
func NewDKGService(
	metadataStore storage.MetadataStore,
	keyShareStorage storage.KeyShareStorage,
	nodeManager *node.Manager,
	nodeDiscovery *node.Discovery,
	grpcClient dkgGRPCClient,
) *DKGService {
	return &DKGService{
		metadataStore:   metadataStore,
		keyShareStorage: keyShareStorage,
		nodeManager:     nodeManager,
		nodeDiscovery:   nodeDiscovery,
		grpcClient:      grpcClient,
		// 缩短同步等待时间，加快失败检测
		MaxWaitTime:  2 * time.Minute,
		PollInterval: 2 * time.Second,
	}
}

// ExecuteDKG 执行 DKG（分布式密钥生成）
// 注意：在 V2 架构中，Service 节点不执行协议计算，只负责协调
// 返回一个简化的响应，包含公钥信息
func (s *DKGService) ExecuteDKG(ctx context.Context, keyID string, req *CreateKeyRequest) (interface{}, error) {
	log.Info().
		Str("key_id", keyID).
		Str("algorithm", req.Algorithm).
		Str("curve", req.Curve).
		Int("threshold", req.Threshold).
		Int("total_nodes", req.TotalNodes).
		Msg("ExecuteDKG: Starting synchronous DKG execution")

	var nodeIDs []string

	// 优先从 DKG 会话中获取参与节点列表（由 coordinator 决定参与者）
	// 这样可以避免在 coordinator 上执行本地 DKG，真正的 DKG 只在 participant 节点上运行
	session, err := s.metadataStore.GetSigningSession(ctx, keyID)
	if err == nil && len(session.ParticipatingNodes) > 0 {
		nodeIDs = session.ParticipatingNodes

		log.Info().
			Str("key_id", keyID).
			Strs("node_ids", nodeIDs).
			Int("node_count", len(nodeIDs)).
			Msg("ExecuteDKG: Using node IDs from DKG session")
	} else {
		// 支持 2-of-2 模式：手机 P1 + 服务器 P2
		if req.Threshold == 2 && req.TotalNodes == 2 {
			if req.MobileNodeID == "" {
				return nil, errors.New("mobile node ID is required for 2-of-2 DKG")
			}
			// 2-of-2 模式：手机节点 P1 + 服务器 Signer P2
			nodeIDs = []string{req.MobileNodeID, "server-signer-p2"}

			log.Info().
				Str("key_id", keyID).
				Strs("node_ids", nodeIDs).
				Int("node_count", len(nodeIDs)).
				Str("mobile_node_id", req.MobileNodeID).
				Msg("ExecuteDKG: Using 2-of-2 mode (mobile signer + server signer)")
		} else if req.Threshold == 2 && req.TotalNodes == 3 {
			// 保持向后兼容：2-of-3 模式
			nodeIDs = []string{"server-proxy-1", "server-proxy-2", "server-backup-1"}

			log.Info().
				Str("key_id", keyID).
				Strs("node_ids", nodeIDs).
				Int("node_count", len(nodeIDs)).
				Msg("ExecuteDKG: Using fallback fixed 2-of-3 node list")
		} else {
			if err != nil {
				return nil, errors.Wrap(err, "failed to get DKG session")
			}
			return nil, errors.New("no participating nodes in DKG session")
		}
	}

	if len(nodeIDs) < req.Threshold {
		return nil, errors.Errorf("insufficient participating nodes: need at least %d, have %d", req.Threshold, len(nodeIDs))
	}

	// 3. Service 节点不执行协议计算，只负责协调
	// 通过 gRPC 调用 Signer 节点执行 DKG
	if s.grpcClient != nil {
		// 确保会话已创建
		session := &storage.SigningSession{
			SessionID:          keyID,
			KeyID:              keyID,
			Protocol:           inferProtocolForDKG(req.Algorithm, req.Curve),
			Status:             "pending",
			Threshold:          req.Threshold,
			TotalNodes:         req.TotalNodes,
			ParticipatingNodes: nodeIDs,
			CreatedAt:          time.Now(),
		}
		// 尝试保存会话，如果已存在则忽略错误（或者是更新？）
		// 这里简化处理，直接保存，覆盖旧的 pending 会话
		if err := s.metadataStore.SaveSigningSession(ctx, session); err != nil {
			log.Warn().Err(err).Msg("Failed to save DKG session, it might already exist")
		} else {
			log.Info().Str("session_id", keyID).Msg("DKG session created by coordinator")
		}

		leaderNodeID := nodeIDs[0]
		startReq := &pb.StartDKGRequest{
			SessionId:  keyID,
			KeyId:      keyID,
			Algorithm:  req.Algorithm,
			Curve:      req.Curve,
			Threshold:  int32(req.Threshold),
			TotalNodes: int32(req.TotalNodes),
			NodeIds:    nodeIDs,
		}
		startCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		_, _ = s.grpcClient.SendStartDKG(startCtx, leaderNodeID, startReq)
		cancel()

		deadline := time.Now().Add(s.MaxWaitTime)
		for time.Now().Before(deadline) {
			sess, gErr := s.metadataStore.GetSigningSession(ctx, keyID)
			if gErr == nil {
				if strings.EqualFold(sess.Status, "completed") || strings.EqualFold(sess.Status, "success") {
					pubHex := sess.Signature
					if pubHex == "" {
						return nil, errors.New("DKG completed but public key missing in session")
					}
					// 返回简化的响应结构
					return map[string]interface{}{
						"public_key": pubHex,
						"key_id":     keyID,
					}, nil
				}
				if strings.EqualFold(sess.Status, "failed") {
					return nil, errors.Errorf("dkg session %s failed", keyID)
				}
			}
			time.Sleep(s.PollInterval)
		}
		return nil, errors.Errorf("dkg session %s timeout (waited %s)", keyID, s.MaxWaitTime)
	}

	// Service 节点不应该执行本地 DKG 协议计算
	// 如果没有 gRPC 客户端，返回错误
	return nil, errors.New("Service node cannot execute DKG protocol locally. DKG must be executed on Signer nodes via gRPC")
}

// DistributeKeyShares 分发密钥分片到各个节点
// 注意：在 V2 架构中，Service 节点不存储密钥分片
func (s *DKGService) DistributeKeyShares(ctx context.Context, keyID string, keyShares map[string][]byte) error {
	// 加密并分发密钥分片到各个节点
	for nodeID, share := range keyShares {
		// 存储密钥分片（内部会加密）
		if err := s.keyShareStorage.StoreKeyShare(ctx, keyID, nodeID, share); err != nil {
			return errors.Wrapf(err, "failed to store key share for node %s", nodeID)
		}
	}

	return nil
}

// RecoverKeyShare 恢复密钥分片（阈值恢复）
// 注意：在 V2 架构中，Service 节点不支持密钥恢复功能
func (s *DKGService) RecoverKeyShare(ctx context.Context, keyID string, nodeIDs []string, threshold int) ([]byte, error) {
	return nil, errors.New("key share recovery is not supported in Service node. This feature should be implemented in Signer nodes")
}

// ValidateKeyShares 验证密钥分片一致性
// 注意：在 V2 架构中，Service 节点不支持密钥分片验证功能
func (s *DKGService) ValidateKeyShares(ctx context.Context, keyID string, publicKeyHex string) error {
	// Service 节点不执行协议计算，不验证密钥分片
	// 验证应该在 Signer 节点完成
	return nil
}

// ExecuteResharing 执行密钥轮换（Resharing）
// 注意：在 V2 架构中，Service 节点不支持密钥轮换功能
// 密钥轮换应该在 Signer 节点完成
func (s *DKGService) ExecuteResharing(
	ctx context.Context,
	keyID string,
	oldNodeIDs []string,
	newNodeIDs []string,
	oldThreshold int,
	newThreshold int,
) (interface{}, error) {
	return nil, errors.New("key rotation (Resharing) is not supported in Service node. This feature should be implemented in Signer nodes")
}

// RotateKey 密钥轮换
// 注意：在 V2 架构中，Service 节点不支持密钥轮换功能
func (s *DKGService) RotateKey(ctx context.Context, keyID string) error {
	return errors.New("key rotation is not supported in Service node. This feature should be implemented in Signer nodes")
	// 4. 更新密钥元数据

	return errors.New("key rotation not yet implemented")
}

// retryProtocol 重试协议执行
func (s *DKGService) retryProtocol(ctx context.Context, opName string, fn func() error) error {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			log.Warn().Str("operation", opName).Int("attempt", i+1).Msg("Retrying operation after error")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second * time.Duration(i)):
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查错误类型（不再使用 protocol.ProtocolError）
		// 如果是致命错误，不重试
		if strings.Contains(err.Error(), "malicious") || strings.Contains(err.Error(), "fatal") {
			// 恶意节点，立即停止并记录
			log.Error().Msg("Malicious nodes detected, aborting")
			return err
		}
		// 如果是超时或网络错误，尝试重试
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network") {
			continue
		}

		// 其他错误，直接返回
		return err
	}
	return lastErr
}
