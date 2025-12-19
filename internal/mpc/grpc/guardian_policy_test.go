package grpc

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	"github.com/kashguard/go-mpc-infra/internal/infra/storage"
	pb "github.com/kashguard/go-mpc-infra/internal/pb/mpc/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMetadataStore 模拟 MetadataStore
type MockMetadataStore struct {
	storage.MetadataStore
	policies map[string]*storage.SigningPolicy
	authKeys map[string][]*storage.UserAuthKey
}

func NewMockMetadataStore() *MockMetadataStore {
	return &MockMetadataStore{
		policies: make(map[string]*storage.SigningPolicy),
		authKeys: make(map[string][]*storage.UserAuthKey),
	}
}

func (m *MockMetadataStore) GetSigningPolicy(ctx context.Context, keyID string) (*storage.SigningPolicy, error) {
	if policy, ok := m.policies[keyID]; ok {
		return policy, nil
	}
	return nil, nil // Return nil if not found, consistent with logic in checkGuardianPolicy
}

func (m *MockMetadataStore) ListUserAuthKeys(ctx context.Context, keyID string) ([]*storage.UserAuthKey, error) {
	if keys, ok := m.authKeys[keyID]; ok {
		return keys, nil
	}
	return []*storage.UserAuthKey{}, nil
}

// 辅助方法：添加策略
func (m *MockMetadataStore) AddPolicy(policy *storage.SigningPolicy) {
	m.policies[policy.WalletID] = policy
}

// 辅助方法：添加 AuthKey
func (m *MockMetadataStore) AddAuthKey(key *storage.UserAuthKey) {
	m.authKeys[key.WalletID] = append(m.authKeys[key.WalletID], key)
}

func TestCheckGuardianPolicy(t *testing.T) {
	// 1. Setup
	mockStore := NewMockMetadataStore()
	server := &GRPCServer{
		metadataStore: mockStore,
	}

	keyID := "test-wallet-1"
	message := []byte("hello world")
	messageHex := hex.EncodeToString(message)

	// 2. 生成测试密钥 (Ed25519)
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	pubKeyHex := hex.EncodeToString(pubKey)

	// 3. 配置策略 (2-of-2, but we only have 1 key for now, let's test 1-of-1 first)
	policy := &storage.SigningPolicy{
		WalletID:      keyID,
		PolicyType:    "single",
		MinSignatures: 1,
	}
	mockStore.AddPolicy(policy)

	// 4. 配置 AuthKey
	authKey := &storage.UserAuthKey{
		WalletID:     keyID,
		PublicKeyHex: pubKeyHex,
		Role:         "admin",
	}
	mockStore.AddAuthKey(authKey)

	// 5. 正常签名测试
	sig := ed25519.Sign(privKey, message)
	req := &pb.StartSignRequest{
		KeyId:      keyID,
		MessageHex: messageHex,
		AuthTokens: []*pb.StartSignRequest_AuthToken{
			{
				PublicKey: pubKeyHex,
				Signature: sig,
			},
		},
	}

	err = server.checkGuardianPolicy(context.Background(), req)
	assert.NoError(t, err, "Valid signature should pass")

	// 6. 无效签名测试
	badSig := make([]byte, len(sig))
	copy(badSig, sig)
	badSig[0] ^= 0xFF // Corrupt signature

	reqBad := &pb.StartSignRequest{
		KeyId:      keyID,
		MessageHex: messageHex,
		AuthTokens: []*pb.StartSignRequest_AuthToken{
			{
				PublicKey: pubKeyHex,
				Signature: badSig,
			},
		},
	}
	err = server.checkGuardianPolicy(context.Background(), reqBad)
	assert.Error(t, err, "Invalid signature should fail")
	assert.Contains(t, err.Error(), "insufficient valid signatures")

	// 7. 未授权公钥测试
	otherPub, _, _ := ed25519.GenerateKey(nil)
	otherPubHex := hex.EncodeToString(otherPub)
	// Don't add to store

	reqUnauth := &pb.StartSignRequest{
		KeyId:      keyID,
		MessageHex: messageHex,
		AuthTokens: []*pb.StartSignRequest_AuthToken{
			{
				PublicKey: otherPubHex,
				Signature: sig, // Signature doesn't matter as key is not found
			},
		},
	}
	err = server.checkGuardianPolicy(context.Background(), reqUnauth)
	assert.Error(t, err, "Unauthorized key should fail")

	// 8. 阈值测试 (需 2 个签名)
	policy2 := &storage.SigningPolicy{
		WalletID:      keyID,
		PolicyType:    "team",
		MinSignatures: 2,
	}
	mockStore.AddPolicy(policy2)

	// Add second key
	pubKey2, privKey2, _ := ed25519.GenerateKey(nil)
	pubKeyHex2 := hex.EncodeToString(pubKey2)
	authKey2 := &storage.UserAuthKey{
		WalletID:     keyID,
		PublicKeyHex: pubKeyHex2,
		Role:         "member",
	}
	mockStore.AddAuthKey(authKey2)

	// Only 1 signature provided
	sig2 := ed25519.Sign(privKey2, message)
	reqThresholdFail := &pb.StartSignRequest{
		KeyId:      keyID,
		MessageHex: messageHex,
		AuthTokens: []*pb.StartSignRequest_AuthToken{
			{
				PublicKey: pubKeyHex,
				Signature: sig,
			},
		},
	}
	err = server.checkGuardianPolicy(context.Background(), reqThresholdFail)
	assert.Error(t, err, "Insufficient signatures should fail")

	// 2 signatures provided
	reqThresholdPass := &pb.StartSignRequest{
		KeyId:      keyID,
		MessageHex: messageHex,
		AuthTokens: []*pb.StartSignRequest_AuthToken{
			{
				PublicKey: pubKeyHex,
				Signature: sig,
			},
			{
				PublicKey: pubKeyHex2,
				Signature: sig2,
			},
		},
	}
	err = server.checkGuardianPolicy(context.Background(), reqThresholdPass)
	assert.NoError(t, err, "Sufficient signatures should pass")
}
