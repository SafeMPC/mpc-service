package webauthn

import (
	"net/http"

	"github.com/go-openapi/swag"
	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/labstack/echo/v4"
)

func PostWebAuthnRegisterBeginRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.POST("/webauthn/register/begin", postWebAuthnRegisterBeginHandler(s))
}

func postWebAuthnRegisterBeginHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		var body types.PostWebAuthnRegisterBeginPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		// 调用 WebAuthn Service
		options, sessionData, err := s.WebAuthnService.BeginRegistration(
			ctx,
			swag.StringValue(body.UserID),
			swag.StringValue(body.UserName),
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to begin WebAuthn registration")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to begin registration")
		}

		// 返回响应
		response := types.WebAuthnRegisterBeginResponse{
			Options:     options,
			SessionData: swag.String(sessionData),
		}

		return util.ValidateAndReturn(c, http.StatusOK, &response)
	}
}
