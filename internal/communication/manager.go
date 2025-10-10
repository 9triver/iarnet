package communication

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/resource"
)

// CommunicationType 通信类型
type CommunicationType string

const (
	CommunicationHTTP   CommunicationType = "http"
	CommunicationGRPC   CommunicationType = "grpc"
	CommunicationStream CommunicationType = "stream" // 流式通信
	CommunicationQueue  CommunicationType = "queue"
	CommunicationFile   CommunicationType = "file"
)

// ServiceEndpoint 服务端点
type ServiceEndpoint struct {
	ComponentID string            `json:"component_id"`
	ServiceName string            `json:"service_name"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Protocol    CommunicationType `json:"protocol"`
	HealthPath  string            `json:"health_path,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ServiceRoute 服务路由
type ServiceRoute struct {
	ID          string            `json:"id"`
	FromService string            `json:"from_service"`
	ToService   string            `json:"to_service"`
	Type        CommunicationType `json:"type"`
	Config      RouteConfig       `json:"config"`
	Status      RouteStatus       `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	Path         string            `json:"path,omitempty"`
	Method       string            `json:"method,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	Retries      int               `json:"retries,omitempty"`
	LoadBalancer string            `json:"load_balancer,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// RouteStatus 路由状态
type RouteStatus string

const (
	RouteStatusActive   RouteStatus = "active"
	RouteStatusInactive RouteStatus = "inactive"
	RouteStatusError    RouteStatus = "error"
)

// Manager 通信管理器
type Manager struct {
	endpoints       map[string]*ServiceEndpoint // componentID -> endpoint
	routes          map[string]*ServiceRoute    // routeID -> route
	serviceMap      map[string]string           // serviceName -> componentID
	resourceManager *resource.Manager
	mu              sync.RWMutex
}

// NewManager 创建通信管理器
func NewManager(resourceManager *resource.Manager) *Manager {
	return &Manager{
		endpoints:       make(map[string]*ServiceEndpoint),
		routes:          make(map[string]*ServiceRoute),
		serviceMap:      make(map[string]string),
		resourceManager: resourceManager,
	}
}

// RegisterServiceEndpoint 注册服务端点
func (m *Manager) RegisterServiceEndpoint(componentID string, component *application.Component) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取Actor组件的容器信息
	if component.ContainerRef == nil {
		return fmt.Errorf("component %s has no container reference", componentID)
	}

	// // 获取容器的网络信息
	// containerInfo, err := m.getContainerNetworkInfo(component.ContainerRef.ID, component.ProviderID)
	// if err != nil {
	// 	return fmt.Errorf("failed to get container network info: %v", err)
	// }

	// // 创建服务端点
	// for _, port := range component.Ports {
	// 	endpoint := &ServiceEndpoint{
	// 		ComponentID: componentID,
	// 		ServiceName: component.Name,
	// 		Host:        containerInfo.Host,
	// 		Port:        port,
	// 		Protocol:    m.inferProtocolFromComponent(component),
	// 		HealthPath:  m.getHealthPath(component),
	// 		Metadata: map[string]string{
	// 			"actor_type": string(component.Type),
	// 			"provider_id":    component.ProviderID,
	// 			"image":          component.Image,
	// 		},
	// 		CreatedAt: time.Now(),
	// 		UpdatedAt: time.Now(),
	// 	}

	// 	m.endpoints[componentID] = endpoint
	// 	m.serviceMap[component.Name] = componentID

	// 	logrus.Infof("Registered service endpoint: %s -> %s:%d", component.Name, containerInfo.Host, port)
	// }

	return nil
}

// // CreateServiceRoutes 创建服务路由
// func (m *Manager) CreateServiceRoutes(dag *application.ApplicationDAG) error {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()

// 	for _, edge := range dag.Edges {
// 		// 检查源和目标组件是否存在
// 		fromComponent := dag.GetComponent(edge.FromComponent)
// 		toComponent := dag.GetComponent(edge.ToComponent)
// 		if fromComponent == nil || toComponent == nil {
// 			logrus.Warnf("Skipping route creation: component not found (from: %s, to: %s)", edge.FromComponent, edge.ToComponent)
// 			continue
// 		}

// 		// 创建路由
// 		routeID := fmt.Sprintf("%s-%s-%s", edge.FromComponent, edge.ToComponent, edge.ConnectionType)
// 		route := &ServiceRoute{
// 			ID:          routeID,
// 			FromService: fromComponent.Name,
// 			ToService:   toComponent.Name,
// 			Type:        CommunicationType(edge.ConnectionType),
// 			Config:      m.createRouteConfig(string(edge.ConnectionType), fromComponent, toComponent),
// 			Status:      RouteStatusActive,
// 			CreatedAt:   time.Now(),
// 			UpdatedAt:   time.Now(),
// 		}

// 		m.routes[routeID] = route
// 		logrus.Infof("Created service route: %s -> %s (%s)", fromComponent.Name, toComponent.Name, edge.ConnectionType)
// 	}

// 	return nil
// }

// GetServiceEndpoint 获取服务端点
func (m *Manager) GetServiceEndpoint(componentID string) (*ServiceEndpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	endpoint, exists := m.endpoints[componentID]
	if !exists {
		return nil, fmt.Errorf("service endpoint not found for component: %s", componentID)
	}

	return endpoint, nil
}

// GetServiceEndpointByName 根据服务名获取端点
func (m *Manager) GetServiceEndpointByName(serviceName string) (*ServiceEndpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	componentID, exists := m.serviceMap[serviceName]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", serviceName)
	}

	return m.endpoints[componentID], nil
}

// GetServiceRoutes 获取所有服务路由
func (m *Manager) GetServiceRoutes() map[string]*ServiceRoute {
	m.mu.RLock()
	defer m.mu.RUnlock()

	routes := make(map[string]*ServiceRoute)
	for id, route := range m.routes {
		routes[id] = route
	}

	return routes
}

// UpdateRouteStatus 更新路由状态
func (m *Manager) UpdateRouteStatus(routeID string, status RouteStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	route, exists := m.routes[routeID]
	if !exists {
		return fmt.Errorf("route not found: %s", routeID)
	}

	route.Status = status
	route.UpdatedAt = time.Now()

	logrus.Infof("Updated route status: %s -> %s", routeID, status)
	return nil
}

// HealthCheckEndpoint 健康检查端点
func (m *Manager) HealthCheckEndpoint(componentID string) error {
	m.mu.RLock()
	endpoint, exists := m.endpoints[componentID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("endpoint not found for component: %s", componentID)
	}

	// 执行健康检查
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch endpoint.Protocol {
	case CommunicationHTTP:
		return m.httpHealthCheck(ctx, endpoint)
	case CommunicationGRPC:
		return m.grpcHealthCheck(ctx, endpoint)
	default:
		return m.tcpHealthCheck(ctx, endpoint)
	}
}

// UnregisterServiceEndpoint 注销服务端点
func (m *Manager) UnregisterServiceEndpoint(componentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	endpoint, exists := m.endpoints[componentID]
	if !exists {
		return fmt.Errorf("endpoint not found for component: %s", componentID)
	}

	// 删除服务映射
	delete(m.serviceMap, endpoint.ServiceName)
	// 删除端点
	delete(m.endpoints, componentID)

	// 删除相关路由
	for routeID, route := range m.routes {
		if route.FromService == endpoint.ServiceName || route.ToService == endpoint.ServiceName {
			delete(m.routes, routeID)
			logrus.Infof("Removed route: %s", routeID)
		}
	}

	logrus.Infof("Unregistered service endpoint: %s", endpoint.ServiceName)
	return nil
}

// 辅助方法

// getContainerNetworkInfo 获取容器网络信息
func (m *Manager) getContainerNetworkInfo(containerID, providerID string) (*ContainerNetworkInfo, error) {
	// 这里应该调用资源管理器获取容器的实际网络信息
	// 目前返回模拟数据
	return &ContainerNetworkInfo{
		Host:  "localhost", // 在实际实现中应该获取真实的容器IP
		Ports: []int{},
	}, nil
}

// ContainerNetworkInfo 容器网络信息
type ContainerNetworkInfo struct {
	Host  string `json:"host"`
	Ports []int  `json:"ports"`
}

// // inferProtocolFromComponent 从Actor组件推断协议类型
// func (m *Manager) inferProtocolFromComponent(component *application.Component) CommunicationType {
// 	switch component.Type {
// 	case application.ComponentTypeGateway:
// 		return CommunicationHTTP // 网关通常使用HTTP
// 	case application.ComponentTypeWeb:
// 		return CommunicationHTTP // Web服务使用HTTP
// 	case application.ComponentTypeAPI:
// 		return CommunicationHTTP // API服务使用HTTP
// 	case application.ComponentTypeWorker:
// 		return CommunicationQueue // Worker通常通过消息队列通信
// 	case application.ComponentTypeCompute:
// 		// 计算服务可能使用gRPC或流式通信
// 		if len(component.Ports) > 0 {
// 			port := component.Ports[0]
// 			if port >= 5000 && port <= 5999 {
// 				return CommunicationGRPC // 高性能计算服务使用gRPC
// 			}
// 		}
// 		return CommunicationStream // 默认流式通信
// 	default:
// 		// 根据镜像或端口推断
// 		if len(component.Ports) > 0 {
// 			port := component.Ports[0]
// 			if port == 80 || port == 443 || port == 8080 {
// 				return CommunicationHTTP
// 			} else if port >= 9000 && port <= 9999 {
// 				return CommunicationGRPC
// 			}
// 		}
// 		return CommunicationHTTP // 默认HTTP
// 	}
// }

// // getHealthPath 获取健康检查路径
// func (m *Manager) getHealthPath(component *application.Component) string {
// 	switch component.Type {
// 	case application.ComponentTypeWeb, application.ComponentTypeAPI:
// 		return "/health"
// 	case application.ComponentTypeWorker:
// 		return "/ping"
// 	default:
// 		return ""
// 	}
// }

// createRouteConfig 创建路由配置
func (m *Manager) createRouteConfig(edgeType string, from, to *application.Component) RouteConfig {
	config := RouteConfig{
		Timeout:      30 * time.Second,
		Retries:      3,
		LoadBalancer: "round_robin",
		Headers:      make(map[string]string),
	}

	switch edgeType {
	case "http":
		config.Method = "GET"
		config.Path = "/"
		config.Headers["Content-Type"] = "application/json"
	case "grpc":
		config.Headers["Content-Type"] = "application/grpc"
	case "database":
		config.Timeout = 60 * time.Second
	case "queue":
		config.Timeout = 10 * time.Second
		config.Retries = 5
	}

	return config
}

// httpHealthCheck HTTP健康检查
func (m *Manager) httpHealthCheck(ctx context.Context, endpoint *ServiceEndpoint) error {
	// 实现HTTP健康检查
	logrus.Debugf("HTTP health check for %s:%d%s", endpoint.Host, endpoint.Port, endpoint.HealthPath)
	// 这里应该实现实际的HTTP请求
	return nil
}

// grpcHealthCheck gRPC健康检查
func (m *Manager) grpcHealthCheck(ctx context.Context, endpoint *ServiceEndpoint) error {
	// 实现gRPC健康检查
	logrus.Debugf("gRPC health check for %s:%d", endpoint.Host, endpoint.Port)
	// 这里应该实现实际的gRPC健康检查
	return nil
}

// tcpHealthCheck TCP健康检查
func (m *Manager) tcpHealthCheck(ctx context.Context, endpoint *ServiceEndpoint) error {
	// 实现TCP连接检查
	address := net.JoinHostPort(endpoint.Host, fmt.Sprintf("%d", endpoint.Port))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("TCP health check failed for %s: %v", address, err)
	}
	conn.Close()

	logrus.Debugf("TCP health check passed for %s", address)
	return nil
}

// GetCommunicationStats 获取通信统计信息
func (m *Manager) GetCommunicationStats() *CommunicationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &CommunicationStats{
		TotalEndpoints: len(m.endpoints),
		TotalRoutes:    len(m.routes),
		ActiveRoutes:   0,
		ProtocolStats:  make(map[string]int),
	}

	// 统计活跃路由
	for _, route := range m.routes {
		if route.Status == RouteStatusActive {
			stats.ActiveRoutes++
		}
	}

	// 统计协议分布
	for _, endpoint := range m.endpoints {
		stats.ProtocolStats[string(endpoint.Protocol)]++
	}

	return stats
}

// CommunicationStats 通信统计信息
type CommunicationStats struct {
	TotalEndpoints int            `json:"total_endpoints"`
	TotalRoutes    int            `json:"total_routes"`
	ActiveRoutes   int            `json:"active_routes"`
	ProtocolStats  map[string]int `json:"protocol_stats"`
}
