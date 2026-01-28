package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	baseURL        = flag.String("url", "http://localhost:8080", "Base URL of the MPC service")
	wsURL          = flag.String("ws-url", "ws://localhost:8080", "WebSocket URL of the MPC service")
	testType       = flag.String("test", "full", "Test type: webauthn, dkg, sign, full, ping")
	userID         = flag.String("user-id", "test-user", "User ID for testing")
	walletID       = flag.String("wallet-id", "", "Wallet ID for signing test")
	signerEndpoint = flag.String("signer-endpoint", "host.docker.internal:9091", "Signer node gRPC endpoint")
	tlsEnabled     = flag.Bool("tls", true, "Enable TLS for gRPC connections")
	tlsCertFile    = flag.String("tls-cert", "", "TLS client certificate file (optional, for mTLS)")
	tlsKeyFile     = flag.String("tls-key", "", "TLS client key file (optional, for mTLS)")
	tlsCACertFile  = flag.String("tls-ca", "/app/certs/ca.crt", "TLS CA certificate file (required for TLS)")
	verbose        = flag.Bool("verbose", false, "Enable verbose logging")
)

func main() {
	flag.Parse()

	// 设置日志级别
	if *verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Info().Msg("Received interrupt signal, shutting down...")
		cancel()
	}()

	// 创建测试客户端
	client := NewTestClient(*baseURL, *wsURL)

	// 根据测试类型执行相应的测试
	switch *testType {
	case "webauthn":
		if err := testWebAuthn(ctx, client); err != nil {
			log.Fatal().Err(err).Msg("WebAuthn test failed")
		}
	case "dkg":
		if err := testDKG(ctx, client); err != nil {
			log.Fatal().Err(err).Msg("DKG test failed")
		}
	case "sign":
		if *walletID == "" {
			log.Fatal().Msg("wallet-id is required for sign test")
		}
		if err := testSign(ctx, client, *walletID); err != nil {
			log.Fatal().Err(err).Msg("Sign test failed")
		}
	case "full":
		if err := testFullFlow(ctx, client); err != nil {
			log.Fatal().Err(err).Msg("Full flow test failed")
		}
	case "ping":
		// 测试直接连接 Signer
		if err := TestSignerPing(ctx, *signerEndpoint); err != nil {
			log.Fatal().Err(err).Msg("Direct Ping test failed")
		}
		// 测试通过 Service 客户端连接 Signer（模拟 Service -> Signer 通信）
		log.Info().Msg("Testing Service -> Signer communication (simulated)...")
		if err := TestServiceToSignerCommunication(ctx, *signerEndpoint); err != nil {
			log.Fatal().Err(err).Msg("Service -> Signer communication test failed")
		}
	default:
		log.Fatal().Str("test-type", *testType).Msg("Unknown test type")
	}

	log.Info().Msg("All tests completed successfully")
}

// testWebAuthn 测试 WebAuthn 注册和登录
func testWebAuthn(ctx context.Context, client *TestClient) error {
	log.Info().Msg("=== Testing WebAuthn ===")

	// 测试注册
	log.Info().Msg("Testing WebAuthn registration...")
	if err := client.TestWebAuthnRegistration(ctx, *userID); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// 测试登录
	log.Info().Msg("Testing WebAuthn login...")
	token, err := client.TestWebAuthnLogin(ctx, *userID)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	log.Info().Str("token", token[:20]+"...").Msg("Login successful, token received")
	return nil
}

// testDKG 测试 DKG 流程
func testDKG(ctx context.Context, client *TestClient) error {
	log.Info().Msg("=== Testing DKG (Wallet Creation) ===")

	// 测试环境：跳过登录，使用空 token（后端已允许跳过 WebAuthn 验证）
	// 生产环境：必须先登录获取有效 token
	token := "test-token-skip-auth" // 测试用的 token，后端会跳过验证
	
	log.Info().Msg("Creating wallet (DKG)...")
	walletID, err := client.TestCreateWallet(ctx, token)
	if err != nil {
		return fmt.Errorf("create wallet failed: %w", err)
	}

	log.Info().Str("wallet_id", walletID).Msg("Wallet created successfully")
	return nil
}

// testSign 测试签名流程
func testSign(ctx context.Context, client *TestClient, walletID string) error {
	log.Info().Msg("=== Testing Sign Transaction ===")

	// 清理 walletID（移除可能的 ANSI 转义字符）
	walletID = strings.TrimSpace(walletID)
	walletID = strings.Trim(walletID, "\"")
	
	// 测试环境：跳过登录，使用空 token（后端已允许跳过 WebAuthn 验证）
	// 生产环境：必须先登录获取有效 token
	token := "test-token-skip-auth" // 测试用的 token，后端会跳过验证

	// 签名交易
	log.Info().Str("wallet_id", walletID).Msg("Signing transaction...")
	signature, err := client.TestSignTransaction(ctx, token, walletID)
	if err != nil {
		return fmt.Errorf("sign transaction failed: %w", err)
	}

	log.Info().Str("signature", signature).Msg("Transaction signed successfully")
	return nil
}

// testFullFlow 测试完整流程
func testFullFlow(ctx context.Context, client *TestClient) error {
	log.Info().Msg("=== Testing Full Flow ===")

	// 1. WebAuthn 注册
	log.Info().Msg("Step 1: WebAuthn Registration")
	if err := client.TestWebAuthnRegistration(ctx, *userID); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// 2. WebAuthn 登录
	log.Info().Msg("Step 2: WebAuthn Login")
	token, err := client.TestWebAuthnLogin(ctx, *userID)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// 3. 创建钱包（DKG）
	log.Info().Msg("Step 3: Create Wallet (DKG)")
	walletID, err := client.TestCreateWallet(ctx, token)
	if err != nil {
		return fmt.Errorf("create wallet failed: %w", err)
	}

	// 4. 签名交易
	log.Info().Msg("Step 4: Sign Transaction")
	signature, err := client.TestSignTransaction(ctx, token, walletID)
	if err != nil {
		return fmt.Errorf("sign transaction failed: %w", err)
	}

	log.Info().
		Str("wallet_id", walletID).
		Str("signature", signature).
		Msg("Full flow test completed successfully")

	return nil
}
