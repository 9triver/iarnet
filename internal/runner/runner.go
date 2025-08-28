package runner

import (
	"context"

	"github.com/yourusername/container-peer-service/internal/resource"
)

type ContainerSpec struct {
	Image   string
	Command []string
	CPU     float64
	Memory  float64
	GPU     float64
}

type Runner interface {
	Run(ctx context.Context, spec ContainerSpec) error
	Stop(containerID string) error
	GetUsage() resource.ResourceUsage // For monitoring
}
