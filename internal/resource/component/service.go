package component

import (
	"context"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/resource/repository"
)

type ComponentService interface {
	RegisterProvider(provider Provider)
	DeployComponent(ctx context.Context, runtimeEnv resource.RuntimeEnv, resourceRequest *resource.ResourceRequest) (*resource.ContainerRef, error)
}

type componentService struct {
	providers           []Provider
	componentRepository repository.ComponentRepository
}

func NewComponentService(componentRepository repository.ComponentRepository) ComponentService {
	return &componentService{
		providers:           []Provider{nil},
		componentRepository: componentRepository,
	}
}

func (c *componentService) RegisterProvider(provider Provider) {
	c.providers = append(c.providers, provider)
}

func (c *componentService) DeployComponent(ctx context.Context, runtimeEnv resource.RuntimeEnv, resourceRequest *resource.ResourceRequest) (*resource.ContainerRef, error) {

	return nil, nil
}
