package wallets

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/api/httperrors"
	"github.com/SafeMPC/mpc-service/internal/auth"
	"github.com/SafeMPC/mpc-service/internal/infra/key"
	"github.com/SafeMPC/mpc-service/internal/infra/service"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func PostCreateWalletRoute(s *api.Server) *echo.Route {
	// 使用 /api/v1/auth/wallets 路径（在 APIV1Auth 组下）
	return s.Router.APIV1Auth.POST("/wallets", postCreateWalletHandler(s))
}

func postCreateWalletHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		var body types.PostCreateWalletPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		// TODO: JWT Token 验证（从请求头获取）
		// 这里暂时跳过，等待 JWT 中间件实现

		// WebAuthn 二次验证（测试环境暂时允许跳过）
		// TODO: 生产环境必须要求 WebAuthn Assertion
		if body.WebauthnAssertion != nil {
			credentialID := body.WebauthnAssertion.CredentialID
			if credentialID == nil {
				return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, "credential_id is required")
			}
			// TODO: 验证 WebAuthn Assertion
			// 需要从 JWT Token 中获取 userID，然后验证 assertion
			_ = credentialID // 暂时未使用
		} else {
			log.Warn().Msg("WebAuthn assertion not provided - skipping validation for testing")
		}

		// 生成钱包 ID（使用 UUID）
		walletID := uuid.New().String()
		keyID := walletID // 钱包 ID 等于密钥 ID

		// 推断协议
		algorithm := swag.StringValue(body.Algorithm)
		curve := swag.StringValue(body.Curve)
		protocol := inferProtocol(algorithm, curve)
		chainType := swag.StringValue(body.ChainType)

		// 2-of-2 模式：需要提供 mobile node ID
		mobileNodeID := body.MobileNodeID
		if mobileNodeID == "" {
			mobileNodeID = "mobile-p1" // 默认值（向后兼容）
			log.Warn().Msg("mobile_node_id not provided in request, using default 'mobile-p1'")
		}

		// 先创建 key 占位符（Pending 状态），以满足 DKG Session 的外键约束
		keyReq := &key.CreateKeyRequest{
			KeyID:        keyID,
			Algorithm:    algorithm,
			Curve:        curve,
			Threshold:    2,
			TotalNodes:   2,
			ChainType:    chainType,
			MobileNodeID: mobileNodeID,
		}

		// 创建 pending key（不执行 DKG，只创建占位符）
		if err := s.KeyService.CreateKeyPlaceholder(ctx, keyReq); err != nil {
			log.Error().Err(err).Msg("Failed to create key placeholder")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to create wallet: "+err.Error())
		}

		// 创建 DKG 会话请求
		dkgReq := &service.CreateDKGSessionRequest{
			KeyID:        keyID,
			Algorithm:    algorithm,
			Curve:        curve,
			Protocol:     protocol,
			Threshold:    2,
			TotalNodes:   2,
			MobileNodeID: mobileNodeID,
		}

		// 调用 MPC Service 创建 DKG 会话
		dkgSession, err := s.MPCService.CreateDKGSession(ctx, dkgReq)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create DKG session")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to create wallet: "+err.Error())
		}

		// 生成临时 JWT Token
		// TODO: 在生产环境中，应该使用真实的认证 Token
		secretKey := s.Config.MPC.JWTSecret
		if secretKey == "" {
			secretKey = "default-secret-key-change-in-production"
		}
		issuer := "safempc"
		if s.Config.MPC.JWTIssuer != "" {
			issuer = s.Config.MPC.JWTIssuer
		}
		// 使用 mobileNodeID 作为 userID/AppID
		tokenDuration := s.Config.MPC.JWTDuration
		if tokenDuration <= 0 {
			tokenDuration = 24 * time.Hour
		}
		jwtManager := auth.NewJWTManager(secretKey, issuer, tokenDuration)
		token, err := jwtManager.Generate(mobileNodeID, "default-tenant", nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate temp JWT token")
			// 降级：使用空 token，并在 handler 中放行（如果配置允许）或者客户端需要处理
			token = "temp-token"
		}

		// 发现可用的 Signer 节点 (V3)
		// 从 Service Discovery 中获取活跃的 Signer 列表
		// TODO: 目前简单硬编码或查询 Discovery Service
		// 理想情况下，MPCService.CreateDKGSession 应该已经分配了参与的 Signer
		// 我们假设 dkgSession 中包含了参与者信息

		// 临时：查询 Discovery Service 获取所有 Signer
		var signerEndpoints []string
		if s.DiscoveryService != nil {
			signers, err := s.DiscoveryService.DiscoverSigners(ctx, 1) // 至少需要 1 个 Signer (2-of-2: 1 mobile + 1 signer)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to discover signers, using fallback")
			} else {
				for _, signer := range signers {
					// 构造 endpoint
					endpoint := fmt.Sprintf("%s:%d", signer.Address, signer.Port)
					signerEndpoints = append(signerEndpoints, endpoint)
				}
			}
		}

		if len(signerEndpoints) == 0 {
			host := c.Request().Host
			if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
				host = h
			} else if idx := strings.IndexByte(host, ':'); idx >= 0 {
				host = host[:idx]
			}
			if host == "" {
				host = "localhost"
			}
			signerEndpoints = []string{net.JoinHostPort(host, strconv.Itoa(9091))}
		}

		log.Info().
			Str("session_id", dkgSession.SessionID).
			Str("mobile_node_id", mobileNodeID).
			Strs("signer_endpoints", signerEndpoints).
			Msg("Created DKG session with mobile node ID")

		// 返回响应
		walletIDUUID := strfmt.UUID(walletID)
		dkgSessionIDUUID := strfmt.UUID(dkgSession.SessionID)
		status := dkgSession.Status
		response := &types.CreateWalletResponse{
			WalletID:        &walletIDUUID,
			DkgSessionID:    &dkgSessionIDUUID,
			Status:          &status,
			SignerEndpoints: signerEndpoints,
			SessionToken:    token,
		}

		return util.ValidateAndReturn(c, http.StatusCreated, response)
	}
}

// inferProtocol 根据算法和曲线推断协议
func inferProtocol(algorithm, curve string) string {
	algo := swag.String(algorithm)
	curveStr := swag.String(curve)

	if *algo == "EdDSA" || *algo == "Schnorr" {
		if *curveStr == "ed25519" || *curveStr == "secp256k1" {
			return "frost"
		}
	}

	if *algo == "ECDSA" {
		if *curveStr == "secp256k1" || *curveStr == "secp256r1" {
			return "gg20"
		}
	}

	// 默认使用 GG20
	return "gg20"
}
