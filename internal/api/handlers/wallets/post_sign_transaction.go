package wallets

import (
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/api/httperrors"
	"github.com/SafeMPC/mpc-service/internal/auth"
	"github.com/SafeMPC/mpc-service/internal/mpc/node"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/types/wallets"
	"github.com/SafeMPC/mpc-service/internal/util"
	pb "github.com/SafeMPC/mpc-service/pb/mpc/v1"
	"github.com/go-openapi/strfmt"
	"github.com/labstack/echo/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PostSignTransactionRoute 注册交易签名路由
func PostSignTransactionRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.POST("/wallets/:walletId/sign", postSignTransactionHandler(s))
}

// postSignTransactionHandler 签名交易
func postSignTransactionHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		// TODO: JWT Token 验证（从请求头获取）

		// 使用统一的参数绑定方式
		var params wallets.PostSignTransactionParams
		if err := util.BindAndValidatePathParams(c, &params); err != nil {
			return err
		}

		// 绑定请求体
		var body types.PostSignTransactionPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		walletID := params.WalletID

		// WebAuthn 二次验证（测试环境暂时允许跳过）
		// TODO: 生产环境必须要求 WebAuthn Assertion
		if body.WebauthnAssertion != nil {
			// TODO: 验证 WebAuthn Assertion
			// 需要从 JWT Token 中获取 userID，然后验证 assertion
		} else {
			log.Warn().Msg("WebAuthn assertion not provided - skipping validation for testing")
		}

		// 查询密钥信息
		keyMetadata, err := s.KeyService.GetKey(ctx, walletID)
		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to get key")
			return httperrors.NewHTTPError(http.StatusNotFound, types.PublicHTTPErrorTypeGeneric, "Wallet not found")
		}

		// 解码消息
		messageHex := ""
		if body.MessageHex != nil {
			messageHex = *body.MessageHex
		}
		if messageHex == "" {
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, "message_hex is required")
		}

		// 移除 "0x" 前缀（如果有）
		if len(messageHex) > 2 && messageHex[:2] == "0x" {
			messageHex = messageHex[2:]
		}

		_, err = hex.DecodeString(messageHex)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode message_hex")
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, "Invalid message_hex format")
		}

		// 推断协议
		protocol := inferProtocol(keyMetadata.Algorithm, keyMetadata.Curve)

		// 创建签名会话
		// 使用 SigningService 创建签名会话
		signingSession, err := s.SigningService.CreateSigningSession(ctx, walletID, protocol)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create signing session")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to create signing session: "+err.Error())
		}

		nodes, err := s.NodeDiscovery.DiscoverNodes(ctx, node.NodeTypeSigner, node.NodeStatusActive, signingSession.TotalNodes)
		if err != nil {
			log.Error().Err(err).Msg("Failed to discover signer nodes")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to discover signer nodes: "+err.Error())
		}

		signerEndpoints := make([]string, 0, len(nodes))
		for _, n := range nodes {
			if n.Endpoint != "" {
				signerEndpoints = append(signerEndpoints, n.Endpoint)
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
			signerEndpoints = append(signerEndpoints, net.JoinHostPort(host, strconv.Itoa(9091)))
		}

		sessionToken := ""
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" {
			sessionToken = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}

		secretKey := s.Config.MPC.JWTSecret
		if secretKey == "" {
			secretKey = "default-secret-key-change-in-production"
		}

		var mobileNodeID string
		if sessionToken != "" {
			log.Info().Int("session_token_len", len(sessionToken)).Msg("Parsed session token from Authorization header")
			jwtManager := auth.NewJWTManager(secretKey, "", 0)
			claims, err := jwtManager.Validate(sessionToken)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to validate session token as JWT; will try parsing without claims validation")
				claims, err = jwtManager.ParseWithoutClaimsValidation(sessionToken)
				if err != nil {
					log.Warn().Err(err).Msg("Failed to parse session token as JWT; cannot derive mobile node id")
					claims = nil
				}
			}
			if claims != nil {
				if claims.Subject != "" {
					mobileNodeID = claims.Subject
				} else if claims.AppID != "" {
					mobileNodeID = claims.AppID
				}
			}
		}

		if mobileNodeID == "" {
			log.Warn().Msg("Unable to resolve mobile node id from token; StartSign will be skipped and direct-connect signing may stall")
		} else {
			issuer := "safempc"
			if s.Config.MPC.JWTIssuer != "" {
				issuer = s.Config.MPC.JWTIssuer
			}
			tokenDuration := s.Config.MPC.JWTDuration
			if tokenDuration <= 0 {
				tokenDuration = 24 * time.Hour
			}
			jwtManager := auth.NewJWTManager(secretKey, issuer, tokenDuration)
			if refreshedToken, err := jwtManager.Generate(mobileNodeID, "default-tenant", nil); err == nil {
				sessionToken = refreshedToken
			} else {
				log.Warn().Err(err).Msg("Failed to refresh session token for signer gRPC")
			}

			nodeIDs := make([]string, 0, signingSession.TotalNodes)
			nodeIDs = append(nodeIDs, mobileNodeID)
			for _, n := range nodes {
				if n.NodeID != "" && len(nodeIDs) < signingSession.TotalNodes {
					nodeIDs = append(nodeIDs, n.NodeID)
				}
			}

			msgBytes, _ := hex.DecodeString(messageHex)
			startReq := &pb.StartSignRequest{
				SessionId:       signingSession.SessionID,
				KeyId:           walletID,
				Message:         msgBytes,
				MessageHex:      messageHex,
				Protocol:        protocol,
				Threshold:       int32(signingSession.Threshold),
				TotalNodes:      int32(signingSession.TotalNodes),
				NodeIds:         nodeIDs,
				DerivationPath:  "",
				ParentChainCode: nil,
			}

			if s.MPCGRPCClient != nil {
				for _, n := range nodes {
					if n.NodeID == "" {
						continue
					}
					log.Info().Str("signer_node_id", n.NodeID).Str("session_id", signingSession.SessionID).Msg("Starting signer signing session via StartSign RPC")
					_, err := s.MPCGRPCClient.SendStartSign(ctx, n.NodeID, startReq)
					if err != nil {
						log.Error().Err(err).Str("signer_node_id", n.NodeID).Str("session_id", signingSession.SessionID).Msg("Failed to StartSign on signer")
					}
				}
			} else {
				log.Warn().Msg("MPCGRPCClient is nil; dialing signer endpoints directly for StartSign")
				for _, n := range nodes {
					if n.Endpoint == "" {
						continue
					}
					callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					conn, err := grpc.DialContext(callCtx, n.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
					cancel()
					if err != nil {
						log.Error().Err(err).Str("signer_endpoint", n.Endpoint).Str("session_id", signingSession.SessionID).Msg("Failed to dial signer for StartSign")
						continue
					}
					client := pb.NewSignerServiceClient(conn)
					log.Info().Str("signer_endpoint", n.Endpoint).Str("session_id", signingSession.SessionID).Msg("Starting signer signing session via StartSign RPC (direct dial)")
					_, err = client.StartSign(ctx, startReq)
					_ = conn.Close()
					if err != nil {
						log.Error().Err(err).Str("signer_endpoint", n.Endpoint).Str("session_id", signingSession.SessionID).Msg("Failed to StartSign on signer endpoint")
					}
				}
			}
		}

		sessionIDUUID := strfmt.UUID(signingSession.SessionID)
		status := signingSession.Status
		estimatedTime := "2s"
		response := &types.SignTransactionResponse{
			SessionID:       &sessionIDUUID,
			Status:          &status,
			SessionToken:    sessionToken,
			SignerEndpoints: signerEndpoints,
			EstimatedTime:   estimatedTime,
		}

		return util.ValidateAndReturn(c, http.StatusOK, response)
	}
}
