package analysis

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/proto"
	"github.com/sirupsen/logrus"
)

// MockCodeAnalysisService 代码分析服务的mock实现
type MockCodeAnalysisService struct {
	resourceManager *resource.Manager
}

// NewMockCodeAnalysisService 创建新的mock代码分析服务
func NewMockCodeAnalysisService(resourceManager *resource.Manager) *MockCodeAnalysisService {
	return &MockCodeAnalysisService{
		resourceManager: resourceManager,
	}
}

// AnalyzeCode 分析代码并返回组件DAG图
func (s *MockCodeAnalysisService) AnalyzeCode(ctx context.Context, req *proto.CodeAnalysisRequest) (*proto.CodeAnalysisResponse, error) {
	logrus.Infof("Starting code analysis for application: %s", req.ApplicationId)

	// 解码代码内容（在实际实现中，这里会是真正的代码分析逻辑）
	codeContent, err := base64.StdEncoding.DecodeString(req.CodeContent)
	if err != nil {
		return &proto.CodeAnalysisResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to decode code content: %v", err),
		}, nil
	}

	// Mock分析逻辑：根据代码内容和元数据生成组件
	codeMap := map[string]string{"content": string(codeContent)}
	appMap := map[string]string{"id": req.ApplicationId}
	components, edges := s.generateMockComponents(appMap, codeMap, req.Metadata)

	// 为组件分配合适的provider
	s.assignProvidersToComponents(components, req.AvailableProviders)

	// 生成全局配置
	globalConfig := s.generateGlobalConfig(req.ApplicationId, req.Metadata)

	// 生成分析元数据
	analysisMetadata := fmt.Sprintf(`{
		"analysis_time": "%s",
		"detected_language": "%s",
		"framework": "%s",
		"components_count": %d,
		"complexity_score": "medium"
	}`, time.Now().Format(time.RFC3339), req.Metadata["language"], req.Metadata["framework"], len(components))

	return &proto.CodeAnalysisResponse{
		Success:          true,
		Components:       components,
		Edges:            edges,
		GlobalConfig:     globalConfig,
		AnalysisMetadata: analysisMetadata,
	}, nil
}

