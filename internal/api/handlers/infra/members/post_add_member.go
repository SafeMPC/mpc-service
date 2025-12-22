package members

import (
	"net/http"

	"github.com/kashguard/go-mpc-infra/internal/api"
	"github.com/kashguard/go-mpc-infra/internal/api/httperrors"
	"github.com/kashguard/go-mpc-infra/internal/types"
	"github.com/kashguard/go-mpc-infra/internal/types/infrastructure"
	"github.com/kashguard/go-mpc-infra/internal/util"
	pb "github.com/kashguard/go-mpc-infra/pb/mpc/v1"
	"github.com/labstack/echo/v4"
)

// PostAddWalletMemberRoute 注册路由
func PostAddWalletMemberRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Infra.POST("/wallets/:walletId/members", postAddWalletMemberHandler(s))
}

func postAddWalletMemberHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		walletID := c.Param("walletId")
		var body infrastructure.AddWalletMemberPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		// 转换 AdminAuths
		var adminAuths []*pb.AdminAuthToken
		for _, auth := range body.AdminAuths {
			adminAuths = append(adminAuths, &pb.AdminAuthToken{
				ReqId:             auth.ReqID,
				CredentialId:      auth.CredentialID,
				PasskeySignature:  auth.PasskeySignature,
				AuthenticatorData: auth.AuthenticatorData,
				ClientDataJson:    auth.ClientDataJSON,
			})
		}

		req := &pb.AddWalletMemberRequest{
			WalletId:     walletID,
			CredentialId: body.CredentialID,
			Role:         body.Role,
			AdminAuths:   adminAuths,
		}

		var resp *pb.AddWalletMemberResponse
		var err error

		// 优先使用本地 gRPC Server
		if s.MPCGRPCServer != nil {
			resp, err = s.MPCGRPCServer.AddWalletMember(ctx, req)
		} else if s.MPCGRPCClient != nil {
			// 如果本地没有 Server (纯 API 节点)，则发给 Coordinator
			// 注意：这里需要知道 Coordinator 的 NodeID
			// 简单起见，尝试发给 Coordinator (假设配置中知道或能发现)
			// 或者遍历所有节点直到成功？不，应该发给 Coordinator。
			// 这里假设 "coordinator" 是服务名或 ID
			// 如果不知道 Coordinator ID，这里会失败。
			// 更好的方式是 API 和 Coordinator 部署在一起。
			// 暂且尝试发给配置中的 NodeID (如果是 participant 也没办法，只能试)
			// 实际部署中 API 通常连着 DB，可以直接写，但这里复用 gRPC 逻辑。
			// 如果无法本地处理，报错暂不支持远程调用（除非有明确的 Coordinator 地址）
			// 但 GRPCClient 有 getOrCreateConnection，如果传入 "coordinator" 且在 Consul 注册了，可以通。
			resp, err = s.MPCGRPCClient.AddWalletMember(ctx, "coordinator", req)
		} else {
			log.Error().Msg("No MPC gRPC Server or Client available")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Internal server error")
		}

		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to add wallet member")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to add wallet member")
		}

		if !resp.Success {
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, resp.Message)
		}

		return util.ValidateAndReturn(c, http.StatusOK, &infrastructure.WalletMemberResponse{
			Success: true,
			Message: resp.Message,
		})
	}
}
