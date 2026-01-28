package webauthn

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-openapi/swag"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/labstack/echo/v4"
)

func PostWebAuthnRegisterFinishRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.POST("/webauthn/register/finish", postWebAuthnRegisterFinishHandler(s))
}

func postWebAuthnRegisterFinishHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		var body types.PostWebAuthnRegisterFinishPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		// 解析前端返回的 credential response
		// 前端返回的是 navigator.credentials.create() 的结果，需要转换为 protocol.ParsedCredentialCreationData
		// 先转换为 map[string]interface{}，然后使用 protocol.ParseCredentialCreationResponseBody
		responseBytes, err := json.Marshal(body.Response)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal credential response")
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid credential response format")
		}

		// 使用 go-webauthn 的解析函数
		credentialResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(responseBytes))
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse credential creation response")
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid credential response format: "+err.Error())
		}

		// 调用 WebAuthn Service 完成注册
		err = s.WebAuthnService.FinishRegistration(
			ctx,
			swag.StringValue(body.UserID),
			swag.StringValue(body.UserName),
			swag.StringValue(body.SessionData),
			credentialResponse,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to finish WebAuthn registration")
			return echo.NewHTTPError(http.StatusBadRequest, "Failed to complete registration: "+err.Error())
		}

		// 生成 JWT Token（注册成功后自动登录）
		userID := swag.StringValue(body.UserID)
		token, err := generateJWTToken(s, userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate JWT token")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate token")
		}

		// 返回成功响应（包含 JWT Token）
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":      true,
			"message":      "Passkey registered successfully",
			"access_token": token,
			"expires_in":   3600, // 1 hour
		})
	}
}
