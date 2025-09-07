package runner

import (
	"context"

	"github.com/9triver/iarnet/internal/resource"
)

type ContainerSpec struct {
	Image   string
	Command []string
	EnvVars map[string]string
	Ports   []int
	CPU     float64
	Memory  float64
	GPU     float64
}

type Runner interface {
	Run(ctx context.Context, spec ContainerSpec) error
	Stop(containerID string) error
	GetUsage() resource.Usage // For monitoring
}
