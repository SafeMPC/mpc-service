package ethereum

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// RPCClient Ethereum RPC 客户端
type RPCClient struct {
	endpoint string
	client   *http.Client
}

// NewRPCClient 创建 Ethereum RPC 客户端
func NewRPCClient(endpoint string) *RPCClient {
	return &RPCClient{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RPCRequest RPC 请求
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// RPCResponse RPC 响应
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// RPCError RPC 错误
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call 执行 RPC 调用
func (c *RPCClient) call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	req := &RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal RPC request")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute HTTP request")
	}
	defer resp.Body.Close()

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, errors.Wrap(err, "failed to decode RPC response")
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return rpcResp.Result, nil
}

// GetBalance 查询余额
func (c *RPCClient) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	// 使用 "latest" 作为区块号
	result, err := c.call(ctx, "eth_getBalance", []interface{}{address, "latest"})
	if err != nil {
		return nil, errors.Wrap(err, "failed to call eth_getBalance")
	}

	// 解析结果（hex 字符串）
	var balanceHex string
	if err := json.Unmarshal(result, &balanceHex); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal balance")
	}

	// 移除 "0x" 前缀
	if len(balanceHex) > 2 && balanceHex[:2] == "0x" {
		balanceHex = balanceHex[2:]
	}

	// 转换为 big.Int
	balance := new(big.Int)
	balance, ok := balance.SetString(balanceHex, 16)
	if !ok {
		return nil, errors.New("failed to parse balance hex string")
	}

	return balance, nil
}

// GetTransactionCount 获取交易计数（用于 nonce）
func (c *RPCClient) GetTransactionCount(ctx context.Context, address string) (uint64, error) {
	result, err := c.call(ctx, "eth_getTransactionCount", []interface{}{address, "latest"})
	if err != nil {
		return 0, errors.Wrap(err, "failed to call eth_getTransactionCount")
	}

	var nonceHex string
	if err := json.Unmarshal(result, &nonceHex); err != nil {
		return 0, errors.Wrap(err, "failed to unmarshal nonce")
	}

	// 移除 "0x" 前缀
	if len(nonceHex) > 2 && nonceHex[:2] == "0x" {
		nonceHex = nonceHex[2:]
	}

	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return 0, errors.Wrap(err, "failed to decode nonce hex")
	}

	// 转换为 uint64
	nonceBig := new(big.Int).SetBytes(nonce)
	return nonceBig.Uint64(), nil
}

// SendRawTransaction 广播交易
func (c *RPCClient) SendRawTransaction(ctx context.Context, rawTx string) (string, error) {
	result, err := c.call(ctx, "eth_sendRawTransaction", []interface{}{rawTx})
	if err != nil {
		return "", errors.Wrap(err, "failed to call eth_sendRawTransaction")
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal transaction hash")
	}

	return txHash, nil
}

// GetGasPrice 获取当前 gas price
func (c *RPCClient) GetGasPrice(ctx context.Context) (*big.Int, error) {
	result, err := c.call(ctx, "eth_gasPrice", []interface{}{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to call eth_gasPrice")
	}

	var gasPriceHex string
	if err := json.Unmarshal(result, &gasPriceHex); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal gas price")
	}

	// 移除 "0x" 前缀
	if len(gasPriceHex) > 2 && gasPriceHex[:2] == "0x" {
		gasPriceHex = gasPriceHex[2:]
	}

	gasPrice := new(big.Int)
	gasPrice, ok := gasPrice.SetString(gasPriceHex, 16)
	if !ok {
		return nil, errors.New("failed to parse gas price hex string")
	}

	return gasPrice, nil
}
