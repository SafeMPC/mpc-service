package webauthn

import "github.com/go-webauthn/webauthn/webauthn"

// User 实现 webauthn.User 接口
type User struct {
	ID          string
	Name        string
	DisplayName string
	Credentials []webauthn.Credential
}

// WebAuthnID 返回用户 ID
func (u *User) WebAuthnID() []byte {
	return []byte(u.ID)
}

// WebAuthnName 返回用户名
func (u *User) WebAuthnName() string {
	return u.Name
}

// WebAuthnDisplayName 返回显示名称
func (u *User) WebAuthnDisplayName() string {
	return u.DisplayName
}

// WebAuthnIcon 返回用户图标 URL
func (u *User) WebAuthnIcon() string {
	return ""
}

// WebAuthnCredentials 返回用户的凭证列表
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// RegistrationRequest 注册请求
type RegistrationRequest struct {
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	DisplayName string `json:"display_name"`
}

// RegistrationResponse 注册响应
type RegistrationResponse struct {
	Options     interface{} `json:"options"`      // protocol.CredentialCreation
	SessionData string      `json:"session_data"` // Base64URL 编码的 challenge
}

// LoginRequest 登录请求
type LoginRequest struct {
	UserID string `json:"user_id"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Options     interface{} `json:"options"`      // protocol.CredentialAssertion
	SessionData string      `json:"session_data"` // Base64URL 编码的 challenge
}

// VerifyRegistrationRequest 验证注册请求
type VerifyRegistrationRequest struct {
	UserID      string      `json:"user_id"`
	UserName    string      `json:"user_name"`
	SessionData string      `json:"session_data"`
	Response    interface{} `json:"response"` // 前端返回的 credential
}

// VerifyLoginRequest 验证登录请求
type VerifyLoginRequest struct {
	UserID      string      `json:"user_id"`
	SessionData string      `json:"session_data"`
	Response    interface{} `json:"response"` // 前端返回的 assertion
}

// VerifyAssertionRequest 验证断言请求（用于关键操作）
type VerifyAssertionRequest struct {
	CredentialID      string `json:"credential_id"`
	Challenge         []byte `json:"challenge"`
	AuthenticatorData []byte `json:"authenticator_data"`
	ClientDataJSON    []byte `json:"client_data_json"`
	Signature         []byte `json:"signature"`
}
