package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// loadTLSConfig 加载 TLS 配置
func loadTLSConfig() (*tls.Config, error) {
	// 加载 CA 证书（必需）
	caBytes, err := os.ReadFile(*tlsCACertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate from %s: %w", *tlsCACertFile, err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	// 加载客户端证书和私钥（可选，用于 mTLS）
	var certs []tls.Certificate
	if *tlsCertFile != "" && *tlsKeyFile != "" {
		// 检查文件是否存在
		if _, err := os.Stat(*tlsCertFile); os.IsNotExist(err) {
			log.Warn().
				Str("cert_file", *tlsCertFile).
				Msg("Client certificate file not found, skipping client certificate (using TLS without mTLS)")
		} else if _, err := os.Stat(*tlsKeyFile); os.IsNotExist(err) {
			log.Warn().
				Str("key_file", *tlsKeyFile).
				Msg("Client key file not found, skipping client certificate (using TLS without mTLS)")
		} else {
			clientCert, err := tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
			if err != nil {
				log.Warn().
					Err(err).
					Str("cert_file", *tlsCertFile).
					Str("key_file", *tlsKeyFile).
					Msg("Failed to load client certificate, continuing without client cert (using TLS without mTLS)")
			} else {
				certs = []tls.Certificate{clientCert}
				log.Debug().Msg("Client certificate loaded successfully (mTLS enabled)")
			}
		}
	}

	return &tls.Config{
		RootCAs:      caPool,
		Certificates: certs,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// createGRPCConnection 创建 gRPC 连接（支持 TLS）
func createGRPCConnection(endpoint string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	if *tlsEnabled {
		tlsCfg, err := loadTLSConfig()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load TLS config, falling back to insecure")
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// 添加超时和重试
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithTimeout(5*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", endpoint, err)
	}

	return conn, nil
}
