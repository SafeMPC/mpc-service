package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/SafeMPC/mpc-service/internal/infra/discovery"
	"github.com/SafeMPC/mpc-service/internal/infra/session"
	pb "github.com/SafeMPC/mpc-service/pb/mpc/v1"
)

type ManagementServer struct {
	pb.UnimplementedManagementServiceServer
	discovery *discovery.Service
	sessions  *session.Manager
}

func NewManagementServer(discovery *discovery.Service, sessions *session.Manager) *ManagementServer {
	return &ManagementServer{
		discovery: discovery,
		sessions:  sessions,
	}
}

// RegisterNode Signer 节点注册
func (s *ManagementServer) RegisterNode(ctx context.Context, req *pb.RegisterNodeRequest) (*pb.RegisterNodeResponse, error) {
	log.Info().
		Str("node_id", req.NodeId).
		Str("endpoint", req.Endpoint).
		Strs("capabilities", req.Capabilities).
		Msg("Received RegisterNode request")

	// 注册到 Consul (或者 DB)
	// 这里假设 nodeType 为 "signer"
	// Parse endpoint to address and port?
	// For now, let's assume we store the full endpoint in Address field or Meta

	// discovery.RegisterNode expects address and port separately.
	// We might need to adjust discovery service or parse the endpoint.
	// For simplicity, let's assume endpoint is "host:port"

	address := req.Endpoint
	port := 9091 // Default? Or parse from string

	// 简单的解析 endpoint (host:port)
	if host, portStr, err := net.SplitHostPort(req.Endpoint); err == nil {
		address = host
		fmt.Sscanf(portStr, "%d", &port)
	}

	err := s.discovery.RegisterNode(ctx, req.NodeId, "signer", address, port)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register node")
		return &pb.RegisterNodeResponse{
			Registered: false,
			Message:    err.Error(),
		}, nil
	}

	return &pb.RegisterNodeResponse{
		Registered: true,
		ExpiresAt:  time.Now().Add(1 * time.Minute).Unix(), // TTL
	}, nil
}

// ReportResult Signer 上报结果
func (s *ManagementServer) ReportResult(ctx context.Context, req *pb.ReportResultRequest) (*pb.ReportResultResponse, error) {
	log.Info().
		Str("session_id", req.SessionId).
		Str("node_id", req.NodeId).
		Str("result_type", req.ResultType).
		Msg("Received ReportResult request")

	if req.Error != "" {
		log.Warn().
			Str("session_id", req.SessionId).
			Str("node_id", req.NodeId).
			Str("result_type", req.ResultType).
			Str("error", req.Error).
			Msg("Received failed result report")
		if req.ResultType == "DKG_PUBKEY" {
			_ = s.sessions.FailKeygenSession(ctx, req.SessionId)
		} else {
			_ = s.sessions.FailSession(ctx, req.SessionId)
		}
		return &pb.ReportResultResponse{Received: true}, nil
	}

	switch req.ResultType {
	case "DKG_PUBKEY":
		if err := s.sessions.CompleteKeygenSession(ctx, req.SessionId, req.Data); err != nil {
			log.Error().Err(err).Str("session_id", req.SessionId).Msg("Failed to complete keygen session")
			return nil, err
		}
	default:
		if err := s.sessions.CompleteSession(ctx, req.SessionId, req.Data); err != nil {
			log.Error().Err(err).Str("session_id", req.SessionId).Msg("Failed to complete signing session")
			return nil, err
		}
	}

	return &pb.ReportResultResponse{
		Received: true,
	}, nil
}

// Heartbeat 节点心跳
func (s *ManagementServer) Heartbeat(ctx context.Context, req *pb.ServiceHeartbeatRequest) (*pb.ServiceHeartbeatResponse, error) {
	// 刷新节点注册 TTL
	// TODO: Implement TTL refresh
	return &pb.ServiceHeartbeatResponse{
		Acknowledged: true,
	}, nil
}
