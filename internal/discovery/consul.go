package discovery

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/rs/zerolog/log"
)

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	// 注册服务
	Register(ctx context.Context, service *ServiceInfo) error

	// 注销服务
	Deregister(ctx context.Context, serviceID string) error

	// 发现服务
	Discover(ctx context.Context, serviceName string, tags []string) ([]*ServiceInfo, error)

	// 监听服务变化
	Watch(ctx context.Context, serviceName string, tags []string) (<-chan []*ServiceInfo, error)

	// 健康检查
	HealthCheck(ctx context.Context, serviceID string) (*HealthStatus, error)

	// 关闭连接
	Close() error
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	ID          string            // 服务实例ID
	Name        string            // 服务名称
	Address     string            // 服务地址
	Port        int               // 服务端口
	Tags        []string          // 服务标签
	Meta        map[string]string // 元数据
	Check       *HealthCheck      // 健康检查配置
	NodeType    string            // 节点类型 (coordinator, participant)
	Protocol    string            // 协议版本
	Weight      int               // 负载均衡权重
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	Type                           string // "http", "tcp", "grpc"
	Interval                       time.Duration
	Timeout                        time.Duration
	DeregisterCriticalServiceAfter time.Duration
	Path                           string // HTTP健康检查路径
}

// HealthStatus 健康状态
type HealthStatus struct {
	ServiceID string
	Status    string // "passing", "warning", "critical"
	Output    string
	Timestamp time.Time
}

// ConsulDiscovery Consul实现的服务发现
type ConsulDiscovery struct {
	client *api.Client
	config *api.Config
}

// NewConsulDiscovery 创建Consul服务发现实例
func NewConsulDiscovery(address string) (ServiceDiscovery, error) {
	config := api.DefaultConfig()
	config.Address = address

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &ConsulDiscovery{
		client: client,
		config: config,
	}, nil
}

// Register 注册服务到Consul
func (c *ConsulDiscovery) Register(ctx context.Context, service *ServiceInfo) error {
	if service.ID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}

	if service.Name == "" {
		service.Name = "mpc-" + service.NodeType
	}

	// 构建Consul服务注册信息
	registration := &api.AgentServiceRegistration{
		ID:      service.ID,
		Name:    service.Name,
		Address: service.Address,
		Port:    service.Port,
		Tags:    service.Tags,
		Meta:    service.Meta,
	}

	// 添加节点类型到标签
	if service.NodeType != "" {
		registration.Tags = append(registration.Tags, "node-type:"+service.NodeType)
	}

	// 添加协议版本到标签
	if service.Protocol != "" {
		registration.Tags = append(registration.Tags, "protocol:"+service.Protocol)
	}

	// 配置健康检查
	if service.Check != nil {
		check := &api.AgentServiceCheck{}

		switch service.Check.Type {
		case "http":
			check.HTTP = fmt.Sprintf("http://%s:%d%s", service.Address, service.Port, service.Check.Path)
			check.Method = "GET"
		case "tcp":
			check.TCP = fmt.Sprintf("%s:%d", service.Address, service.Port)
		case "grpc":
			check.GRPC = fmt.Sprintf("%s:%d", service.Address, service.Port)
		default:
			return fmt.Errorf("unsupported health check type: %s", service.Check.Type)
		}

		check.Interval = service.Check.Interval.String()
		check.Timeout = service.Check.Timeout.String()

		if service.Check.DeregisterCriticalServiceAfter > 0 {
			check.DeregisterCriticalServiceAfter = service.Check.DeregisterCriticalServiceAfter.String()
		}

		registration.Check = check
	}

	// 注册服务
	if err := c.client.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("failed to register service %s: %w", service.ID, err)
	}

	log.Info().
		Str("service_id", service.ID).
		Str("service_name", service.Name).
		Str("address", service.Address).
		Int("port", service.Port).
		Strs("tags", service.Tags).
		Msg("Service registered successfully")

	return nil
}

// Deregister 从Consul注销服务
func (c *ConsulDiscovery) Deregister(ctx context.Context, serviceID string) error {
	if err := c.client.Agent().ServiceDeregister(serviceID); err != nil {
		return fmt.Errorf("failed to deregister service %s: %w", serviceID, err)
	}

	log.Info().
		Str("service_id", serviceID).
		Msg("Service deregistered successfully")

	return nil
}

