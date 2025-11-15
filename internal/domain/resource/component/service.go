package component

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/lithammer/shortuuid/v4"
	"github.com/sirupsen/logrus"
)

type Service interface {
	DeployComponent(ctx context.Context, name string, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*Component, error)
}

type componentService struct {
	manager         Manager
	providerService provider.Service
	images          map[types.RuntimeEnv]string
}

func NewService(manager Manager, providerService provider.Service, componentImages map[string]string) Service {
	return &componentService{
		manager:         manager,
		providerService: providerService,
		images:          componentImages,
	}
}

func (c *componentService) DeployComponent(ctx context.Context, name string, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*Component, error) {
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

	// 通过 provider service 查找可用的 provider
	p, err := c.providerService.FindAvailableProvider(ctx, resourceRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to find available provider: %w", err)
	}

	logrus.Infof("Deploying component on provider %s", p.GetID())
	if err := p.Deploy(ctx, id, image, resourceRequest); err != nil {
		return nil, fmt.Errorf("failed to deploy component on provider %s: %w", p.GetID(), err)
	}

	// TODO: 保存到 repository

	return component, nil
}
