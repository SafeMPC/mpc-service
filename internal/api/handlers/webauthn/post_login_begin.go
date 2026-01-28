package webauthn

import (
	"net/http"

	"github.com/go-openapi/swag"
	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/labstack/echo/v4"
)

func PostWebAuthnLoginBeginRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.POST("/webauthn/login/begin", postWebAuthnLoginBeginHandler(s))
}

func postWebAuthnLoginBeginHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		var body types.PostWebAuthnLoginBeginPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		// 调用 WebAuthn Service
		options, sessionData, err := s.WebAuthnService.BeginLogin(
			ctx,
			swag.StringValue(body.UserID),
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to begin WebAuthn login")
			// 如果用户不存在或无凭证，返回 404
			if err.Error() == "no credentials found for user" {
				return echo.NewHTTPError(http.StatusNotFound, "User not found or no credentials")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to begin login")
		}

		// 返回响应
		response := types.WebAuthnLoginBeginResponse{
			Options:     options,
			SessionData: swag.String(sessionData),
		}

		return util.ValidateAndReturn(c, http.StatusOK, &response)
	}
}