// Discover 从Consul发现服务
func (c *ConsulDiscovery) Discover(ctx context.Context, serviceName string, tags []string) ([]*ServiceInfo, error) {
	// 发现健康的服务实例
	services, _, err := c.client.Health().ServiceMultipleTags(serviceName, tags, true, &api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to discover services %s: %w", serviceName, err)
	}

	var result []*ServiceInfo
	for _, service := range services {
		info := &ServiceInfo{
			ID:       service.Service.ID,
			Name:     service.Service.Service,
			Address:  service.Service.Address,
			Port:     service.Service.Port,
			Tags:     service.Service.Tags,
			Meta:     service.Service.Meta,
			NodeType: extractNodeType(service.Service.Tags),
			Protocol: extractProtocol(service.Service.Tags),
		}

		// 解析权重
		if weightStr, ok := service.Service.Meta["weight"]; ok {
			if weight, err := strconv.Atoi(weightStr); err == nil {
				info.Weight = weight
			}
		}

		result = append(result, info)
	}

	log.Debug().
		Str("service_name", serviceName).
		Strs("tags", tags).
		Int("found_services", len(result)).
		Msg("Service discovery completed")

	return result, nil
}

// Watch 监听服务变化
func (c *ConsulDiscovery) Watch(ctx context.Context, serviceName string, tags []string) (<-chan []*ServiceInfo, error) {
	ch := make(chan []*ServiceInfo, 1)

	go func() {
		defer close(ch)

		var lastIndex uint64

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// 查询服务
			services, meta, err := c.client.Health().ServiceMultipleTags(serviceName, tags, true, &api.QueryOptions{
				WaitIndex: lastIndex,
				WaitTime:  30 * time.Second,
			})

			if err != nil {
				log.Error().
					Err(err).
					Str("service_name", serviceName).
					Msg("Failed to watch services")
				time.Sleep(5 * time.Second)
				continue
			}

			// 检查是否有变化
			if meta.LastIndex == lastIndex {
				continue
			}
			lastIndex = meta.LastIndex

			// 转换服务信息
			var result []*ServiceInfo
			for _, service := range services {
				info := &ServiceInfo{
					ID:       service.Service.ID,
					Name:     service.Service.Service,
					Address:  service.Service.Address,
					Port:     service.Service.Port,
					Tags:     service.Service.Tags,
					Meta:     service.Service.Meta,
					NodeType: extractNodeType(service.Service.Tags),
					Protocol: extractProtocol(service.Service.Tags),
				}
				result = append(result, info)
			}

			// 发送更新
			select {
			case ch <- result:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// HealthCheck 执行健康检查
func (c *ConsulDiscovery) HealthCheck(ctx context.Context, serviceID string) (*HealthStatus, error) {
	// 查询服务的健康检查
	checks, _, err := c.client.Health().Checks(serviceID, &api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get health checks for service %s: %w", serviceID, err)
	}

	// 找到对应的健康检查
	var check *api.HealthCheck
	for _, chk := range checks {
		if chk.ServiceID == serviceID {
			check = chk
			break
		}
	}

	if check == nil {
		return nil, fmt.Errorf("no health check found for service %s", serviceID)
	}

	return &HealthStatus{
		ServiceID: serviceID,
		Status:    check.Status,
		Output:    check.Output,
		Timestamp: time.Now(),
	}, nil
}

// Close 关闭Consul连接
func (c *ConsulDiscovery) Close() error {
	// Consul客户端通常不需要显式关闭
	return nil
}

// extractNodeType 从标签中提取节点类型
func extractNodeType(tags []string) string {
	for _, tag := range tags {
		if len(tag) > 10 && tag[:10] == "node-type:" {
			return tag[10:]
		}
	}
	return ""
}

// extractProtocol 从标签中提取协议版本
func extractProtocol(tags []string) string {
	for _, tag := range tags {
		if len(tag) > 9 && tag[:9] == "protocol:" {
			return tag[9:]
		}
	}
	return ""
}
