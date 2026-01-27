package api

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/dropbox/godropbox/time2"
	"github.com/SafeMPC/mpc-service/internal/auth"
	"github.com/SafeMPC/mpc-service/internal/config"
	"github.com/SafeMPC/mpc-service/internal/i18n"
	"github.com/SafeMPC/mpc-service/internal/infra/service"
	"github.com/SafeMPC/mpc-service/internal/infra/discovery"
	"github.com/SafeMPC/mpc-service/internal/infra/key"
	"github.com/SafeMPC/mpc-service/internal/infra/session"
	"github.com/SafeMPC/mpc-service/internal/infra/signing"
	"github.com/SafeMPC/mpc-service/internal/infra/storage"
	"github.com/SafeMPC/mpc-service/internal/infra/webauthn"
	"github.com/SafeMPC/mpc-service/internal/infra/websocket"
	"github.com/SafeMPC/mpc-service/internal/mailer"
	mpcgrpc "github.com/SafeMPC/mpc-service/internal/mpc/grpc"
	"github.com/SafeMPC/mpc-service/internal/mpc/node"
	"github.com/SafeMPC/mpc-service/internal/persistence"
	"github.com/SafeMPC/mpc-service/internal/push"
	"github.com/SafeMPC/mpc-service/internal/push/provider"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// PROVIDERS - define here only providers that for various reasons (e.g. cyclic dependency) can't live in their corresponding packages
// or for wrapping providers that only accept sub-configs to prevent the requirements for defining providers for sub-configs.
// https://github.com/google/wire/blob/main/docs/guide.md#defining-providers

// NewPush creates an instance of the push service and registers the configured push providers.
func NewPush(cfg config.Server, db *sql.DB) (*push.Service, error) {
	pusher := push.New(db)

	if cfg.Push.UseFCMProvider {
		fcmProvider, err := provider.NewFCM(cfg.FCMConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create FCM provider: %w", err)
		}
		pusher.RegisterProvider(fcmProvider)
	}

	if cfg.Push.UseMockProvider {
		log.Warn().Msg("Initializing mock push provider")
		mockProvider := provider.NewMock(push.ProviderTypeFCM)
		pusher.RegisterProvider(mockProvider)
	}

	if pusher.GetProviderCount() < 1 {
		log.Warn().Msg("No providers registered for push service")
	}

	return pusher, nil
}

func NewClock(t ...*testing.T) time2.Clock {
	var clock time2.Clock

	useMock := len(t) > 0 && t[0] != nil

	if useMock {
		clock = time2.NewMockClock(time.Now())
	} else {
		clock = time2.DefaultClock
	}

	return clock
}

func NewAuthService(config config.Server, db *sql.DB, clock time2.Clock) *auth.Service {
	return auth.NewService(config, db, clock)
}

func NewMailer(config config.Server) (*mailer.Mailer, error) {
	return mailer.NewWithConfig(config.Mailer, config.SMTP)
}

func NewDB(config config.Server) (*sql.DB, error) {
	return persistence.NewDB(config.Database)
}

func NewI18N(config config.Server) (*i18n.Service, error) {
	return i18n.New(config.I18n)
}

func NoTest() []*testing.T {
	return nil
}

func NewMetadataStore(db *sql.DB) storage.MetadataStore {
	return storage.NewPostgreSQLStore(db)
}

// NewWebAuthnServiceProvider 创建 WebAuthn 服务
func NewWebAuthnServiceProvider(cfg config.Server, metadataStore storage.MetadataStore) (*webauthn.Service, error) {
	// 从环境变量或配置获取 WebAuthn 配置
	// 如果没有配置，使用默认值（仅用于开发环境）
	rpID := "localhost" // TODO: 从配置读取
	rpName := "SafeMPC"
	rpOrigin := "http://localhost:8080"
	
	return webauthn.NewService(rpID, rpName, rpOrigin, metadataStore)
}

// NewWebSocketServerProvider 创建 WebSocket 服务器
func NewWebSocketServerProvider(grpcClient *mpcgrpc.GRPCClient) *websocket.Server {
	return websocket.NewServer(grpcClient)
}

