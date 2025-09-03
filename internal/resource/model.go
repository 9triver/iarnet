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
	Total     Usage `json:"total"`
	Allocated Usage `json:"allocated"`
	Available Usage `json:"available"`
}
