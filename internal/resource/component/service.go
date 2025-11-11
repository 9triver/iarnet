package component

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/resource/repository"
	"github.com/lithammer/shortuuid/v4"
	"github.com/sirupsen/logrus"
)

type ConsumerSupplier interface {
	GetConsumers() (resource.Consumer, error)
}

type ComponentService interface {
	RegisterProvider(provider Provider)
	DeployComponent(ctx context.Context, name string, runtimeEnv resource.RuntimeEnv, resourceRequest *resource.Info) (*Component, error)
}

type componentService struct {
	providers           []Provider
	manager             Manager
	componentRepository repository.ComponentRepository
	images              map[resource.RuntimeEnv]string
}

func NewComponentService(componentRepository repository.ComponentRepository, manager Manager) ComponentService {
	return &componentService{
		providers:           []Provider{nil},
		componentRepository: componentRepository,
		manager:             manager,
		images:              make(map[resource.RuntimeEnv]string),
	}
}

func (c *componentService) RegisterProvider(provider Provider) {
	c.providers = append(c.providers, provider)
}

func (c *componentService) DeployComponent(ctx context.Context, name string, runtimeEnv resource.RuntimeEnv, resourceRequest *resource.Info) (*Component, error) {
	if resourceRequest == nil {
		return nil, fmt.Errorf("resource request is required")
	}

	image, ok := c.images[runtimeEnv]
	if !ok {
		return nil, fmt.Errorf("image for runtime environment %s not found", runtimeEnv)
	}

	id := "comp-" + shortuuid.New()
	component := NewComponent(id, name, image, resourceRequest)

	if err := c.manager.AddComponent(ctx, component); err != nil {
		return nil, fmt.Errorf("failed to add component to manager: %w", err)
	}

	for _, provider := range c.providers {
		if provider == nil {
			continue
		}

		if provider.GetStatus() != resource.ProviderStatusConnected {
			logrus.Debugf("Skipping provider %s: status is not connected", provider.GetID())
			continue
		}

		available, err := provider.GetAvailable(ctx)
		if err != nil {
			logrus.Warnf("Failed to get available resources from provider %s: %v", provider.GetID(), err)
			continue
		}

		if !satisfiesResourceRequest(available, resourceRequest) {
			logrus.Debugf("Provider %s does not have sufficient resources", provider.GetID())
			continue
		}

		logrus.Infof("Deploying component on provider %s", provider.GetID())
		if err := provider.DeployComponent(ctx, id, image, resourceRequest); err != nil {
			logrus.Errorf("Failed to deploy component on provider %s: %v", provider.GetID(), err)
			continue
		}

		// TODO: 保存到 repository

		return component, nil
	}

	return nil, fmt.Errorf("no available provider found that satisfies the resource requirements")
}

// satisfiesResourceRequest 检查可用资源是否满足资源请求
func satisfiesResourceRequest(available *resource.Info, request *resource.Info) bool {
	if available == nil || request == nil {
		return false
	}

	// 检查 CPU
	if available.CPU < request.CPU {
		return false
	}

	// 检查 Memory
	if available.Memory < request.Memory {
		return false
	}

	// 检查 GPU
	if available.GPU < request.GPU {
		return false
	}

	return true
}
