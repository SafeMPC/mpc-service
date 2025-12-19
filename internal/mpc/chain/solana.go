package chain

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/pkg/errors"
)

// SolanaAdapter 用于 Solana 链的适配器
type SolanaAdapter struct{}

// NewSolanaAdapter 创建一个 Solana 适配器
func NewSolanaAdapter() *SolanaAdapter {
	return &SolanaAdapter{}
}

// GenerateAddress 根据公钥生成 Solana 地址（Base58 编码）
// Solana 地址就是 Ed25519 公钥的 Base58 表示
func (a *SolanaAdapter) GenerateAddress(pubKey []byte) (string, error) {
	if len(pubKey) == 0 {
		return "", errors.New("public key is required")
	}

	// Solana 公钥通常是 32 字节
	if len(pubKey) != 32 {
		return "", errors.Errorf("invalid public key length: expected 32 bytes, got %d", len(pubKey))
	}

	return base58.Encode(pubKey), nil
}

// BuildTransaction 构建 Solana 交易（暂未实现）
func (a *SolanaAdapter) BuildTransaction(req *BuildTxRequest) (*Transaction, error) {
	// TODO: 实现 Solana 交易构建
	return nil, errors.New("not implemented")
}
