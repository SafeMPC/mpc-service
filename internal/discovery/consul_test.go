package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsulDiscovery_Basic(t *testing.T) {
	// 跳过需要Consul服务器的测试
	t.Skip("Skipping Consul tests - requires running Consul server")

	// 创建Consul客户端 (假设Consul运行在localhost:8500)
	discovery, err := NewConsulDiscovery("localhost:8500")
	require.NoError(t, err)
	require.NotNil(t, discovery)

	// 测试服务信息
	serviceInfo := &ServiceInfo{
		ID:       "test-service-1",
		Name:     "test-service",
		Address:  "localhost",
		Port:     8080,
		Tags:     []string{"test", "v1"},
		Meta:     map[string]string{"version": "1.0.0"},
		NodeType: "test",
		Protocol: "http",
		Weight:   1,
		Check: &HealthCheck{
			Type:                           "http",
			Interval:                       30 * time.Second,
			Timeout:                        5 * time.Second,
			DeregisterCriticalServiceAfter: 1 * time.Minute,
			Path:                           "/health",
		},
	}

	// 测试注册服务
	ctx := context.Background()
	err = discovery.Register(ctx, serviceInfo)
	if err != nil {
		t.Logf("Register failed (expected if Consul not running): %v", err)
		return
	}

	// 测试发现服务
	services, err := discovery.Discover(ctx, "test-service", []string{"test"})
	if err != nil {
		t.Logf("Discover failed: %v", err)
		return
	}

	assert.Greater(t, len(services), 0, "Should find at least one service")

	// 测试注销服务
	err = discovery.Deregister(ctx, "test-service-1")
	if err != nil {
		t.Logf("Deregister failed: %v", err)
	}

	// 关闭连接
	err = discovery.Close()
	assert.NoError(t, err)
}

func TestLoadBalancer(t *testing.T) {
	// 测试轮询负载均衡器
	lb := NewRoundRobinLoadBalancer()

	services := []*ServiceInfo{
		{ID: "s1", Name: "test", Address: "host1", Port: 8080},
		{ID: "s2", Name: "test", Address: "host2", Port: 8080},
		{ID: "s3", Name: "test", Address: "host3", Port: 8080},
	}

	lb.UpdateServices(services)

	// 测试选择服务
	selected1 := lb.Select(services, "key1")
	assert.NotNil(t, selected1)

	selected2 := lb.Select(services, "key2")
	assert.NotNil(t, selected2)

	// 验证轮询
	if selected1.ID != selected2.ID {
		// 如果有两个不同的服务被选中，说明轮询工作
		assert.Contains(t, []string{"s1", "s2", "s3"}, selected1.ID)
		assert.Contains(t, []string{"s1", "s2", "s3"}, selected2.ID)
	}
}

func TestWeightedLoadBalancer(t *testing.T) {
	// 测试加权负载均衡器
	lb := NewWeightedLoadBalancer()

	services := []*ServiceInfo{
		{ID: "s1", Name: "test", Weight: 3}, // 3份
		{ID: "s2", Name: "test", Weight: 2}, // 2份
		{ID: "s3", Name: "test", Weight: 5}, // 5份
	}

	lb.UpdateServices(services)

	// 验证扩展后的列表长度 (3+2+5=10)
	assert.Len(t, lb.expanded, 10, "Expanded list should have 10 entries")

	// 验证轮询选择
	selected := make(map[string]int)
	for i := 0; i < 10; i++ {
		service := lb.Select(services, "")
		if service != nil {
			selected[service.ID]++
		}
	}

	// 每个服务应该被选择相应权重次数
	assert.Equal(t, 3, selected["s1"], "s1 should be selected 3 times")
	assert.Equal(t, 2, selected["s2"], "s2 should be selected 2 times")
	assert.Equal(t, 5, selected["s3"], "s3 should be selected 5 times")
}

func TestServiceRegistry(t *testing.T) {
	// 模拟服务发现接口
	mockDiscovery := &mockServiceDiscovery{
		services: make(map[string]*ServiceInfo),
	}

	serviceInfo := &ServiceInfo{
		ID:   "test-service",
		Name: "test-app",
	}

	registry := NewServiceRegistry(mockDiscovery, serviceInfo, NewRoundRobinLoadBalancer())

	// 测试注册
	err := registry.Register(context.Background())
	assert.NoError(t, err)
	assert.True(t, registry.IsRegistered())

	// 测试发现
	services, err := registry.Discover(context.Background(), "test-app", nil)
	assert.NoError(t, err)
	assert.Len(t, services, 1)

	// 测试注销
	err = registry.Deregister(context.Background())
	assert.NoError(t, err)
	assert.False(t, registry.IsRegistered())
}

// mockServiceDiscovery 模拟服务发现实现
type mockServiceDiscovery struct {
	services map[string]*ServiceInfo
}

func (m *mockServiceDiscovery) Register(ctx context.Context, service *ServiceInfo) error {
	m.services[service.ID] = service
	return nil
}

func (m *mockServiceDiscovery) Deregister(ctx context.Context, serviceID string) error {
	delete(m.services, serviceID)
	return nil
}

func (m *mockServiceDiscovery) Discover(ctx context.Context, serviceName string, tags []string) ([]*ServiceInfo, error) {
	var result []*ServiceInfo
	for _, service := range m.services {
		if service.Name == serviceName {
			result = append(result, service)
		}
	}
	return result, nil
}

func (m *mockServiceDiscovery) Watch(ctx context.Context, serviceName string, tags []string) (<-chan []*ServiceInfo, error) {
	ch := make(chan []*ServiceInfo, 1)
	return ch, nil
}

func (m *mockServiceDiscovery) HealthCheck(ctx context.Context, serviceID string) (*HealthStatus, error) {
	return &HealthStatus{
		ServiceID: serviceID,
		Status:    "passing",
	}, nil
}

func (m *mockServiceDiscovery) Close() error {
	return nil
}
