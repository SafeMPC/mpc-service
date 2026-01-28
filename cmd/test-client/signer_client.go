package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	pb "github.com/SafeMPC/mpc-service/pb/mpc/v1"
)

// TestSignerPing 测试 Signer 节点的 Ping RPC（直接连接）
func TestSignerPing(ctx context.Context, signerEndpoint string) error {
	log.Info().Str("endpoint", signerEndpoint).Bool("tls", *tlsEnabled).Msg("Testing Signer node Ping RPC (direct connection)...")

	// 创建 gRPC 连接（支持 TLS）
	conn, err := createGRPCConnection(signerEndpoint)
	if err != nil {
		return fmt.Errorf("failed to connect to signer: %w", err)
	}
	defer conn.Close()

	// 创建 gRPC 客户端
	client := pb.NewSignerServiceClient(conn)

	// 调用 Ping RPC
	pingReq := &pb.PingRequest{
		FromService: "test-client",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	log.Info().Msg("Calling Ping RPC...")
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pingResp, err := client.Ping(pingCtx, pingReq)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	log.Info().
		Bool("alive", pingResp.Alive).
		Str("node_id", pingResp.NodeId).
		Str("timestamp", pingResp.Timestamp).
		Msg("Ping successful!")

	return nil
}
