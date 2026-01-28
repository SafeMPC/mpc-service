package webauthn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/SafeMPC/mpc-service/internal/infra/storage"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Service WebAuthn 服务
type Service struct {
	webAuthn      *webauthn.WebAuthn
	metadataStore storage.MetadataStore
	rpID          string
	rpName        string
	rpOrigin      string
}

// GetMetadataStore 获取元数据存储（用于 gRPC Server）
func (s *Service) GetMetadataStore() storage.MetadataStore {
	return s.metadataStore
}

// NewService 创建 WebAuthn 服务
func NewService(
	rpID string,
	rpName string,
	rpOrigin string,
	metadataStore storage.MetadataStore,
) (*Service, error) {
	wconfig := &webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpName,
		RPOrigins:     []string{rpOrigin},
	}

	webAuthn, err := webauthn.New(wconfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create webauthn instance")
	}

	return &Service{
		webAuthn:      webAuthn,
		metadataStore: metadataStore,
		rpID:          rpID,
		rpName:        rpName,
		rpOrigin:      rpOrigin,
	}, nil
}

// BeginRegistration 开始 Passkey 注册
func (s *Service) BeginRegistration(ctx context.Context, userID string, userName string) (*protocol.CredentialCreation, string, error) {
	// 生成随机 challenge（32 字节）
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return nil, "", errors.Wrap(err, "failed to generate challenge")
	}

	// 创建用户对象
	user := &User{
		ID:          userID,
		Name:        userName,
		DisplayName: userName,
	}

	// 查询用户已有的凭证（排除重复注册）
	existingCredentials, err := s.getUserCredentials(ctx, userID)
	if err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("Failed to get existing credentials")
		existingCredentials = []webauthn.Credential{}
	}

	user.Credentials = existingCredentials

	// 创建注册选项
	options, sessionData, err := s.webAuthn.BeginRegistration(
		user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.Platform, // 平台认证器（TouchID/FaceID）
			RequireResidentKey:      protocol.ResidentKeyNotRequired(),
			UserVerification:        protocol.VerificationRequired, // 要求用户验证
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to begin registration")
	}

	// 保存 session data（实际应该存到 Redis，这里简化）
	// sessionData.Challenge 已经是 string 类型，直接使用
	sessionDataJSON := sessionData.Challenge

	log.Info().
		Str("user_id", userID).
		Str("challenge", sessionDataJSON).
		Msg("WebAuthn registration started")

	return options, sessionDataJSON, nil
}

// FinishRegistration 完成 Passkey 注册
func (s *Service) FinishRegistration(ctx context.Context, userID string, userName string, sessionData string, credentialResponse *protocol.ParsedCredentialCreationData) error {
	// sessionData 就是 challenge string，直接使用
	challengeString := sessionData

	// 创建用户对象
	user := &User{
		ID:          userID,
		Name:        userName,
		DisplayName: userName,
	}

	// 查询用户已有的凭证
	existingCredentials, err := s.getUserCredentials(ctx, userID)
	if err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("Failed to get existing credentials")
		existingCredentials = []webauthn.Credential{}
	}
	user.Credentials = existingCredentials

	// 验证注册响应
	session := webauthn.SessionData{
		Challenge:            challengeString,
		UserID:               []byte(userID),
		UserVerification:     protocol.VerificationRequired,
		RelyingPartyID:       s.rpID,
		AllowedCredentialIDs: [][]byte{},
	}

	credential, err := s.webAuthn.CreateCredential(user, session, credentialResponse)
	if err != nil {
		return errors.Wrap(err, "failed to create credential")
	}

	// 保存 Passkey 到数据库
	publicKeyHex := hex.EncodeToString(credential.PublicKey)
	credentialIDBase64 := base64.RawURLEncoding.EncodeToString(credential.ID)

	passkey := &storage.Passkey{
		CredentialID: credentialIDBase64,
		PublicKey:    publicKeyHex,
		DeviceName:   "", // 可以从 authenticator data 中提取
	}

	// 1. 保存 Passkey
	if err := s.metadataStore.SavePasskey(ctx, passkey); err != nil {
		return errors.Wrap(err, "failed to save passkey")
	}

	// 2. 保存用户凭证关联
	if err := s.metadataStore.SaveUserCredential(ctx, userID, credentialIDBase64, passkey.DeviceName); err != nil {
		return errors.Wrap(err, "failed to save user credential")
	}

	log.Info().
		Str("user_id", userID).
		Str("credential_id", credentialIDBase64).
		Msg("Passkey registered successfully")

	return nil
}

