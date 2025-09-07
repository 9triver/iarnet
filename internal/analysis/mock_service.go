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

// generateMockComponents 生成mock组件（返回固定的测试数据）
func (s *MockCodeAnalysisService) generateMockComponents(appID, codeContent, metadata map[string]string) ([]*proto.Component, []*proto.DAGEdge) {
	appIDStr := appID["id"]

	// 返回固定的组件结构
	components := []*proto.Component{
		{
			Id:    fmt.Sprintf("%s-web", appIDStr),
			Name:  "Web Application",
			Type:  "web",
			Image: "nginx:latest",
			Ports: []int32{80},
			Environment: map[string]string{
				"ENV": "production",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     1.0,
				Memory:  1.0,
				Gpu:     0,
				Storage: 10.0,
			},
			ProviderType: "docker",
		},
		{
			Id:    fmt.Sprintf("%s-api", appIDStr),
			Name:  "API Server",
			Type:  "api",
			Image: "node:16-alpine",
			Ports: []int32{3000},
			Environment: map[string]string{
				"NODE_ENV": "production",
				"PORT":     "3000",
			},
			Resources: &proto.ResourceRequirements{
				Cpu:     0.5,
				Memory:  0.5,
				Gpu:     0,
				Storage: 5.0,
			},
			ProviderType: "docker",
		},
	}

	// 返回固定的连接关系
	edges := []*proto.DAGEdge{
		{
			FromComponent:  fmt.Sprintf("%s-web", appIDStr),
			ToComponent:    fmt.Sprintf("%s-api", appIDStr),
			ConnectionType: "http",
			ConnectionConfig: map[string]string{
				"host": fmt.Sprintf("%s-api", appIDStr),
				"port": "3000",
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
