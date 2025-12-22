package policy

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

// GetSigningPolicyRoute 注册路由
func GetSigningPolicyRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Infra.GET("/wallets/:walletId/policy", getSigningPolicyHandler(s))
}

func getSigningPolicyHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		walletID := c.Param("walletId")

		req := &pb.GetSigningPolicyRequest{
			KeyId: walletID,
		}

		var resp *pb.GetSigningPolicyResponse
		var err error

		if s.MPCGRPCServer != nil {
			resp, err = s.MPCGRPCServer.GetSigningPolicy(ctx, req)
		} else if s.MPCGRPCClient != nil {
			resp, err = s.MPCGRPCClient.GetSigningPolicy(ctx, "coordinator", req)
		} else {
			log.Error().Msg("No MPC gRPC Server or Client available")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Internal server error")
		}

		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to get signing policy")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to get signing policy")
		}

		if resp == nil {
			return httperrors.NewHTTPError(http.StatusNotFound, types.PublicHTTPErrorTypeGeneric, "Signing policy not found")
		}

		return util.ValidateAndReturn(c, http.StatusOK, &infrastructure.SigningPolicyResponse{
			Success:       true,
			KeyID:         resp.KeyId,
			PolicyType:    resp.PolicyType,
			MinSignatures: resp.MinSignatures,
		})
	}
}