// BeginLogin 开始 Passkey 登录
func (s *Service) BeginLogin(ctx context.Context, userID string) (*protocol.CredentialAssertion, string, error) {
	// 生成随机 challenge
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return nil, "", errors.Wrap(err, "failed to generate challenge")
	}

	// 创建用户对象
	user := &User{
		ID:          userID,
		Name:        userID,
		DisplayName: userID,
	}

	// 查询用户的凭证
	existingCredentials, err := s.getUserCredentials(ctx, userID)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to get user credentials")
	}

	if len(existingCredentials) == 0 {
		return nil, "", errors.New("no credentials found for user")
	}

	user.Credentials = existingCredentials

	// 创建登录选项
	options, sessionData, err := s.webAuthn.BeginLogin(
		user,
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to begin login")
	}

	// 保存 session data
	// sessionData.Challenge 已经是 string 类型，直接使用
	sessionDataJSON := sessionData.Challenge

	log.Info().
		Str("user_id", userID).
		Str("challenge", sessionDataJSON).
		Int("credentials_count", len(existingCredentials)).
		Msg("WebAuthn login started")

	return options, sessionDataJSON, nil
}

// FinishLogin 完成 Passkey 登录
func (s *Service) FinishLogin(ctx context.Context, userID string, sessionData string, credentialResponse *protocol.ParsedCredentialAssertionData) error {
	// sessionData 就是 challenge string，直接使用
	challengeString := sessionData

	// 创建用户对象
	user := &User{
		ID:          userID,
		Name:        userID,
		DisplayName: userID,
	}

	// 查询用户的凭证
	existingCredentials, err := s.getUserCredentials(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "failed to get user credentials")
	}

	if len(existingCredentials) == 0 {
		return errors.New("no credentials found for user")
	}

	user.Credentials = existingCredentials

	// 验证登录响应
	session := webauthn.SessionData{
		Challenge:        challengeString,
		UserID:           []byte(userID),
		UserVerification: protocol.VerificationRequired,
		RelyingPartyID:   s.rpID,
	}

	_, err = s.webAuthn.ValidateLogin(user, session, credentialResponse)
	if err != nil {
		return errors.Wrap(err, "failed to validate login")
	}

	log.Info().
		Str("user_id", userID).
		Msg("Passkey login successful")

	return nil
}

// VerifyAssertion 验证 Passkey 签名（用于关键操作的二次验证）
func (s *Service) VerifyAssertion(ctx context.Context, credentialID string, challenge []byte, authData []byte, clientDataJSON []byte, signature []byte) error {
	// 从数据库获取 Passkey 公钥
	credentialIDBase64 := base64.RawURLEncoding.EncodeToString([]byte(credentialID))
	passkey, err := s.metadataStore.GetPasskey(ctx, credentialIDBase64)
	if err != nil {
		return errors.Wrap(err, "failed to get passkey")
	}

	// 使用 auth 包的验证函数
	expectedChallenge := base64.RawURLEncoding.EncodeToString(challenge)
	
	// 导入验证函数（从 internal/auth/passkey.go）
	// 这里需要调用 auth.VerifyPasskeySignature
	// 为了避免循环依赖，我们直接实现验证逻辑
	
	return s.verifyPasskeySignature(passkey.PublicKey, signature, authData, clientDataJSON, expectedChallenge)
}

// getUserCredentials 获取用户的所有凭证
func (s *Service) getUserCredentials(ctx context.Context, userID string) ([]webauthn.Credential, error) {
	// 从数据库查询用户的所有 Passkey
	passkeys, err := s.metadataStore.ListUserPasskeys(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list user passkeys")
	}

	// 转换为 webauthn.Credential
	credentials := make([]webauthn.Credential, 0, len(passkeys))
	for _, pk := range passkeys {
		credentialIDBytes, err := base64.RawURLEncoding.DecodeString(pk.CredentialID)
		if err != nil {
			log.Warn().Err(err).Str("credential_id", pk.CredentialID).Msg("Failed to decode credential ID")
			continue
		}

		publicKeyBytes, err := hex.DecodeString(pk.PublicKey)
		if err != nil {
			log.Warn().Err(err).Str("credential_id", pk.CredentialID).Msg("Failed to decode public key")
			continue
		}

		credentials = append(credentials, webauthn.Credential{
			ID:        credentialIDBytes,
			PublicKey: publicKeyBytes,
		})
	}

	return credentials, nil
}

// verifyPasskeySignature 验证 Passkey 签名（内部实现）
func (s *Service) verifyPasskeySignature(publicKeyHex string, signature []byte, authData []byte, clientDataJSON []byte, expectedChallenge string) error {
	// 使用 auth 包的实现
	// 这里应该调用 auth.VerifyPasskeySignature
	// 为了避免循环依赖，建议将验证逻辑移到 pkg/ 或单独的包
	return fmt.Errorf("not implemented: use auth.VerifyPasskeySignature")
}
