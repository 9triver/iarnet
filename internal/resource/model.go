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

type Capacity struct {
	Total     *Info `json:"total"`
	Used      *Info `json:"used"`
	Available *Info `json:"available"`
}

type ContainerRef struct {
	ID       string        `json:"id"`
	Provider Provider      `json:"provider"`
	Spec     ContainerSpec `json:"spec"`
}

type Info struct {
	CPU    int64 `json:"cpu"` // millicores
	Memory int64 `json:"memory"`
	GPU    int64 `json:"gpu"`
}

type ContainerSpec struct {
	Image        string
	Ports        []int
	Command      []string
	Requirements Info
	Env          map[string]string
}
