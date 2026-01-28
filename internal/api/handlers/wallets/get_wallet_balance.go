package wallets

import (
	"math/big"
	"net/http"

	"github.com/go-openapi/swag"
	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/api/httperrors"
	"github.com/SafeMPC/mpc-service/internal/mpc/chain"
	"github.com/SafeMPC/mpc-service/internal/types"
	"github.com/SafeMPC/mpc-service/internal/types/wallets"
	"github.com/SafeMPC/mpc-service/internal/util"
	"github.com/labstack/echo/v4"
)

// GetWalletBalanceRoute 注册余额查询路由
func GetWalletBalanceRoute(s *api.Server) *echo.Route {
	return s.Router.APIV1Auth.GET("/wallets/:walletId/balance", getWalletBalanceHandler(s))
}

// getWalletBalanceHandler 查询钱包余额
func getWalletBalanceHandler(s *api.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		log := util.LogFromContext(ctx)

		// TODO: JWT Token 验证（从请求头获取）

		// 使用统一的参数绑定方式
		var params wallets.GetWalletBalanceParams
		if err := util.BindAndValidatePathAndQueryParams(c, &params); err != nil {
			return err
		}

		walletID := params.WalletID
		chainType := params.ChainType

		if chainType == "" {
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, "chain_type is required")
		}

		// 查询密钥信息（钱包 ID 等于密钥 ID）
		keyMetadata, err := s.KeyService.GetKey(ctx, walletID)
		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to get key")
			return httperrors.NewHTTPError(http.StatusNotFound, types.PublicHTTPErrorTypeGeneric, "Wallet not found")
		}

		// 如果地址不存在，生成地址
		address := keyMetadata.Address
		if address == "" {
			// 生成地址
			address, err = s.KeyService.GenerateAddress(ctx, walletID, chainType)
			if err != nil {
				log.Error().Err(err).Str("wallet_id", walletID).Str("chain_type", chainType).Msg("Failed to generate address")
				return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to generate address")
			}
		}

		// 根据链类型选择适配器并查询余额
		var balance *big.Int
		var symbol string
		var decimals int64

		switch chainType {
		case "ethereum":
			// 创建 Ethereum 适配器（使用 Sepolia 测试网 ChainID: 11155111）
			// TODO: 从配置获取 RPC 端点和 ChainID
			rpcEndpoint := "https://sepolia.infura.io/v3/YOUR_API_KEY" // TODO: 从配置读取
			chainID := big.NewInt(11155111)                            // Sepolia
			ethereumAdapter := chain.NewEthereumAdapter(chainID, rpcEndpoint)

			balance, err = ethereumAdapter.GetBalance(ctx, address)
			if err != nil {
				log.Error().Err(err).Str("address", address).Msg("Failed to get Ethereum balance")
				return httperrors.NewHTTPError(http.StatusInternalServerError, types.PublicHTTPErrorTypeGeneric, "Failed to query balance")
			}

			symbol = "ETH"
			decimals = 18
		default:
			return httperrors.NewHTTPError(http.StatusBadRequest, types.PublicHTTPErrorTypeGeneric, "Unsupported chain type: "+chainType)
		}

		// 转换余额为字符串（Wei -> ETH）
		// 1 ETH = 10^18 Wei
		ethDivisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil))
		balanceFloat := new(big.Float).SetInt(balance)
		ethBalance := new(big.Float).Quo(balanceFloat, ethDivisor)

		decimalsPtr := swag.Int64(decimals)
		response := &types.WalletBalanceResponse{
			Balance:   swag.String(ethBalance.Text('f', 18)),
			Symbol:    swag.String(symbol),
			Decimals:  decimalsPtr,
			ChainType: chainType,
		}

		return util.ValidateAndReturn(c, http.StatusOK, response)
	}
}
