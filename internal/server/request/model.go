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

// RegisterProviderRequest 注册资源提供者请求结构
type RegisterProviderRequest struct {
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
	KubeConfigPath string `json:"kubeConfigPath,omitempty"` // kubeconfig文件路径
	Namespace      string `json:"namespace,omitempty"`      // Kubernetes命名空间
	Context        string `json:"context,omitempty"`        // kubeconfig上下文
}