// generateMockComponents 生成复杂的mock Actor组件DAG（返回多层架构的测试数据）
func (s *MockCodeAnalysisService) generateMockComponents(appID, codeContent, metadata map[string]string) ([]*proto.Component, []*proto.DAGEdge) {
	appIDStr := appID["id"]

	// 创建复杂的多层Actor组件架构
	components := []*proto.Component{
		// Gateway层 - 网关代理Actor
		{
			Id:    fmt.Sprintf("%s-gateway", appIDStr),
			Name:  "API Gateway",
			Type:  "gateway",
			Image: "nginx:alpine",
			Ports: []int32{80, 443},
			Environment: map[string]string{
				"NGINX_HOST": "localhost",
				"NGINX_PORT": "80",
				"SSL_ENABLED": "true",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     0.5,
				Memory:  0.5,
				Gpu:     0,
				Storage: 2.0,
			},
			ProviderType: "docker",
		},
		// Web层 - Web服务Actor
		{
			Id:    fmt.Sprintf("%s-web-frontend", appIDStr),
			Name:  "Frontend Web Service",
			Type:  "web",
			Image: "node:18-alpine",
			Ports: []int32{3000},
			Environment: map[string]string{
				"NODE_ENV": "production",
				"PORT": "3000",
				"API_BASE_URL": "http://user-api:4000",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     1.0,
				Memory:  1.0,
				Gpu:     0,
				Storage: 5.0,
			},
			ProviderType: "docker",
		},
		// API层 - 用户服务Actor
		{
			Id:    fmt.Sprintf("%s-user-api", appIDStr),
			Name:  "User Management API",
			Type:  "api",
			Image: "node:18-alpine",
			Ports: []int32{4000},
			Environment: map[string]string{
				"NODE_ENV": "production",
				"PORT": "4000",
				"DB_HOST": "postgres",
				"REDIS_URL": "redis://cache:6379",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     1.5,
				Memory:  2.0,
				Gpu:     0,
				Storage: 8.0,
			},
			ProviderType: "docker",
		},
		// API层 - 订单服务Actor
		{
			Id:    fmt.Sprintf("%s-order-api", appIDStr),
			Name:  "Order Management API",
			Type:  "api",
			Image: "openjdk:11-jre-slim",
			Ports: []int32{4001},
			Environment: map[string]string{
				"SPRING_PROFILES_ACTIVE": "production",
				"SERVER_PORT": "4001",
				"DATABASE_URL": "jdbc:postgresql://postgres:5432/orders",
				"MESSAGE_QUEUE_URL": "amqp://rabbitmq:5672",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     2.0,
				Memory:  3.0,
				Gpu:     0,
				Storage: 10.0,
			},
			ProviderType: "docker",
		},
		// Worker层 - 邮件处理Actor
		{
			Id:    fmt.Sprintf("%s-email-worker", appIDStr),
			Name:  "Email Processing Worker",
			Type:  "worker",
			Image: "python:3.9-slim",
			Ports: []int32{},
			Environment: map[string]string{
				"WORKER_TYPE": "email",
				"QUEUE_URL": "amqp://rabbitmq:5672",
				"SMTP_HOST": "smtp.gmail.com",
				"SMTP_PORT": "587",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     0.5,
				Memory:  1.0,
				Gpu:     0,
				Storage: 3.0,
			},
			ProviderType: "docker",
		},
		// Worker层 - 报表生成Actor
		{
			Id:    fmt.Sprintf("%s-report-worker", appIDStr),
			Name:  "Report Generation Worker",
			Type:  "worker",
			Image: "python:3.9-slim",
			Ports: []int32{},
			Environment: map[string]string{
				"WORKER_TYPE": "report",
				"QUEUE_URL": "amqp://rabbitmq:5672",
				"OUTPUT_FORMAT": "pdf,excel",
				"STORAGE_PATH": "/app/reports",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     1.0,
				Memory:  2.0,
				Gpu:     0,
				Storage: 15.0,
			},
			ProviderType: "docker",
		},
		// Compute层 - 数据分析Actor
		{
			Id:    fmt.Sprintf("%s-analytics-compute", appIDStr),
			Name:  "Data Analytics Engine",
			Type:  "compute",
			Image: "python:3.9-slim",
			Ports: []int32{5000},
			Environment: map[string]string{
				"COMPUTE_TYPE": "analytics",
				"SPARK_MASTER": "local[*]",
				"DATA_SOURCE": "postgresql://postgres:5432/analytics",
				"CACHE_ENABLED": "true",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     4.0,
				Memory:  8.0,
				Gpu:     1,
				Storage: 50.0,
			},
			ProviderType: "k8s",
		},
		// Compute层 - 机器学习Actor
		{
			Id:    fmt.Sprintf("%s-ml-compute", appIDStr),
			Name:  "Machine Learning Service",
			Type:  "compute",
			Image: "tensorflow/tensorflow:latest-gpu",
			Ports: []int32{5001},
			Environment: map[string]string{
				"COMPUTE_TYPE": "ml",
				"MODEL_PATH": "/app/models",
				"GPU_ENABLED": "true",
				"BATCH_SIZE": "32",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     2.0,
				Memory:  16.0,
				Gpu:     2,
				Storage: 100.0,
			},
			ProviderType: "k8s",
		},
	}

	// 创建复杂的连接关系网络
	edges := []*proto.DAGEdge{
		// Gateway -> Web Frontend
		{
			FromComponent:  fmt.Sprintf("%s-gateway", appIDStr),
			ToComponent:    fmt.Sprintf("%s-web-frontend", appIDStr),
			ConnectionType: "http",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-web-frontend", appIDStr),
				"port": "3000",
				"path": "/",
				"method": "GET,POST",
			},
		},
		// Gateway -> User API
		{
			FromComponent:  fmt.Sprintf("%s-gateway", appIDStr),
			ToComponent:    fmt.Sprintf("%s-user-api", appIDStr),
			ConnectionType: "http",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-user-api", appIDStr),
				"port": "4000",
				"path": "/api/users",
				"method": "GET,POST,PUT,DELETE",
			},
		},
		// Gateway -> Order API
		{
			FromComponent:  fmt.Sprintf("%s-gateway", appIDStr),
			ToComponent:    fmt.Sprintf("%s-order-api", appIDStr),
			ConnectionType: "http",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-order-api", appIDStr),
				"port": "4001",
				"path": "/api/orders",
				"method": "GET,POST,PUT,DELETE",
			},
		},
		// Web Frontend -> User API
		{
			FromComponent:  fmt.Sprintf("%s-web-frontend", appIDStr),
			ToComponent:    fmt.Sprintf("%s-user-api", appIDStr),
			ConnectionType: "http",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-user-api", appIDStr),
				"port": "4000",
				"timeout": "30s",
			},
		},
		// Web Frontend -> Order API
		{
			FromComponent:  fmt.Sprintf("%s-web-frontend", appIDStr),
			ToComponent:    fmt.Sprintf("%s-order-api", appIDStr),
			ConnectionType: "http",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-order-api", appIDStr),
				"port": "4001",
				"timeout": "30s",
			},
		},
		// Order API -> Email Worker (异步消息)
		{
			FromComponent:  fmt.Sprintf("%s-order-api", appIDStr),
			ToComponent:    fmt.Sprintf("%s-email-worker", appIDStr),
			ConnectionType: "queue",
			ConnectionConfig: map[string]string{
				"queue_name": "email_notifications",
				"exchange": "orders",
				"routing_key": "order.created",
			},
		},
		// Order API -> Report Worker (异步消息)
		{
			FromComponent:  fmt.Sprintf("%s-order-api", appIDStr),
			ToComponent:    fmt.Sprintf("%s-report-worker", appIDStr),
			ConnectionType: "queue",
			ConnectionConfig: map[string]string{
				"queue_name": "report_generation",
				"exchange": "orders",
				"routing_key": "order.completed",
			},
		},
		// User API -> Analytics Compute (数据流)
		{
			FromComponent:  fmt.Sprintf("%s-user-api", appIDStr),
			ToComponent:    fmt.Sprintf("%s-analytics-compute", appIDStr),
			ConnectionType: "stream",
			ConnectionConfig: map[string]string{
				"stream_name": "user_events",
				"format": "json",
				"batch_size": "1000",
			},
		},
		// Order API -> Analytics Compute (数据流)
		{
			FromComponent:  fmt.Sprintf("%s-order-api", appIDStr),
			ToComponent:    fmt.Sprintf("%s-analytics-compute", appIDStr),
			ConnectionType: "stream",
			ConnectionConfig: map[string]string{
				"stream_name": "order_events",
				"format": "json",
				"batch_size": "500",
			},
		},
		// Analytics Compute -> ML Compute (计算管道)
		{
			FromComponent:  fmt.Sprintf("%s-analytics-compute", appIDStr),
			ToComponent:    fmt.Sprintf("%s-ml-compute", appIDStr),
			ConnectionType: "grpc",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-ml-compute", appIDStr),
				"port": "5001",
				"service": "MLPredictionService",
				"method": "Predict",
			},
		},
		// ML Compute -> User API (预测结果反馈)
		{
			FromComponent:  fmt.Sprintf("%s-ml-compute", appIDStr),
			ToComponent:    fmt.Sprintf("%s-user-api", appIDStr),
			ConnectionType: "http",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-user-api", appIDStr),
				"port": "4000",
				"path": "/api/ml/predictions",
				"method": "POST",
			},
		},
	}

	return components, edges
}

