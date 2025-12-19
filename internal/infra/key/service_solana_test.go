package key

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcutil/base58"
	"github.com/kashguard/go-mpc-infra/internal/infra/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMetadataStoreSolana for testing
type MockMetadataStoreSolana struct {
	mock.Mock
}

// Key Operations
func (m *MockMetadataStoreSolana) SaveKeyMetadata(ctx context.Context, key *storage.KeyMetadata) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) GetKeyMetadata(ctx context.Context, keyID string) (*storage.KeyMetadata, error) {
	args := m.Called(ctx, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.KeyMetadata), args.Error(1)
}

func (m *MockMetadataStoreSolana) UpdateKeyMetadata(ctx context.Context, key *storage.KeyMetadata) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) DeleteKeyMetadata(ctx context.Context, keyID string) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) ListKeys(ctx context.Context, filter *storage.KeyFilter) ([]*storage.KeyMetadata, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.KeyMetadata), args.Error(1)
}

// Node Operations
func (m *MockMetadataStoreSolana) SaveNode(ctx context.Context, node *storage.NodeInfo) error {
	args := m.Called(ctx, node)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) GetNode(ctx context.Context, nodeID string) (*storage.NodeInfo, error) {
	args := m.Called(ctx, nodeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.NodeInfo), args.Error(1)
}

func (m *MockMetadataStoreSolana) UpdateNode(ctx context.Context, node *storage.NodeInfo) error {
	args := m.Called(ctx, node)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) ListNodes(ctx context.Context, filter *storage.NodeFilter) ([]*storage.NodeInfo, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.NodeInfo), args.Error(1)
}

func (m *MockMetadataStoreSolana) UpdateNodeHeartbeat(ctx context.Context, nodeID string) error {
	args := m.Called(ctx, nodeID)
	return args.Error(0)
}

// Session Operations
func (m *MockMetadataStoreSolana) SaveSigningSession(ctx context.Context, session *storage.SigningSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) GetSigningSession(ctx context.Context, sessionID string) (*storage.SigningSession, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.SigningSession), args.Error(1)
}

func (m *MockMetadataStoreSolana) UpdateSigningSession(ctx context.Context, session *storage.SigningSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

// Delegated Guardian Operations
func (m *MockMetadataStoreSolana) GetSigningPolicy(ctx context.Context, keyID string) (*storage.SigningPolicy, error) {
	args := m.Called(ctx, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.SigningPolicy), args.Error(1)
}

func (m *MockMetadataStoreSolana) ListUserAuthKeys(ctx context.Context, keyID string) ([]*storage.UserAuthKey, error) {
	args := m.Called(ctx, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.UserAuthKey), args.Error(1)
}

// Backup Delivery Operations
func (m *MockMetadataStoreSolana) SaveBackupShareDelivery(ctx context.Context, delivery *storage.BackupShareDelivery) error {
	args := m.Called(ctx, delivery)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) GetBackupShareDelivery(ctx context.Context, keyID, userID, nodeID string, shareIndex int) (*storage.BackupShareDelivery, error) {
	args := m.Called(ctx, keyID, userID, nodeID, shareIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.BackupShareDelivery), args.Error(1)
}

func (m *MockMetadataStoreSolana) UpdateBackupShareDeliveryStatus(ctx context.Context, keyID, userID, nodeID string, shareIndex int, status string, reason string) error {
	args := m.Called(ctx, keyID, userID, nodeID, shareIndex, status, reason)
	return args.Error(0)
}

func (m *MockMetadataStoreSolana) ListBackupShareDeliveries(ctx context.Context, keyID, nodeID string) ([]*storage.BackupShareDelivery, error) {
	args := m.Called(ctx, keyID, nodeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.BackupShareDelivery), args.Error(1)
}

func TestService_GenerateAddress_Solana(t *testing.T) {
	// Setup
	mockMetadataStore := new(MockMetadataStoreSolana)
	service := NewService(mockMetadataStore, nil, nil, nil, nil)

	// Test Data
	keyID := "test-key-solana"
	// Valid Ed25519 public key (32 bytes)
	pubKeyHex := "3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29"
	pubKeyBytes, _ := hex.DecodeString(pubKeyHex)
	expectedAddress := base58.Encode(pubKeyBytes)

	// Mock GetKeyMetadata
	mockMetadataStore.On("GetKeyMetadata", mock.Anything, keyID).Return(&storage.KeyMetadata{
		KeyID:     keyID,
		PublicKey: pubKeyHex,
		Algorithm: "EdDSA",
		Curve:     "Ed25519",
		ChainType: "solana",
		Status:    "Active",
	}, nil)

	// Mock UpdateKeyMetadata
	mockMetadataStore.On("UpdateKeyMetadata", mock.Anything, mock.MatchedBy(func(m *storage.KeyMetadata) bool {
		return m.Address == expectedAddress && m.ChainType == "solana"
	})).Return(nil)

	// Execute
	ctx := context.Background()
	address, err := service.GenerateAddress(ctx, keyID, "solana")

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, expectedAddress, address)
	mockMetadataStore.AssertExpectations(t)
}

func TestService_GenerateAddress_Solana_Alias(t *testing.T) {
	// Setup
	mockMetadataStore := new(MockMetadataStoreSolana)
	service := NewService(mockMetadataStore, nil, nil, nil, nil)

	// Test Data
	keyID := "test-key-solana-alias"
	pubKeyHex := "3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29"
	pubKeyBytes, _ := hex.DecodeString(pubKeyHex)
	expectedAddress := base58.Encode(pubKeyBytes)

	// Mock GetKeyMetadata
	mockMetadataStore.On("GetKeyMetadata", mock.Anything, keyID).Return(&storage.KeyMetadata{
		KeyID:     keyID,
		PublicKey: pubKeyHex,
		Algorithm: "EdDSA",
		Curve:     "Ed25519",
		ChainType: "solana",
		Status:    "Active",
	}, nil)

	// Mock UpdateKeyMetadata
	mockMetadataStore.On("UpdateKeyMetadata", mock.Anything, mock.MatchedBy(func(m *storage.KeyMetadata) bool {
		return m.Address == expectedAddress && m.ChainType == "solana"
	})).Return(nil)

	// Execute
	ctx := context.Background()
	address, err := service.GenerateAddress(ctx, keyID, "sol") // Test alias "sol"

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, expectedAddress, address)
	mockMetadataStore.AssertExpectations(t)
}
