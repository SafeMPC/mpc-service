package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil/base58"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ripemd160"
)

// BitcoinAdapter 基于 btcsuite 的简单实现
type BitcoinAdapter struct {
	params *chaincfg.Params
}

// NewBitcoinAdapter 创建一个 Bitcoin 适配器
func NewBitcoinAdapter(params *chaincfg.Params) *BitcoinAdapter {
	if params == nil {
		params = &chaincfg.MainNetParams
	}
	return &BitcoinAdapter{params: params}
}

// GenerateAddress 根据公钥生成标准的 Bitcoin P2PKH 地址（Base58 编码）
func (a *BitcoinAdapter) GenerateAddress(pubKey []byte) (string, error) {
	if len(pubKey) == 0 {
		return "", errors.New("public key is required")
	}

	// 1. 计算公钥哈希：SHA256 -> RIPEMD160
	sha := sha256.Sum256(pubKey)
	ripemd := ripemd160.New()
	if _, err := ripemd.Write(sha[:]); err != nil {
		return "", errors.Wrap(err, "failed to hash public key")
	}
	hash160 := ripemd.Sum(nil)

	// 2. 获取版本字节（根据网络参数）
	// chaincfg.Params.PubKeyHashAddrID 包含 P2PKH 地址的版本字节
	versionByte := byte(0x00) // 主网默认
	if a.params != nil && a.params.PubKeyHashAddrID != 0 {
		versionByte = a.params.PubKeyHashAddrID
	}

	// 3. 添加版本字节
	versionedPayload := append([]byte{versionByte}, hash160...)

	// 4. 计算校验和：SHA256(SHA256(version + hash160)) 的前4字节
	firstSHA := sha256.Sum256(versionedPayload)
	secondSHA := sha256.Sum256(firstSHA[:])
	checksum := secondSHA[:4]

	// 5. 拼接：版本字节 + hash160 + 校验和
	fullPayload := append(versionedPayload, checksum...)

	// 6. Base58 编码生成最终地址
	address := base58.Encode(fullPayload)

	return address, nil
}

// BuildTransaction 构建一个简单的原始交易描述并返回双哈希
func (a *BitcoinAdapter) BuildTransaction(req *BuildTxRequest) (*Transaction, error) {
	if req == nil {
		return nil, errors.New("build request is nil")
	}
	if req.Amount == nil {
		return nil, errors.New("amount is required")
	}

	raw := fmt.Sprintf(
		"btc-tx|from:%s|to:%s|amount:%s|nonce:%d|feerate:%d|data:%s",
		req.From,
		req.To,
		req.Amount.String(),
		req.Nonce,
		req.FeeRate,
		hex.EncodeToString(req.Data),
	)

	hash := chainhash.DoubleHashH([]byte(raw)).String()
	return &Transaction{
		Raw:  raw,
		Hash: hash,
	}, nil
}
