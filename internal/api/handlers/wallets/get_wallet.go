package wallets

import (
	"net/http"

	"github.com/go-openapi/swag"
	"github.com/go-openapi/strfmt"
	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/api/httperrors"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/types/wallets"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/labstack/echo/v4"
)

func GetWalletRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.GET("/wallets/:wallet_id", getWalletHandler(s))
}

func getWalletHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		// TODO: JWT Token 验证（从请求头获取）

		// 使用统一的参数绑定方式
		var params wallets.GetWalletParams
		if err := util.BindAndValidatePathParams(c, &params); err != nil {
			return err
		}

		walletID := params.WalletID

		// 查询密钥信息（钱包 ID 等于密钥 ID）
		keyMetadata, err := s.KeyService.GetKey(ctx, walletID)
		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to get key")
			return httperrors.NewHTTPError(http.StatusNotFound, types.PublicHTTPErrorTypeGeneric, "Wallet not found")
		}

		// 转换为 WalletResponse
		walletIDUUID := strfmt.UUID(keyMetadata.KeyID)
		response := &types.WalletResponse{
			WalletID:  &walletIDUUID,
			Address:   swag.String(keyMetadata.Address),
			ChainType: swag.String(keyMetadata.ChainType),
			PublicKey: swag.String(keyMetadata.PublicKey),
			Algorithm: keyMetadata.Algorithm,
			Curve:     keyMetadata.Curve,
			Threshold: int64(keyMetadata.Threshold),
			TotalNodes: int64(keyMetadata.TotalNodes),
			CreatedAt: strfmt.DateTime(keyMetadata.CreatedAt),
		}

		return util.ValidateAndReturn(c, http.StatusOK, response)
	}
}
