package key

import "time"

// RootKeyMetadata 根密钥元数据
type RootKeyMetadata struct {
	KeyID        string
	PublicKey    string
	Algorithm    string
	Curve        string
	ChainCode    string // Hex encoded chain code (32 bytes)
	Threshold    int
	TotalNodes   int
	Protocol     string // gg18, gg20, frost
	Status       string
	Description  string
	Tags         map[string]string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletionDate *time.Time
}

// WalletKeyMetadata 钱包密钥元数据
type WalletKeyMetadata struct {
	WalletID     string
	RootKeyID    string
	ChainType    string
	Index        uint32 // 派生索引
	PublicKey    string
	ChainCode    string // Hex encoded chain code
	Address      string
	Status       string
	Description  string
	Tags         map[string]string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletionDate *time.Time
}

// CreateRootKeyRequest 创建根密钥请求
type CreateRootKeyRequest struct {
	KeyID       string // 可选的密钥ID，如果为空则自动生成
	Algorithm   string
	Curve       string
	Protocol    string // gg18, gg20, frost
	Threshold   int    // 默认 2
	TotalNodes  int    // 默认 3
	Description string
	Tags        map[string]string
	// 2-of-2 模式：手机节点ID（P1），必需
	MobileNodeID string
}

// DeriveWalletKeyRequest 派生钱包密钥请求
type DeriveWalletKeyRequest struct {
	RootKeyID   string
	ChainType   string
	Index       uint32
	Description string
	Tags        map[string]string
}

// DeriveWalletKeyByPathRequest 派生钱包密钥请求（支持路径）
type DeriveWalletKeyByPathRequest struct {
	RootKeyID   string
	ChainType   string
	Path        string // e.g. "m/44'/60'/0'/0/0"
	Description string
	Tags        map[string]string
}
