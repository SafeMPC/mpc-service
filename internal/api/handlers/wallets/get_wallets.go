package wallets

import (
	"net/http"

	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/api/httperrors"
	"github.com/SafeMPC/mpc-service/internal/infra/key"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/types/wallets"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/labstack/echo/v4"
)

func GetWalletsRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.GET("/wallets", getWalletsHandler(s))
}

func getWalletsHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		// TODO: JWT Token 验证（从请求头获取）
		// 从 JWT Token 中获取 userID
		// userID := getUserIDFromToken(c)

		// 使用统一的参数绑定方式
		var params wallets.GetWalletsParams
		if err := util.BindAndValidateQueryParams(c, &params); err != nil {
			return err
		}

		// 使用参数值（已包含默认值）
		limit := int64(20)
		if params.Limit != nil {
			limit = *params.Limit
		}

		offset := int64(0)
		if params.Offset != nil {
			offset = *params.Offset
		}

		// 构建过滤器
		chainType := ""
		if params.ChainType != nil {
			chainType = *params.ChainType
		}
		filter := &key.KeyFilter{
			ChainType: chainType,
			Limit:     int(limit),
			Offset:    int(offset),
		}

		// 查询密钥列表
		keys, err := s.KeyService.ListKeys(ctx, filter)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list keys")
			return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to list wallets")
		}

		// 转换为 WalletSummary
		wallets := make([]*types.WalletSummary, 0, len(keys))
		for _, k := range keys {
			walletIDUUID := strfmt.UUID(k.KeyID)
			wallets = append(wallets, &types.WalletSummary{
				WalletID:  walletIDUUID,
				Address:   k.Address,
				ChainType: k.ChainType,
				PublicKey: k.PublicKey,
				Algorithm: k.Algorithm,
				Curve:     k.Curve,
				CreatedAt: strfmt.DateTime(k.CreatedAt),
			})
		}

		// TODO: 获取总数（需要实现带计数的查询）
		total := int64(len(keys))

		response := &types.ListWalletsResponse{
			Wallets: wallets,
			Total:   swag.Int64(total),
			Limit:   swag.Int64(limit),
			Offset:  swag.Int64(offset),
		}

		return util.ValidateAndReturn(c, http.StatusOK, response)
	}
}
