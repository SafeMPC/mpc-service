package webauthn

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-openapi/swag"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/auth"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/labstack/echo/v4"
)

func PostWebAuthnLoginFinishRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.POST("/webauthn/login/finish", postWebAuthnLoginFinishHandler(s))
}

func postWebAuthnLoginFinishHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		var body types.PostWebAuthnLoginFinishPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		// 解析前端返回的 assertion response
		// 前端返回的是 navigator.credentials.get() 的结果，需要转换为 protocol.ParsedCredentialAssertionData
		responseBytes, err := json.Marshal(body.Response)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal assertion response")
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid assertion response format")
		}

		// 使用 go-webauthn 的解析函数
		assertionResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(responseBytes))
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse credential assertion response")
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid assertion response format: "+err.Error())
		}

		// 调用 WebAuthn Service 完成登录
		err = s.WebAuthnService.FinishLogin(
			ctx,
			swag.StringValue(body.UserID),
			swag.StringValue(body.SessionData),
			assertionResponse,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to finish WebAuthn login")
			return echo.NewHTTPError(http.StatusUnauthorized, "Authentication failed: "+err.Error())
		}

		// 生成 JWT Token
		// 使用 Auth Service 的 JWT Manager（如果可用）
		// 或者直接使用 auth.JWTManager
		userID := swag.StringValue(body.UserID)
		token, err := generateJWTToken(s, userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate JWT token")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate token")
		}

		// 返回响应
		response := types.WebAuthnLoginResponse{
			Success:      swag.Bool(true),
			AccessToken:  swag.String(token),
			ExpiresIn:    3600, // 1 hour
		}

		return util.ValidateAndReturn(c, http.StatusOK, &response)
	}
}

// generateJWTToken 生成 JWT Token
func generateJWTToken(s *api.Server, userID string) (string, error) {
	// 从 config 获取 JWT 配置
	secretKey := s.Config.MPC.JWTSecret
	if secretKey == "" {
		secretKey = "default-secret-key-change-in-production" // 默认值，生产环境必须配置
	}
	
	issuer := "safempc" // 默认 issuer
	if s.Config.MPC.JWTIssuer != "" {
		issuer = s.Config.MPC.JWTIssuer
	}
	
	// 创建 JWT Manager
	jwtManager := auth.NewJWTManager(secretKey, issuer, time.Hour)
	
	// 生成 token，使用 userID 作为 appID
	token, err := jwtManager.Generate(userID, "", []string{})
	if err != nil {
		return "", err
	}
	
	return token, nil
}
