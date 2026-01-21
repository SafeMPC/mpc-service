package signing

// AuthToken 鉴权令牌
type AuthToken struct {
	PasskeySignature  []byte
	AuthenticatorData []byte
	ClientDataJSON    []byte
	CredentialID      string
}

// SignRequest 签名请求
type SignRequest struct {
	KeyID          string
	Message        []byte
	MessageHex     string
	MessageType    string // transaction, message, raw
	ChainType      string
	DerivationPath string
	AuthTokens     []AuthToken
	// 2-of-2 模式：手机节点ID（P1），必需
	MobileNodeID string
}

// SignResponse 签名响应
type SignResponse struct {
	Signature          string
	KeyID              string
	PublicKey          string
	Message            string
	ChainType          string
	SessionID          string
	SignedAt           string
	ParticipatingNodes []string
}

// BatchSignRequest 批量签名请求
type BatchSignRequest struct {
	KeyID     string
	Messages  []*SignRequest
	ChainType string
}

// BatchSignResponse 批量签名响应
type BatchSignResponse struct {
	Signatures []*SignResponse
	Total      int
	Success    int
	Failed     int
}

// VerifyRequest 验证请求
type VerifyRequest struct {
	Signature  string
	Message    []byte
	MessageHex string
	PublicKey  string
	Algorithm  string
	ChainType  string
}

// VerifyResponse 验证响应
type VerifyResponse struct {
	Valid      bool
	PublicKey  string
	Address    string
	VerifiedAt string
}
