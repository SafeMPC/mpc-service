package sessions

import (
	"net/http"

	"github.com/kashguard/go-mpc-infra/internal/api"
	"github.com/kashguard/go-mpc-infra/internal/api/httperrors"
	"github.com/kashguard/go-mpc-infra/internal/types"
	"github.com/kashguard/go-mpc-infra/internal/util"
	"github.com/labstack/echo/v4"
)

func PostCancelSessionRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Infra.POST("/sessions/:session_id/cancel", postCancelSessionHandler(s))
}

func postCancelSessionHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		sessionID := c.Param("session_id")
		if sessionID == "" {
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, "session_id is required")
		}

		err := s.SessionManager.CancelSession(ctx, sessionID)
		if err != nil {
			log.Error().Err(err).Str("session_id", sessionID).Msg("Failed to cancel session")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to cancel session")
		}

		response := map[string]interface{}{
			"session_id": sessionID,
			"status":     "cancelled",
		}

		return c.JSON(http.StatusOK, response)
	}
}
