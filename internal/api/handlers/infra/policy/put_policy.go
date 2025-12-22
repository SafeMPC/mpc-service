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

// PutSigningPolicyRoute 注册路由
func PutSigningPolicyRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Infra.PUT("/wallets/:walletId/policy", putSigningPolicyHandler(s))
}

func putSigningPolicyHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		walletID := c.Param("walletId")
		var body infrastructure.SetSigningPolicyPayload
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

		req := &pb.SetSigningPolicyRequest{
			KeyId:         walletID,
			PolicyType:    body.PolicyType,
			MinSignatures: body.MinSignatures,
			AdminAuths:    adminAuths,
		}

		var resp *pb.SetSigningPolicyResponse
		var err error

		if s.MPCGRPCServer != nil {
			resp, err = s.MPCGRPCServer.SetSigningPolicy(ctx, req)
		} else if s.MPCGRPCClient != nil {
			resp, err = s.MPCGRPCClient.SetSigningPolicy(ctx, "coordinator", req)
		} else {
			log.Error().Msg("No MPC gRPC Server or Client available")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Internal server error")
		}

		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to set signing policy")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to set signing policy")
		}

		if !resp.Success {
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, resp.Message)
		}

		return util.ValidateAndReturn(c, http.StatusOK, &infrastructure.SigningPolicyResponse{
			Success:       true,
			Message:       resp.Message,
			KeyID:         walletID,
			PolicyType:    body.PolicyType,
			MinSignatures: body.MinSignatures,
		})
	}
}
