package application

import (
	"time"

	"github.com/9triver/iarnet/internal/domain/application/types"
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