func NewRedisClient(cfg config.Server) (*redis.Client, error) {
	if cfg.MPC.RedisEndpoint == "" {
		return nil, fmt.Errorf("MPC RedisEndpoint is not configured")
	}

	client := redis.NewClient(&redis.Options{
		Addr: cfg.MPC.RedisEndpoint,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}

func NewSessionStore(client *redis.Client) storage.SessionStore {
	return storage.NewRedisStore(client)
}

func NewKeyShareStorage(cfg config.Server) (storage.KeyShareStorage, error) {
	if cfg.MPC.KeyShareStoragePath == "" {
		return nil, fmt.Errorf("MPC KeyShareStoragePath is not configured")
	}
	if cfg.MPC.KeyShareEncryptionKey == "" {
		return nil, fmt.Errorf("MPC KeyShareEncryptionKey is not configured")
	}
	return storage.NewFileSystemKeyShareStorage(cfg.MPC.KeyShareStoragePath, cfg.MPC.KeyShareEncryptionKey)
}

func NewMPCGRPCClient(cfg config.Server, nodeManager *node.Manager) (*mpcgrpc.GRPCClient, error) {
	return mpcgrpc.NewGRPCClient(cfg, nodeManager)
}

func NewNodeManager(discoveryService *discovery.Service, cfg config.Server) *node.Manager {
	heartbeat := time.Duration(cfg.MPC.SessionTimeout)
	if heartbeat <= 0 {
		heartbeat = 30
	}
	return node.NewManager(discoveryService, heartbeat*time.Second)
}

func NewNodeRegistry(manager *node.Manager) *node.Registry {
	return node.NewRegistry(manager)
}

// NewMPCDiscoveryService 创建 MPC 服务发现服务
func NewMPCDiscoveryService(cfg config.Server) (*discovery.Service, error) {
	consulClient, err := discovery.NewConsulClient(&discovery.ConsulConfig{
		Address: cfg.MPC.ConsulAddress,
		Token:   "",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return discovery.NewService(consulClient), nil
}

func NewNodeDiscovery(manager *node.Manager, discoveryService *discovery.Service) *node.Discovery {
	return node.NewDiscovery(manager, discoveryService)
}

func NewSessionManager(metadataStore storage.MetadataStore, sessionStore storage.SessionStore, cfg config.Server) *session.Manager {
	timeout := time.Duration(cfg.MPC.SessionTimeout)
	if timeout <= 0 {
		timeout = 300
	}
	return session.NewManager(metadataStore, sessionStore, timeout*time.Second)
}

func NewDKGServiceProvider(
	metadataStore storage.MetadataStore,
	keyShareStorage storage.KeyShareStorage,
	nodeManager *node.Manager,
	nodeDiscovery *node.Discovery,
	grpcClient *mpcgrpc.GRPCClient, // 用于 Service 触发 Signer StartDKG
	cfg config.Server,
) *key.DKGService {
	// Service 节点不执行协议计算，DKGService 只负责协调
	return key.NewDKGService(metadataStore, keyShareStorage, nodeManager, nodeDiscovery, grpcClient)
}

func NewKeyServiceProvider(
	metadataStore storage.MetadataStore,
	keyShareStorage storage.KeyShareStorage,
	dkgService *key.DKGService,
) *key.Service {
	return key.NewService(metadataStore, keyShareStorage, dkgService)
}


func NewSigningServiceProvider(keyService *key.Service, sessionManager *session.Manager, nodeDiscovery *node.Discovery, cfg config.Server, grpcClient *mpcgrpc.GRPCClient, metadataStore storage.MetadataStore) *signing.Service {
	defaultProtocol := cfg.MPC.DefaultProtocol
	if defaultProtocol == "" {
		defaultProtocol = "gg20"
	}
	return signing.NewService(keyService, sessionManager, nodeDiscovery, defaultProtocol, grpcClient, metadataStore)
}

func NewMPCServiceProvider(
	cfg config.Server,
	keyService *key.Service,
	sessionManager *session.Manager,
	nodeDiscovery *node.Discovery,
	grpcClient *mpcgrpc.GRPCClient,
	metadataStore storage.MetadataStore,
) *service.Service {
	defaultProtocol := cfg.MPC.DefaultProtocol
	if defaultProtocol == "" {
		defaultProtocol = "gg20"
	}
	// service.Service 需要 GRPCClient 接口，mpcgrpc.GRPCClient 实现了该接口
	// 记录配置的 NodeID（用于调试）
	nodeID := cfg.MPC.NodeID
	log.Error().
		Str("mpc_node_id", nodeID).
		Bool("is_empty", nodeID == "").
		Str("mpc_node_type", cfg.MPC.NodeType).
		Msg("NewMPCServiceProvider: creating MPC service with NodeID")

	return service.NewService(keyService, sessionManager, nodeDiscovery, defaultProtocol, grpcClient, nodeID, metadataStore)
}

// ✅ 删除旧的 internal/grpc 相关 providers（已废弃，已统一到 internal/mpc/grpc）
// 统一使用 internal/mpc/grpc 作为唯一的 gRPC 实现
