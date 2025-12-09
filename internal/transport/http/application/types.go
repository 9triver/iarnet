package application

import (
	"sort"
	"time"

	"github.com/9triver/iarnet/internal/domain/application/types"
	taskpkg "github.com/9triver/iarnet/internal/domain/execution/task"
)

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Name          string `json:"name" binding:"required"`       // 应用名称
	GitURL        string `json:"git_url" binding:"required"`    // Git 仓库地址
	Branch        string `json:"branch"`                        // Git 分支，默认为 "main"
	Description   string `json:"description"`                   // 应用描述
	ExecuteCmd    string `json:"execute_cmd"`                   // 执行命令
	EnvInstallCmd string `json:"env_install_cmd"`               // 环境安装命令
	RunnerEnv     string `json:"runner_env" binding:"required"` // 运行环境 (python/go/java)
}

// CreateApplicationResponse 创建应用响应
type CreateApplicationResponse struct {
	ID string `json:"id"` // 应用 ID
}

func (r *CreateApplicationResponse) FromAppID(appID types.AppID) *CreateApplicationResponse {
	r.ID = string(appID)
	return r
}

// GetApplicationListResponse 获取应用列表响应
type GetApplicationListResponse struct {
	Applications []ApplicationItem `json:"applications"` // 应用列表
	Total        int               `json:"total"`        // 总数
}

func (r *GetApplicationListResponse) FromAppMetadataArray(metadata []types.AppMetadata) *GetApplicationListResponse {
	applications := make([]ApplicationItem, 0, len(metadata))
	for _, metadata := range metadata {
		application := (&ApplicationItem{}).FromAppMetadata(metadata)
		applications = append(applications, *application)
	}
	r.Applications = applications
	r.Total = len(applications)
	return r
}

// ApplicationItem 应用列表项
type ApplicationItem struct {
	ID            string    `json:"id"`              // 应用 ID
	Name          string    `json:"name"`            // 应用名称
	Status        string    `json:"status"`          // 应用状态
	GitURL        string    `json:"git_url"`         // Git 仓库地址
	Branch        string    `json:"branch"`          // Git 分支
	Description   string    `json:"description"`     // 应用描述
	ContainerID   string    `json:"container_id"`    // 容器 ID（如果已部署）
	LastDeployed  time.Time `json:"last_deployed"`   // 最后部署时间
	ExecuteCmd    string    `json:"execute_cmd"`     // 执行命令
	EnvInstallCmd string    `json:"env_install_cmd"` // 环境安装命令
	RunnerEnv     string    `json:"runner_env"`      // 运行环境
}

func (r *ApplicationItem) FromAppMetadata(metadata types.AppMetadata) *ApplicationItem {
	r.ID = metadata.ID
	r.Name = metadata.Name
	r.Status = string(metadata.Status)
	r.GitURL = metadata.GitUrl
	r.Branch = metadata.Branch
	r.Description = metadata.Description
	r.ContainerID = metadata.ContainerID
	r.LastDeployed = metadata.LastDeployed
	r.ExecuteCmd = metadata.ExecuteCmd
	r.EnvInstallCmd = metadata.EnvInstallCmd
	r.RunnerEnv = metadata.RunnerEnv
	return r
}

