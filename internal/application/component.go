package application

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/9triver/iarnet/internal/resource"
)

// ComponentType 定义组件类型 - 表示分布式部署的actor类型
type ComponentType string

const (
	ComponentTypeWeb     ComponentType = "web"     // Web服务actor
	ComponentTypeAPI     ComponentType = "api"     // API服务actor
	ComponentTypeWorker  ComponentType = "worker"  // 工作处理actor
	ComponentTypeCompute ComponentType = "compute" // 计算处理actor
	ComponentTypeGateway ComponentType = "gateway" // 网关代理actor
)

// ConnectionType 定义组件间连接类型
type ConnectionType string

const (
	ConnectionTypeHTTP         ConnectionType = "http"
	ConnectionTypeGRPC         ConnectionType = "grpc"
	ConnectionTypeStream       ConnectionType = "stream" // 流式连接
	ConnectionTypeMessageQueue ConnectionType = "message_queue"
)

// Component 表示应用的一个组件 - 可分布式部署的actor执行单元
type Component struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Type             ComponentType          `json:"type"`
	Image            string                 `json:"image"`
	Dependencies     []string               `json:"dependencies"` // 依赖的组件ID列表
	Ports            []int                  `json:"ports"`
	Environment      map[string]string      `json:"environment"`
	EnvVars          map[string]string      `json:"env_vars"` // 环境变量（别名）
	Resources        ResourceRequirements   `json:"resources"`
	ProviderType     string                 `json:"provider_type"`     // 首选的provider类型
	ProviderID       string                 `json:"provider_id"`       // 指定的provider ID
	DeploymentConfig map[string]interface{} `json:"deployment_config"` // 部署配置
	Status           ComponentStatus        `json:"status"`
	ContainerRef     *resource.ContainerRef `json:"container_ref,omitempty"`
	DeployedAt       *time.Time             `json:"deployed_at,omitempty"` // 部署时间
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// ComponentStatus 组件状态
type ComponentStatus string

const (
	ComponentStatusPending   ComponentStatus = "pending"
	ComponentStatusDeploying ComponentStatus = "deploying"
	ComponentStatusRunning   ComponentStatus = "running"
	ComponentStatusStopped   ComponentStatus = "stopped"
	ComponentStatusFailed    ComponentStatus = "failed"
)

// ResourceRequirements 资源需求
type ResourceRequirements struct {
	CPU     float64 `json:"cpu"`     // CPU核心数
	Memory  float64 `json:"memory"`  // 内存GB
	GPU     float64 `json:"gpu"`     // GPU数量
	Storage float64 `json:"storage"` // 存储GB
}

// DAGEdgeOld 表示DAG图中的边（组件间连接）
type DAGEdgeOld struct {
	FromComponent    string            `json:"from_component"`
	ToComponent      string            `json:"to_component"`
	ConnectionType   ConnectionType    `json:"connection_type"`
	ConnectionConfig map[string]string `json:"connection_config"`
}

// ApplicationDAG 表示应用的DAG图结构
type ApplicationDAG struct {
	ApplicationID    string                 `json:"application_id"`
	Components       map[string]*Component  `json:"components"` // 组件ID -> 组件
	Edges            []DAGEdgeOld           `json:"edges"`
	GlobalConfig     map[string]string      `json:"global_config"`     // 全局配置
	AnalysisMetadata map[string]interface{} `json:"analysis_metadata"` // 分析元数据
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// GetRootComponents 获取没有依赖的根组件
func (dag *ApplicationDAG) GetRootComponents() []*Component {
	var roots []*Component
	for _, component := range dag.Components {
		if len(component.Dependencies) == 0 {
			roots = append(roots, component)
		}
	}
	return roots
}

// GetDependentComponents 获取指定组件的所有依赖组件
func (dag *ApplicationDAG) GetDependentComponents(componentID string) []*Component {
	var dependents []*Component
	component, exists := dag.Components[componentID]
	if !exists {
		return dependents
	}

	for _, depID := range component.Dependencies {
		if depComponent, exists := dag.Components[depID]; exists {
			dependents = append(dependents, depComponent)
		}
	}
	return dependents
}

// GetComponentsByType 根据类型获取组件
func (dag *ApplicationDAG) GetComponentsByType(componentType ComponentType) []*Component {
	var components []*Component
	for _, component := range dag.Components {
		if component.Type == componentType {
			components = append(components, component)
		}
	}
	return components
}

// ValidateDAG 验证DAG图的有效性（检查循环依赖等）
func (dag *ApplicationDAG) ValidateDAG() error {
	// 使用DFS检测循环依赖
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for componentID := range dag.Components {
		if !visited[componentID] {
			if dag.hasCycleDFS(componentID, visited, recStack) {
				return fmt.Errorf("circular dependency detected involving component %s", componentID)
			}
		}
	}
	return nil
}

// hasCycleDFS 使用DFS检测循环依赖
func (dag *ApplicationDAG) hasCycleDFS(componentID string, visited, recStack map[string]bool) bool {
	visited[componentID] = true
	recStack[componentID] = true

	component, exists := dag.Components[componentID]
	if !exists {
		return false
	}

	for _, depID := range component.Dependencies {
		if !visited[depID] {
			if dag.hasCycleDFS(depID, visited, recStack) {
				return true
			}
		} else if recStack[depID] {
			return true
		}
	}

	recStack[componentID] = false
	return false
}

// ToJSON 将DAG转换为JSON字符串
func (dag *ApplicationDAG) ToJSON() (string, error) {
	data, err := json.MarshalIndent(dag, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从JSON字符串创建DAG
func (dag *ApplicationDAG) FromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), dag)
}

// GetDeploymentOrder 获取组件的部署顺序（拓扑排序）
func (dag *ApplicationDAG) GetDeploymentOrder() ([]*Component, error) {
	if err := dag.ValidateDAG(); err != nil {
		return nil, err
	}

	// 拓扑排序
	inDegree := make(map[string]int)
	for componentID := range dag.Components {
		inDegree[componentID] = 0
	}

	// 计算入度
	for _, component := range dag.Components {
		for _, depID := range component.Dependencies {
			inDegree[depID]++
		}
	}

	// 队列存储入度为0的节点
	queue := make([]*Component, 0)
	for componentID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, dag.Components[componentID])
		}
	}

	result := make([]*Component, 0)
	for len(queue) > 0 {
		// 取出队首元素
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// 更新依赖当前组件的其他组件的入度
		for _, component := range dag.Components {
			for _, depID := range component.Dependencies {
				if depID == current.ID {
					inDegree[component.ID]--
					if inDegree[component.ID] == 0 {
						queue = append(queue, component)
					}
				}
			}
		}
	}

	if len(result) != len(dag.Components) {
		return nil, fmt.Errorf("failed to generate deployment order, possible circular dependency")
	}

	return result, nil
}
