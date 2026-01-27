package sessions

import (
	"net/http"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/api/httperrors"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/types/sessions"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/labstack/echo/v4"
)

func GetSessionRoute(s *api.Server) *echo.Route {
	// 使用 /v1/sessions/{id} 路径（符合 API 定义）
	return s.Router.APIV1Auth.GET("/sessions/:session_id", getSessionHandler(s))
}

func getSessionHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		// 使用统一的参数绑定方式
		var params sessions.GetSessionParams
		if err := util.BindAndValidatePathParams(c, &params); err != nil {
			return err
		}

		sessionID := params.SessionID

		session, err := s.SessionManager.GetSession(ctx, sessionID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
			return httperrors.NewHTTPError(http.StatusNotFound, types.PublicHTTPErrorTypeGeneric, "Session not found")
		}

		// 确定 session_type
		sessionType := "signing"
		if session.Protocol != "" {
			// 根据协议判断是 DKG 还是 signing
			if strings.Contains(strings.ToLower(session.Protocol), "keygen") || session.KeyID == session.SessionID {
				sessionType = "dkg"
			}
		}

		// 构建 Progress 对象
		progress := &types.SessionResponseProgress{
			CurrentRound: int64(session.CurrentRound),
			TotalRounds:  int64(session.TotalRounds),
		}

		// 转换 SessionID 和 WalletID 为 UUID
		sessionIDUUID := strfmt.UUID(session.SessionID)
		walletIDUUID := strfmt.UUID(session.KeyID) // 使用 KeyID 作为 WalletID

		status := session.Status
		response := &types.SessionResponse{
			SessionID:   &sessionIDUUID,
			WalletID:    &walletIDUUID,
			SessionType: sessionType,
			Status:      &status,
			Progress:    progress,
			Signature:   session.Signature,
			PublicKey:   "", // 如果有公钥，从其他地方获取
			CreatedAt:   strfmt.DateTime(session.CreatedAt),
			DurationMs:  int64(session.DurationMs),
		}

		if session.CompletedAt != nil {
			response.CompletedAt = strfmt.DateTime(*session.CompletedAt)
		}

		return util.ValidateAndReturn(c, http.StatusOK, response)
	}
}
