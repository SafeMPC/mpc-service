package grpc

import (
	"net/http"
	"time"

	"github.com/kashguard/go-mpc-wallet/internal/config"
	pb "github.com/kashguard/go-mpc-wallet/internal/pb/mpc/v1"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	server   *Server
	config   *config.Server
	started  time.Time
	lastPing time.Time
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(server *Server, cfg *config.Server) *HealthChecker {
	return &HealthChecker{
		server:   server,
		config:   cfg,
		started:  time.Now(),
		lastPing: time.Now(),
	}
}

// RegisterRoutes 注册健康检查路由
func (h *HealthChecker) RegisterRoutes(e *echo.Echo) {
	// 基础健康检查
	e.GET("/health", h.healthCheck)
	e.GET("/health/live", h.livenessCheck)
	e.GET("/health/ready", h.readinessCheck)

	// 详细健康检查
	e.GET("/health/detailed", h.detailedHealthCheck)

	// Ping端点
	e.GET("/ping", h.ping)
}

// healthCheck 基础健康检查
func (h *HealthChecker) healthCheck(c echo.Context) error {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Since(h.started).String(),
	}

	// 检查gRPC服务器状态
	if h.server != nil && h.server.grpcServer != nil {
		status["grpc"] = "running"
	} else {
		status["grpc"] = "not_running"
		status["status"] = "degraded"
	}

	return c.JSON(http.StatusOK, status)
}

// livenessCheck 存活检查
func (h *HealthChecker) livenessCheck(c echo.Context) error {
	// 检查进程是否存活（如果能到达这里，说明进程存活）
	status := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	return c.JSON(http.StatusOK, status)
}

// readinessCheck 就绪检查
func (h *HealthChecker) readinessCheck(c echo.Context) error {
	status := "ready"
	httpStatus := http.StatusOK

	// 检查gRPC服务器是否已启动
	if h.server == nil || h.server.grpcServer == nil {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	// 检查关键服务是否可用
	if h.server != nil {
		// TODO: 检查数据库连接
		// TODO: 检查Redis连接
		// TODO: 检查Consul连接
	}

	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	return c.JSON(httpStatus, response)
}

// detailedHealthCheck 详细健康检查
func (h *HealthChecker) detailedHealthCheck(c echo.Context) error {
	health := map[string]interface{}{
		"status":       "ok",
		"timestamp":    time.Now().Format(time.RFC3339),
		"uptime":       time.Since(h.started).String(),
		"last_ping":    h.lastPing.Format(time.RFC3339),
		"version":      "v1.0.0", // TODO: 从配置获取
		"node_type":    h.config.MPC.NodeType,
		"node_id":      h.config.MPC.NodeID,
	}

	// 检查各个组件
	components := map[string]interface{}{}

	// gRPC服务器
	if h.server != nil && h.server.grpcServer != nil {
		components["grpc_server"] = map[string]interface{}{
			"status": "healthy",
			"port":   h.config.MPC.GRPCPort,
		}
	} else {
		components["grpc_server"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  "gRPC server not started",
		}
		health["status"] = "degraded"
	}

	// 数据库连接
	// TODO: 实现数据库健康检查
	components["database"] = map[string]interface{}{
		"status": "unknown", // TODO: 实现检查
	}

	// Redis连接
	// TODO: 实现Redis健康检查
	components["redis"] = map[string]interface{}{
		"status": "unknown", // TODO: 实现检查
	}

	// 服务发现
	// TODO: 实现Consul健康检查
	components["service_discovery"] = map[string]interface{}{
		"status": "unknown", // TODO: 实现检查
	}

	// MPC服务
	if h.server != nil {
		mpcServices := map[string]interface{}{}

		// 检查各个MPC服务是否已设置（通过比较是否为默认的Unimplemented服务）
		nodeServiceConfigured := true
		if _, ok := h.server.nodeService.(pb.UnimplementedMPCNodeServer); ok {
			nodeServiceConfigured = false
		}

		coordServiceConfigured := true
		if _, ok := h.server.coordService.(pb.UnimplementedMPCCoordinatorServer); ok {
			coordServiceConfigured = false
		}

		regServiceConfigured := true
		if _, ok := h.server.regService.(pb.UnimplementedMPCRegistryServer); ok {
			regServiceConfigured = false
		}

		mpcServices["node_service"] = map[string]interface{}{
			"configured": nodeServiceConfigured,
		}
		mpcServices["coordinator_service"] = map[string]interface{}{
			"configured": coordServiceConfigured,
		}
		mpcServices["registry_service"] = map[string]interface{}{
			"configured": regServiceConfigured,
		}

		components["mpc_services"] = mpcServices
	}

	health["components"] = components

	return c.JSON(http.StatusOK, health)
}

// ping Ping检查
func (h *HealthChecker) ping(c echo.Context) error {
	h.lastPing = time.Now()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   "pong",
		"timestamp": h.lastPing.Format(time.RFC3339),
	})
}

// UpdateComponentHealth 更新组件健康状态
func (h *HealthChecker) UpdateComponentHealth(component string, healthy bool, details map[string]interface{}) {
	// TODO: 实现组件健康状态更新
	// 这可以用于动态更新健康检查结果
	log.Debug().
		Str("component", component).
		Bool("healthy", healthy).
		Interface("details", details).
		Msg("Component health updated")
}

// GetHealthSummary 获取健康摘要
func (h *HealthChecker) GetHealthSummary() map[string]interface{} {
	return map[string]interface{}{
		"status":    "ok",
		"uptime":    time.Since(h.started).String(),
		"timestamp": time.Now().Format(time.RFC3339),
	}
}
