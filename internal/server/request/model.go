package request

// CreateApplicationRequest 创建应用请求结构
type CreateApplicationRequest struct {
	Name        string  `json:"name"`
	ImportType  string  `json:"importType"` // "git" or "docker"
	GitUrl      *string `json:"gitUrl,omitempty"`
	Branch      *string `json:"branch,omitempty"`
	DockerImage *string `json:"dockerImage,omitempty"`
	DockerTag   *string `json:"dockerTag,omitempty"`
	Type        string  `json:"type"` // "web", "api", "worker", "database"
	Description *string `json:"description,omitempty"`
	Ports       []int   `json:"ports,omitempty"`
	HealthCheck *string `json:"healthCheck,omitempty"`
}