// GetApplicationResponse 获取单个应用响应
type GetApplicationResponse struct {
	ID            string    `json:"id"`              // 应用 ID
	Name          string    `json:"name"`            // 应用名称
	Status        string    `json:"status"`          // 应用状态
	GitURL        string    `json:"git_url"`         // Git 仓库地址
	Branch        string    `json:"branch"`          // Git 分支
	Description   string    `json:"description"`     // 应用描述
	ContainerID   string    `json:"container_id"`    // 容器 ID（如果已部署）
	LastDeployed  time.Time `json:"last_deployed"`   // 最后部署时间
	ExecuteCmd    string    `json:"execute_cmd"`     // 执行命令
	EnvInstallCmd string    `json:"env_install_cmd"` // 环境安装命令
	RunnerEnv     string    `json:"runner_env"`      // 运行环境
	CreatedAt     time.Time `json:"created_at"`      // 创建时间（如果有）
	UpdatedAt     time.Time `json:"updated_at"`      // 更新时间（如果有）
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`   // 错误消息
	Code    string `json:"code"`    // 错误代码（可选）
	Message string `json:"message"` // 详细错误信息（可选）
}

// ApplicationStats 应用统计信息
type ApplicationStats struct {
	Total      int `json:"total"`      // 总应用数
	Running    int `json:"running"`    // 运行中的应用数
	Stopped    int `json:"stopped"`    // 已停止的应用数
	Undeployed int `json:"undeployed"` // 未部署的应用数
	Failed     int `json:"failed"`     // 失败的应用数
}

// RunnerEnvironment 运行环境
type RunnerEnvironment struct {
	Name string `json:"name"` // 环境名称
}

// GetRunnerEnvironmentsResponse 获取运行环境响应
type GetRunnerEnvironmentsResponse struct {
	Environments []string `json:"environments"` // 运行环境列表
}

// UpdateApplicationRequest 更新应用请求
type UpdateApplicationRequest struct {
	Name          *string `json:"name"`            // 应用名称（可选）
	GitURL        *string `json:"git_url"`         // Git 仓库地址（可选）
	Branch        *string `json:"branch"`          // Git 分支（可选）
	Description   *string `json:"description"`     // 应用描述（可选）
	ExecuteCmd    *string `json:"execute_cmd"`     // 执行命令（可选）
	EnvInstallCmd *string `json:"env_install_cmd"` // 环境安装命令（可选）
	RunnerEnv     *string `json:"runner_env"`      // 运行环境（可选）
}

// ToAppMetadata 将 CreateApplicationRequest 转换为领域层的 AppMetadata
func (req *CreateApplicationRequest) ToAppMetadata() types.AppMetadata {
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	return types.AppMetadata{
		Name:          req.Name,
		Status:        types.AppStatusUndeployed,
		GitUrl:        req.GitURL,
		Branch:        branch,
		Description:   req.Description,
		ExecuteCmd:    req.ExecuteCmd,
		EnvInstallCmd: req.EnvInstallCmd,
		RunnerEnv:     req.RunnerEnv,
	}
}

// FromAppMetadataToItem 将领域层的 AppMetadata 转换为 ApplicationItem
func ToApplicationItem(metadata types.AppMetadata) ApplicationItem {
	return ApplicationItem{
		ID:            metadata.ID,
		Name:          metadata.Name,
		Status:        string(metadata.Status),
		GitURL:        metadata.GitUrl,
		Branch:        metadata.Branch,
		Description:   metadata.Description,
		ContainerID:   metadata.ContainerID,
		LastDeployed:  metadata.LastDeployed,
		ExecuteCmd:    metadata.ExecuteCmd,
		EnvInstallCmd: metadata.EnvInstallCmd,
		RunnerEnv:     metadata.RunnerEnv,
	}
}

// FromAppMetadataToGetResponse 将领域层的 AppMetadata 转换为 GetApplicationResponse
func FromAppMetadataToGetResponse(metadata types.AppMetadata) GetApplicationResponse {
	return GetApplicationResponse{
		ID:            metadata.ID,
		Name:          metadata.Name,
		Status:        string(metadata.Status),
		GitURL:        metadata.GitUrl,
		Branch:        metadata.Branch,
		Description:   metadata.Description,
		ContainerID:   metadata.ContainerID,
		LastDeployed:  metadata.LastDeployed,
		ExecuteCmd:    metadata.ExecuteCmd,
		EnvInstallCmd: metadata.EnvInstallCmd,
		RunnerEnv:     metadata.RunnerEnv,
		CreatedAt:     metadata.LastDeployed, // 如果没有单独的 CreatedAt，使用 LastDeployed
		UpdatedAt:     metadata.LastDeployed, // 如果没有单独的 UpdatedAt，使用 LastDeployed
	}
}

func ToApplicationListResponse(metadata []types.AppMetadata) GetApplicationListResponse {
	applications := make([]ApplicationItem, 0, len(metadata))
	for _, metadata := range metadata {
		applications = append(applications, ToApplicationItem(metadata))
	}
	return GetApplicationListResponse{
		Applications: applications,
		Total:        len(applications),
	}
}

// 文件管理相关类型
type GetFileTreeRequest struct {
	Path string `json:"path"` // 文件路径，默认为 "/"
}

type GetFileTreeResponse struct {
	Files []types.FileInfo `json:"files"` // 文件列表
}

type GetFileContentRequest struct {
	Path string `json:"path"` // 文件路径
}

type GetFileContentResponse struct {
	Content  string `json:"content"`  // 文件内容
	Language string `json:"language"` // 文件语言类型
	Path     string `json:"path"`     // 文件路径
}

type SaveFileContentRequest struct {
	Content string `json:"content"` // 文件内容
}

type SaveFileContentResponse struct {
	Message  string `json:"message"`   // 响应消息
	FilePath string `json:"file_path"` // 文件路径
}

type CreateFileRequest struct {
	FilePath string `json:"filePath"` // 文件路径
}

type CreateFileResponse struct {
	Message  string `json:"message"`   // 响应消息
	FilePath string `json:"file_path"` // 文件路径
}

type DeleteFileRequest struct {
	FilePath string `json:"filePath"` // 文件路径
}

type DeleteFileResponse struct {
	Message  string `json:"message"`   // 响应消息
	FilePath string `json:"file_path"` // 文件路径
}

type CreateDirectoryRequest struct {
	DirPath string `json:"dirPath"` // 目录路径
}

type CreateDirectoryResponse struct {
	Message string `json:"message"`  // 响应消息
	DirPath string `json:"dir_path"` // 目录路径
}

type DeleteDirectoryRequest struct {
	DirPath string `json:"dirPath"` // 目录路径
}

type DeleteDirectoryResponse struct {
	Message string `json:"message"`  // 响应消息
	DirPath string `json:"dir_path"` // 目录路径
}

// DAG 相关类型

// GetApplicationDAGRequest 获取应用 DAG 请求
type GetApplicationDAGRequest struct {
	AppID     string `json:"app_id"`
	SessionID string `json:"session_id,omitempty"`
}

// ControlNodeResponse 控制节点响应
type ControlNodeResponse struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	FunctionName string            `json:"functionName"`
	Params       map[string]string `json:"params"`
}

// DataNodeResponse 数据节点响应
type DataNodeResponse struct {
	ID        string             `json:"id"`
	Status    string             `json:"status"`
	Lambda    string             `json:"lambda"`
	ObjectRef *DataNodeObjectRef `json:"object_ref,omitempty"` // 数据对象的引用（当数据就绪时）
}

// DataNodeObjectRef 数据节点对象引用
type DataNodeObjectRef struct {
	ID     string `json:"id,omitempty"`     // Object ID
	Source string `json:"source,omitempty"` // Source store ID
}

// DAGNodeResponse DAG 节点响应
type DAGNodeResponse struct {
	Type string      `json:"type"`
	Node interface{} `json:"node"`
}

// DAGEdgeResponse DAG 边响应
type DAGEdgeResponse struct {
	FromNodeID string `json:"fromNodeId"`
	ToNodeID   string `json:"toNodeId"`
	Info       string `json:"info,omitempty"`
}

// DAGResponse DAG 响应
type DAGResponse struct {
	Nodes []DAGNodeResponse `json:"nodes"`
	Edges []DAGEdgeResponse `json:"edges"`
}

// GetApplicationDAGResponse 获取应用 DAG 响应
type GetApplicationDAGResponse struct {
	DAG               DAGResponse `json:"dag"`
	Sessions          []string    `json:"sessions"`
	SelectedSessionID string      `json:"selected_session_id"`
}

// BuildGetApplicationDAGResponse 构建 DAG 响应
func BuildGetApplicationDAGResponse(dags map[string]*taskpkg.DAG, requestedSession string) GetApplicationDAGResponse {
	resp := GetApplicationDAGResponse{
		DAG: DAGResponse{
			Nodes: make([]DAGNodeResponse, 0),
			Edges: make([]DAGEdgeResponse, 0),
		},
		Sessions: make([]string, 0, len(dags)),
	}

	if len(dags) == 0 {
		resp.SelectedSessionID = ""
		return resp
	}

	sessionIDs := make([]string, 0, len(dags))
	for sessionID := range dags {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)
	resp.Sessions = sessionIDs

	selectedSession := requestedSession
	if selectedSession == "" || dags[selectedSession] == nil {
		if len(sessionIDs) > 0 {
			selectedSession = sessionIDs[len(sessionIDs)-1]
		} else {
			selectedSession = ""
		}
	}
	resp.SelectedSessionID = selectedSession

	if selectedSession == "" {
		return resp
	}

	edgeSet := make(map[string]struct{})
	addEdge := func(from, to, info string) {
		if from == "" || to == "" {
			return
		}
		key := from + "->" + to + ":" + info
		if _, exists := edgeSet[key]; exists {
			return
		}
		resp.DAG.Edges = append(resp.DAG.Edges, DAGEdgeResponse{
			FromNodeID: from,
			ToNodeID:   to,
			Info:       info,
		})
		edgeSet[key] = struct{}{}
	}

	dag, ok := dags[selectedSession]
	if !ok {
		return resp
	}

	for _, controlNode := range dag.ControlNodes {
		nodeResp := ControlNodeResponse{
			ID:           string(controlNode.ID),
			Status:       string(controlNode.Status),
			FunctionName: controlNode.FunctionName,
			Params:       copyStringMap(controlNode.Params),
		}

		resp.DAG.Nodes = append(resp.DAG.Nodes, DAGNodeResponse{
			Type: "ControlNode",
			Node: nodeResp,
		})
	}

	for _, dataNode := range dag.DataNodes {
		nodeResp := DataNodeResponse{
			ID:     string(dataNode.ID),
			Status: string(dataNode.Status),
			Lambda: dataNode.Lambda,
		}

		// 如果数据节点有 ObjectRef，添加到响应中
		if dataNode.ObjectRef != nil {
			nodeResp.ObjectRef = &DataNodeObjectRef{
				ID:     dataNode.ObjectRef.ID,
				Source: dataNode.ObjectRef.Source,
			}
		}

		resp.DAG.Nodes = append(resp.DAG.Nodes, DAGNodeResponse{
			Type: "DataNode",
			Node: nodeResp,
		})
	}

	for _, edge := range dag.Edges {
		addEdge(string(edge.From), string(edge.To), edge.Label)
	}

	return resp
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Actor 相关类型

// ActorComponentInfo Actor 组件信息
type ActorComponentInfo struct {
	ID            string        `json:"id,omitempty"`
	Image         string        `json:"image,omitempty"`
	ProviderID    string        `json:"provider_id,omitempty"`
	ResourceUsage *ResourceInfo `json:"resource_usage,omitempty"`
}

// ResourceInfo 资源信息
type ResourceInfo struct {
	CPU    int64 `json:"cpu,omitempty"`
	Memory int64 `json:"memory,omitempty"`
	GPU    int64 `json:"gpu,omitempty"`
}

// ActorLatencyInfo Actor 延迟信息
type ActorLatencyInfo struct {
	CalcLatency int64 `json:"calc_latency"` // 计算延迟（毫秒），0也是有效值
	LinkLatency int64 `json:"link_latency"` // 链路延迟（毫秒），0也是有效值
}

// ActorResponse Actor 响应
type ActorResponse struct {
	ID        string              `json:"id,omitempty"`
	Component *ActorComponentInfo `json:"component,omitempty"`
	Info      *ActorLatencyInfo   `json:"info,omitempty"`
}

// GetApplicationActorsResponse 获取应用 Actors 响应
type GetApplicationActorsResponse map[string][]ActorResponse

// GetExecutionResultRequest 获取执行结果请求
type GetExecutionResultRequest struct {
	AppID     string `json:"app_id"`           // 应用 ID
	SessionID string `json:"session_id"`       // Session ID
	ObjectID  string `json:"object_id"`        // Object ID
	Source    string `json:"source,omitempty"` // Source store ID (可选)
}

// GetExecutionResultResponse 获取执行结果响应
type GetExecutionResultResponse struct {
	ObjectID string `json:"object_id"`           // Object ID
	Source   string `json:"source,omitempty"`    // Source store ID
	Data     string `json:"data,omitempty"`      // 数据内容（JSON 字符串或 base64 编码）
	DataType string `json:"data_type,omitempty"` // 数据类型（如 "json", "pickle", "bytes"）
	Size     int64  `json:"size,omitempty"`      // 数据大小（字节）
}

// BuildGetApplicationActorsResponse 构建 Actor 响应
func BuildGetApplicationActorsResponse(actors map[string][]*taskpkg.Actor) GetApplicationActorsResponse {
	resp := make(GetApplicationActorsResponse)

	for functionName, actorList := range actors {
		actorResponses := make([]ActorResponse, 0, len(actorList))
		for _, actor := range actorList {
			actorResp := ActorResponse{
				ID: string(actor.GetID()),
			}

			// 获取延迟信息
			info := actor.GetInfo()
			if info != nil {
				actorResp.Info = &ActorLatencyInfo{
					CalcLatency: info.CalcLatency,
					LinkLatency: info.LinkLatency,
				}
			}

			// 获取组件信息
			component := actor.GetComponent()
			if component != nil {
				componentInfo := &ActorComponentInfo{
					ID:         component.GetID(),
					Image:      component.GetImage(),
					ProviderID: component.GetProviderID(),
				}

				// 获取资源使用信息
				resourceUsage := component.GetResourceUsage()
				if resourceUsage != nil {
					componentInfo.ResourceUsage = &ResourceInfo{
						CPU:    resourceUsage.CPU,
						Memory: resourceUsage.Memory,
						GPU:    resourceUsage.GPU,
					}
				}

				actorResp.Component = componentInfo
			}

			actorResponses = append(actorResponses, actorResp)
		}
		resp[functionName] = actorResponses
	}

	return resp
}
