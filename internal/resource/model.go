package resource

type ResourceType string

const (
	CPU    ResourceType = "cpu"
	Memory ResourceType = "memory"
	GPU    ResourceType = "gpu"
)

type Usage struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	GPU    float64 `json:"gpu"`
}

type Capacity struct {
	Total     Usage `json:"total"`
	Used      Usage `json:"used"`
	Available Usage `json:"available"`
}
