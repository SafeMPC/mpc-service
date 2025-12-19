package keys

import (
	"net/http"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/kashguard/go-mpc-infra/internal/api"
	"github.com/kashguard/go-mpc-infra/internal/api/httperrors"
	"github.com/kashguard/go-mpc-infra/internal/infra/key"
	"github.com/kashguard/go-mpc-infra/internal/types"
	"github.com/kashguard/go-mpc-infra/internal/util"
	"github.com/labstack/echo/v4"
)

// PostDeriveKeyRoute registers the route for key derivation
func PostDeriveKeyRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Infra.POST("/keys/derive", postDeriveKeyHandler(s))
}

func postDeriveKeyHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		var body types.PostDeriveKeyPayload
		if err := util.BindAndValidateBody(c, &body); err != nil {
			return err
		}

		// Build DeriveWalletKeyRequest
		req := &key.DeriveWalletKeyRequest{
			RootKeyID:   swag.StringValue(body.RootKeyID),
			ChainType:   swag.StringValue(body.ChainType),
			Index:       uint32(swag.Int64Value(body.Index)),
			Description: body.Description,
			Tags:        body.Tags,
		}

		// Call KeyService.DeriveWalletKey
		walletKey, err := s.KeyService.DeriveWalletKey(ctx, req)
		if err != nil {
			log.Error().Err(err).Msg("Failed to derive wallet key")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to derive wallet key: "+err.Error())
		}

		log.Info().Str("wallet_id", walletKey.WalletID).Msg("Wallet key derived successfully")

		// Construct response
		response := &types.DeriveKeyResponse{
			WalletID:    swag.String(walletKey.WalletID),
			RootKeyID:   swag.String(walletKey.RootKeyID),
			PublicKey:   swag.String(walletKey.PublicKey),
			ChainCode:   swag.String(walletKey.ChainCode),
			ChainType:   swag.String(walletKey.ChainType),
			Address:     walletKey.Address,
			Index:       swag.Int64(int64(walletKey.Index)),
			Status:      swag.String(string(walletKey.Status)),
			Description: walletKey.Description,
			Tags:        walletKey.Tags,
			CreatedAt:   strfmt.DateTime(walletKey.CreatedAt),
		}

		return c.JSON(http.StatusCreated, response)
	}
}
