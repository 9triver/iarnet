package component

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	"github.com/9triver/iarnet/internal/proto/common"
	"github.com/9triver/iarnet/internal/util"
	"github.com/sirupsen/logrus"
)

type Service interface {
	DeployComponent(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*Component, error)
}

type componentService struct {
	manager         Manager
	providerService provider.Service
	// images 已废弃：不再在 iarnet 侧选择镜像，由 provider 根据语言决定
	images map[types.RuntimeEnv]string
}

func NewService(manager Manager, providerService provider.Service, componentImages map[string]string) Service {
	return &componentService{
		manager:         manager,
		providerService: providerService,
		images:          componentImages, // 保留用于向后兼容，但不再使用
	}
}

func (c *componentService) DeployComponent(ctx context.Context, runtimeEnv types.RuntimeEnv, resourceRequest *types.Info) (*Component, error) {
	if resourceRequest == nil {
		return nil, fmt.Errorf("resource request is required")
	}

	// 将 RuntimeEnv 转换为 Language
	language := types.RuntimeEnvToLanguage(runtimeEnv)
	if language == common.Language_LANG_UNKNOWN {
		return nil, fmt.Errorf("unsupported runtime environment: %s", runtimeEnv)
	}

	id := util.GenIDWith("comp.")
	// 不再在 iarnet 侧选择镜像，image 字段保留用于向后兼容，但实际部署时由 provider 决定
	// 使用空字符串或占位符，provider 会根据语言选择镜像
	image := "" // 不再使用，由 provider 根据语言决定
	component := NewComponent(id, image, resourceRequest)

	if err := c.manager.AddComponent(ctx, component); err != nil {
		return nil, fmt.Errorf("failed to add component to manager: %w", err)
	}

	// 通过 provider service 查找支持该语言的可用 provider
	p, err := c.providerService.FindAvailableProvider(ctx, resourceRequest, language)
	if err != nil {
		return nil, fmt.Errorf("failed to find available provider for language %v: %w", language, err)
	}
	logrus.Infof("Deploying component %s (language: %v) on provider %s", id, language, p.GetID())

	// 传递语言而不是镜像，由 provider 决定使用什么镜像
	if err := p.Deploy(ctx, id, language, resourceRequest); err != nil {
		return nil, fmt.Errorf("failed to deploy component on provider %s: %w", p.GetID(), err)
	}
	component.SetProviderID(p.GetID())

	// TODO: 保存到 repository

	return component, nil
}
