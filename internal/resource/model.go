package resource

type Type string

const (
	CPU    Type = "cpu"
	Memory Type = "memory"
	GPU    Type = "gpu"
)

type Usage struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	GPU    float64 `json:"gpu"`
}

type ContainerRef struct {
	ID       string        `json:"id"`
	Provider Provider      `json:"provider"`
	Spec     ContainerSpec `json:"spec"`
}

type ContainerSpec struct {
	Image        string
	Ports        []int
	Command      []string
	Requirements Info
	Env          map[string]string
}
