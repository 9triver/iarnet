package request

// CreateApplicationRequest 创建应用请求结构
type CreateApplicationRequest struct {
	Name        string  `json:"name"`
	GitUrl      *string `json:"gitUrl,omitempty"`
	Branch      *string `json:"branch,omitempty"`
	Type        string  `json:"type"` // "web", "api", "worker", "database"
	Description *string `json:"description,omitempty"`
	Ports       []int   `json:"ports,omitempty"`
	HealthCheck *string `json:"healthCheck,omitempty"`
	ExecuteCmd  *string `json:"executeCmd,omitempty"`
	RunnerEnv   *string `json:"runnerEnv,omitempty"` // 运行环境，如 "python", "node", "go" 等
}

// RegisterProviderRequest 注册资源提供者请求结构
type RegisterProviderRequest struct {
	Name   string      `json:"name"`   // 资源提供者名称
	Type   string      `json:"type"`   // "docker" or "k8s"
	Config interface{} `json:"config"` // 配置信息，根据类型不同而不同
}

// DockerProviderConfig Docker提供者配置
type DockerProviderConfig struct {
	Host        string `json:"host"`                  // Docker daemon host
	TLSCertPath string `json:"tlsCertPath,omitempty"` // TLS证书路径
	TLSVerify   bool   `json:"tlsVerify,omitempty"`   // 是否启用TLS验证
	APIVersion  string `json:"apiVersion,omitempty"`  // Docker API版本
}

// K8sProviderConfig Kubernetes提供者配置
type K8sProviderConfig struct {
	KubeConfigContent string `json:"kubeConfigContent"`   // kubeconfig文件内容
	Namespace         string `json:"namespace,omitempty"` // Kubernetes命名空间
	Context           string `json:"context,omitempty"`   // kubeconfig上下文
}

// AddPeerNodeRequest 添加peer节点请求结构
type AddPeerNodeRequest struct {
	Address string `json:"address"` // peer节点地址，格式: host:port
}

// SaveFileRequest 保存文件请求结构
type SaveFileRequest struct {
	Content string `json:"content"` // 文件内容
}

// CreateFileRequest 创建文件请求结构
type CreateFileRequest struct {
	FilePath string `json:"filePath"` // 文件路径
	Content  string `json:"content"`  // 文件内容
}

// CreateDirectoryRequest 创建目录请求结构
type CreateDirectoryRequest struct {
	DirectoryPath string `json:"directoryPath"` // 目录路径
}
