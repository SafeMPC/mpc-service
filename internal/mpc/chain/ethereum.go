package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pkg/errors"

	"github.com/SafeMPC/mpc-service/internal/mpc/chain/ethereum"
)

// EthereumAdapter 实现 EVM 链基础能力
type EthereumAdapter struct {
	chainID  *big.Int
	rpcClient *ethereum.RPCClient
}

// NewEthereumAdapter 创建以太坊适配器
func NewEthereumAdapter(chainID *big.Int, rpcEndpoint string) *EthereumAdapter {
	if chainID == nil {
		chainID = big.NewInt(1) // mainnet
	}

	var rpcClient *ethereum.RPCClient
	if rpcEndpoint != "" {
		rpcClient = ethereum.NewRPCClient(rpcEndpoint)
	}

	return &EthereumAdapter{
		chainID:   chainID,
		rpcClient: rpcClient,
	}
}

// GetBalance 查询余额
func (a *EthereumAdapter) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	if a.rpcClient == nil {
		return nil, errors.New("RPC client not configured")
	}
	return a.rpcClient.GetBalance(ctx, address)
}

// GetTransactionCount 获取交易计数（用于 nonce）
func (a *EthereumAdapter) GetTransactionCount(ctx context.Context, address string) (uint64, error) {
	if a.rpcClient == nil {
		return 0, errors.New("RPC client not configured")
	}
	return a.rpcClient.GetTransactionCount(ctx, address)
}

// BroadcastTransaction 广播交易
func (a *EthereumAdapter) BroadcastTransaction(ctx context.Context, rawTx string) (string, error) {
	if a.rpcClient == nil {
		return "", errors.New("RPC client not configured")
	}
	return a.rpcClient.SendRawTransaction(ctx, rawTx)
}

// GetGasPrice 获取当前 gas price
func (a *EthereumAdapter) GetGasPrice(ctx context.Context) (*big.Int, error) {
	if a.rpcClient == nil {
		return nil, errors.New("RPC client not configured")
	}
	return a.rpcClient.GetGasPrice(ctx)
}

// GenerateAddress 通过 Keccak256(pubKey[1:]) 生成地址
func (a *EthereumAdapter) GenerateAddress(pubKey []byte) (string, error) {
	if len(pubKey) == 0 {
		return "", errors.New("public key is required")
	}
	var uncompressed64 []byte
	switch {
	case len(pubKey) == 65 && pubKey[0] == 0x04:
		uncompressed64 = pubKey[1:]
	case len(pubKey) == 33 && (pubKey[0] == 0x02 || pubKey[0] == 0x03):
		key, err := btcec.ParsePubKey(pubKey)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse compressed secp256k1 pubkey")
		}
		u := key.SerializeUncompressed() // 65 bytes, 0x04 | X | Y
		uncompressed64 = u[1:]
	default:
		return "", errors.Errorf("unsupported public key format: len=%d", len(pubKey))
	}
	hash := crypto.Keccak256(uncompressed64)
	return fmt.Sprintf("0x%s", hex.EncodeToString(hash[12:])), nil
}

// BuildTransaction 构建一个简化的 RLP 交易负载
func (a *EthereumAdapter) BuildTransaction(req *BuildTxRequest) (*Transaction, error) {
	if req == nil {
		return nil, errors.New("build request is nil")
	}
	if req.Amount == nil {
		return nil, errors.New("amount is required")
	}

	txPayload := []interface{}{
		req.Nonce,
		req.FeeRate, // 这里复用 FeeRate 作为 gas price
		uint64(21000),
		req.To,
		req.Amount,
		req.Data,
		a.chainID,
		uint(0),
		uint(0),
	}

	raw, err := rlp.EncodeToBytes(txPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to RLP encode tx payload")
	}

	hash := crypto.Keccak256Hash(raw).Hex()
	return &Transaction{
		Raw:  fmt.Sprintf("0x%s", hex.EncodeToString(raw)),
		Hash: hash,
	}, nil
}
