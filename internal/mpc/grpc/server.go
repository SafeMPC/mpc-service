package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/SafeMPC/mpc-service/internal/config"
	"github.com/SafeMPC/mpc-service/internal/infra/session"
	"github.com/SafeMPC/mpc-service/internal/infra/storage"
	pb "github.com/SafeMPC/mpc-service/pb/mpc/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// GRPCServer gRPC服务端
// 在 V3 架构中，Service 节点提供 ManagementService 供 Signer 调用
type GRPCServer struct {
	sessionManager  *session.Manager
	keyShareStorage storage.KeyShareStorage
	metadataStore   storage.MetadataStore
	nodeID          string
	cfg             *ServerConfig

	managementServer *ManagementServer // V3: 管理服务实现

	// gRPC 服务器实例
	grpcServer *grpc.Server
	listener   net.Listener
}

// ServerConfig gRPC服务端配置
type ServerConfig struct {
	Port          int
	TLSEnabled    bool
	TLSCertFile   string
	TLSKeyFile    string
	TLSCACertFile string
	MaxConnAge    time.Duration
	KeepAlive     time.Duration
}

// NewGRPCServer 创建gRPC服务端
func NewGRPCServer(
	cfg config.Server,
	sessionManager *session.Manager,
	keyShareStorage storage.KeyShareStorage,
	metadataStore storage.MetadataStore,
	nodeID string,
	managementServer *ManagementServer, // V3: 注入管理服务
) *GRPCServer {
	serverCfg := &ServerConfig{
		Port:          cfg.MPC.GRPCPort,
		TLSEnabled:    cfg.MPC.TLSEnabled,
		TLSCertFile:   cfg.MPC.TLSCertFile,
		TLSKeyFile:    cfg.MPC.TLSKeyFile,
		TLSCACertFile: cfg.MPC.TLSCACertFile,
		MaxConnAge:    2 * time.Hour,
		KeepAlive:     30 * time.Second,
	}

	srv := &GRPCServer{
		sessionManager:   sessionManager,
		keyShareStorage:  keyShareStorage,
		metadataStore:    metadataStore,
		nodeID:           nodeID,
		cfg:              serverCfg,
		managementServer: managementServer,
	}

	return srv
}

// GetServerOptions 获取gRPC服务器选项
func (s *GRPCServer) GetServerOptions() ([]grpc.ServerOption, error) {
	var opts []grpc.ServerOption

	// TLS配置
	if s.cfg.TLSEnabled {
		creds, err := credentials.NewServerTLSFromFile(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load TLS credentials")
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// KeepAlive配置
	opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionAge:      s.cfg.MaxConnAge,
		MaxConnectionAgeGrace: 30 * time.Second,
		Time:                  s.cfg.KeepAlive,
		Timeout:               20 * time.Second,
	}))

	// Enforcement Policy
	opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime:             10 * time.Second,
		PermitWithoutStream: true,
	}))

	return opts, nil
}

// Start 启动 gRPC 服务器
func (s *GRPCServer) Start(ctx context.Context) error {
	// TLS 证书检查
	if s.cfg.TLSEnabled {
		if _, err := os.Stat(s.cfg.TLSCertFile); err != nil {
			return errors.Wrapf(err, "TLS certificate file not found: %s", s.cfg.TLSCertFile)
		}
		if _, err := os.Stat(s.cfg.TLSKeyFile); err != nil {
			return errors.Wrapf(err, "TLS key file not found: %s", s.cfg.TLSKeyFile)
		}
		log.Info().Msg("TLS certificate files verified")
	}

	addr := fmt.Sprintf(":%d", s.cfg.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener

	// 创建 gRPC 服务器实例
	opts, _ := s.GetServerOptions()
	s.grpcServer = grpc.NewServer(opts...)

	// 注册 ManagementService
	pb.RegisterManagementServiceServer(s.grpcServer, s.managementServer)

	// 启用反射（开发环境）
	reflection.Register(s.grpcServer)

	log.Info().
		Str("address", addr).
		Bool("tls", s.cfg.TLSEnabled).
		Msg("Starting Management gRPC server")

	// 在 goroutine 中启动服务器
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			log.Error().Err(err).Msg("Management gRPC server failed")
		}
	}()

	// 等待上下文取消
	<-ctx.Done()
	return s.Stop()
}

// Stop 停止 gRPC 服务器
func (s *GRPCServer) Stop() error {
	log.Info().Msg("Stopping Management gRPC server")

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	return nil
}
