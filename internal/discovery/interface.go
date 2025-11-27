package discovery

import (
	"context"
	"time"
)

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	Provider      string        // "consul", "etcd", "kubernetes"
	Address       string        // 服务发现服务器地址
	Timeout       time.Duration // 超时时间
	RetryInterval time.Duration // 重试间隔
	MaxRetries    int           // 最大重试次数
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// 选择服务实例
	Select(services []*ServiceInfo, key string) *ServiceInfo

	// 更新服务列表
	UpdateServices(services []*ServiceInfo)
}

// RoundRobinLoadBalancer 轮询负载均衡器
type RoundRobinLoadBalancer struct {
	services []*ServiceInfo
	index    int
}

// NewRoundRobinLoadBalancer 创建轮询负载均衡器
func NewRoundRobinLoadBalancer() *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{
		services: make([]*ServiceInfo, 0),
		index:    0,
	}
}

// Select 选择服务实例（轮询）
func (lb *RoundRobinLoadBalancer) Select(services []*ServiceInfo, key string) *ServiceInfo {
	if len(services) == 0 {
		return nil
	}

	// 如果服务列表有变化，更新本地列表
	if len(services) != len(lb.services) {
		lb.services = make([]*ServiceInfo, len(services))
		copy(lb.services, services)
	}

	// 轮询选择
	service := lb.services[lb.index]
	lb.index = (lb.index + 1) % len(lb.services)

	return service
}

// UpdateServices 更新服务列表
func (lb *RoundRobinLoadBalancer) UpdateServices(services []*ServiceInfo) {
	lb.services = make([]*ServiceInfo, len(services))
	copy(lb.services, services)
}

// WeightedLoadBalancer 加权负载均衡器
type WeightedLoadBalancer struct {
	services   []*ServiceInfo
	expanded  []*ServiceInfo // 按权重扩展的服务列表
	nextIndex int
}

// NewWeightedLoadBalancer 创建加权负载均衡器
func NewWeightedLoadBalancer() *WeightedLoadBalancer {
	return &WeightedLoadBalancer{
		services:  make([]*ServiceInfo, 0),
		expanded: make([]*ServiceInfo, 0),
	}
}

// Select 选择服务实例（基于权重）
func (lb *WeightedLoadBalancer) Select(services []*ServiceInfo, key string) *ServiceInfo {
	if len(lb.expanded) == 0 {
		return nil
	}

	// 轮询选择
	service := lb.expanded[lb.nextIndex]
	lb.nextIndex = (lb.nextIndex + 1) % len(lb.expanded)

	return service
}

// UpdateServices 更新服务列表并按权重扩展
func (lb *WeightedLoadBalancer) UpdateServices(services []*ServiceInfo) {
	lb.services = make([]*ServiceInfo, len(services))
	copy(lb.services, services)

	// 按权重扩展服务列表
	lb.expanded = make([]*ServiceInfo, 0)
	for _, service := range services {
		weight := service.Weight
		if weight <= 0 {
			weight = 1 // 默认权重为1
		}

		// 根据权重重复添加服务实例
		for i := 0; i < weight; i++ {
			lb.expanded = append(lb.expanded, service)
		}
	}

	lb.nextIndex = 0
}

// ServiceRegistry 服务注册管理器
type ServiceRegistry struct {
	discovery     ServiceDiscovery
	config        *ServiceInfo
	registered    bool
	loadBalancer  LoadBalancer
}

// NewServiceRegistry 创建服务注册管理器
func NewServiceRegistry(discovery ServiceDiscovery, config *ServiceInfo, loadBalancer LoadBalancer) *ServiceRegistry {
	return &ServiceRegistry{
		discovery:    discovery,
		config:       config,
		loadBalancer: loadBalancer,
	}
}

// Register 注册服务
func (r *ServiceRegistry) Register(ctx context.Context) error {
	if r.registered {
		return nil
	}

	if err := r.discovery.Register(ctx, r.config); err != nil {
		return err
	}

	r.registered = true
	return nil
}

// Deregister 注销服务
func (r *ServiceRegistry) Deregister(ctx context.Context) error {
	if !r.registered {
		return nil
	}

	if err := r.discovery.Deregister(ctx, r.config.ID); err != nil {
		return err
	}

	r.registered = false
	return nil
}

// Discover 发现服务
func (r *ServiceRegistry) Discover(ctx context.Context, serviceName string, tags []string) ([]*ServiceInfo, error) {
	return r.discovery.Discover(ctx, serviceName, tags)
}

// SelectService 选择服务实例
func (r *ServiceRegistry) SelectService(serviceName string, tags []string, key string) (*ServiceInfo, error) {
	services, err := r.discovery.Discover(context.Background(), serviceName, tags)
	if err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, ErrNoServiceAvailable
	}

	r.loadBalancer.UpdateServices(services)
	return r.loadBalancer.Select(services, key), nil
}

// Watch 监听服务变化
func (r *ServiceRegistry) Watch(ctx context.Context, serviceName string, tags []string) (<-chan []*ServiceInfo, error) {
	return r.discovery.Watch(ctx, serviceName, tags)
}

// IsRegistered 检查是否已注册
func (r *ServiceRegistry) IsRegistered() bool {
	return r.registered
}

// Close 关闭注册管理器
func (r *ServiceRegistry) Close() error {
	if r.registered {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r.Deregister(ctx)
	}
	return r.discovery.Close()
}

// 错误定义
var (
	ErrNoServiceAvailable = NewDiscoveryError("no service available")
	ErrServiceNotFound   = NewDiscoveryError("service not found")
)

// DiscoveryError 服务发现错误
type DiscoveryError struct {
	Message string
}

func NewDiscoveryError(message string) *DiscoveryError {
	return &DiscoveryError{Message: message}
}

func (e *DiscoveryError) Error() string {
	return e.Message
}
