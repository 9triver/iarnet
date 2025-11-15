package types

type RuntimeEnv = string

type Info struct {
	CPU    int64 `json:"cpu"`    // millicores
	Memory int64 `json:"memory"` // bytes
	GPU    int64 `json:"gpu"`
}

type Capacity struct {
	Total     *Info `json:"total"`
	Used      *Info `json:"used"`
	Available *Info `json:"available"`
}

const (
	RuntimeEnvPython RuntimeEnv = "python"
)

type ResourceRequest Info

type ProviderType string

type ProviderStatus int32

const (
	ProviderStatusUnknown      ProviderStatus = 0
	ProviderStatusConnected    ProviderStatus = 1
	ProviderStatusDisconnected ProviderStatus = 2
)

type ObjectID = string

type StoreID = string
