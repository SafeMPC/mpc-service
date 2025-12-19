package keys

import (
	"net/http"

	"github.com/kashguard/go-mpc-infra/internal/api"
	"github.com/kashguard/go-mpc-infra/internal/api/httperrors"
	"github.com/kashguard/go-mpc-infra/internal/types"
	"github.com/kashguard/go-mpc-infra/internal/util"
	"github.com/labstack/echo/v4"
)

func DeleteWalletKeyRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Infra.DELETE("/wallets/:wallet_id", deleteWalletKeyHandler(s))
}

func deleteWalletKeyHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		walletID := c.Param("wallet_id")
		if walletID == "" {
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, "wallet_id is required")
		}

		if err := s.KeyService.DeleteWalletKey(ctx, walletID); err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to delete wallet key")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to delete wallet key")
		}

		return c.NoContent(http.StatusOK)
	}
}
