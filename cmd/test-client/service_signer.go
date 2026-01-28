package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	pb "github.com/SafeMPC/mpc-service/pb/mpc/v1"
)

// TestServiceToSignerCommunication 测试 Service -> Signer 通信（模拟 Service 的 gRPC 客户端）
func TestServiceToSignerCommunication(ctx context.Context, signerEndpoint string) error {
	log.Info().Str("endpoint", signerEndpoint).Bool("tls", *tlsEnabled).Msg("Testing Service -> Signer communication (simulating Service gRPC client)...")

	// 创建 gRPC 连接（使用共享的 createGRPCConnection 函数，支持 TLS）
	conn, err := createGRPCConnection(signerEndpoint)
	if err != nil {
		return fmt.Errorf("failed to connect to signer: %w", err)
	}
	defer conn.Close()

	// 创建 gRPC 客户端（模拟 Service 使用的 SignerServiceClient）
	client := pb.NewSignerServiceClient(conn)

	// 调用 Ping（模拟 Service 调用 Signer）
	pingReq := &pb.PingRequest{
		FromService: "mpc-service-1",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	log.Info().Msg("Calling Ping via Service gRPC client (simulated)...")
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pingResp, err := client.Ping(pingCtx, pingReq)
	if err != nil {
		return fmt.Errorf("ping via service client failed: %w", err)
	}

	log.Info().
		Bool("alive", pingResp.Alive).
		Str("node_id", pingResp.NodeId).
		Str("timestamp", pingResp.Timestamp).
		Msg("Service -> Signer communication successful!")

	return nil
}