// generateDockerImage 根据语言和框架生成Docker镜像名
func (s *MockCodeAnalysisService) generateDockerImage(language, framework, componentType string) string {
	switch strings.ToLower(language) {
	case "javascript", "typescript", "node.js":
		if framework == "next.js" {
			return "node:18-alpine"
		}
		return "node:18-alpine"
	case "python":
		if framework == "django" {
			return "python:3.9-slim"
		} else if framework == "flask" {
			return "python:3.9-slim"
		}
		return "python:3.9-slim"
	case "java":
		if framework == "spring" {
			return "openjdk:11-jre-slim"
		}
		return "openjdk:11-jre-slim"
	case "go":
		return "golang:1.19-alpine"
	default:
		return "ubuntu:22.04"
	}
}

// assignProvidersToComponents 为组件分配合适的provider
func (s *MockCodeAnalysisService) assignProvidersToComponents(components []*proto.Component, availableProviders []*proto.ProviderInfo) {
	// 简单的分配策略：优先使用Docker provider
	var dockerProviders []*proto.ProviderInfo
	var k8sProviders []*proto.ProviderInfo

	for _, provider := range availableProviders {
		if provider.Type == "docker" {
			dockerProviders = append(dockerProviders, provider)
		} else if provider.Type == "k8s" {
			k8sProviders = append(k8sProviders, provider)
		}
	}

	// 为每个组件分配provider
	for i, component := range components {
		if len(dockerProviders) > 0 {
			// 轮询分配Docker provider
			provider := dockerProviders[i%len(dockerProviders)]
			component.ProviderId = provider.Id
			component.ProviderType = "docker"
		} else if len(k8sProviders) > 0 {
			// 轮询分配K8s provider
			provider := k8sProviders[i%len(k8sProviders)]
			component.ProviderId = provider.Id
			component.ProviderType = "k8s"
		}
	}
}

// generateGlobalConfig 生成全局配置
func (s *MockCodeAnalysisService) generateGlobalConfig(appID string, metadata map[string]string) map[string]string {
	return map[string]string{
		"APP_NAME":    appID,
		"APP_VERSION": "1.0.0",
		"ENVIRONMENT": "production",
		"LOG_LEVEL":   "info",
		"TIMEZONE":    "UTC",
		"LANGUAGE":    metadata["language"],
		"FRAMEWORK":   metadata["framework"],
	}
}
