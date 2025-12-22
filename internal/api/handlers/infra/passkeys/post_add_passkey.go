package passkeys

import (
	"net/http"

	"github.com/go-openapi/swag"
	"github.com/kashguard/go-mpc-infra/internal/api"
	"github.com/kashguard/go-mpc-infra/internal/api/httperrors"
	"github.com/kashguard/go-mpc-infra/internal/types"
	"github.com/kashguard/go-mpc-infra/internal/util"
	"github.com/labstack/echo/v4"
)

func PostAddPasskeyRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Infra.POST("/passkeys", postAddPasskeyHandler(s))
}

func postAddPasskeyHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		var body types.PostAddPasskeyPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		if err := s.KeyService.AddPasskey(ctx, swag.StringValue(body.CredentialID), swag.StringValue(body.PublicKey), body.DeviceName); err != nil {
			log.Error().Err(err).Msg("Failed to add user passkey")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to add user passkey")
		}

		return util.ValidateAndReturn(c, http.StatusOK, &types.AddPasskeyResponse{
			Success: true,
			Message: "User passkey added successfully",
		})
	}
}
