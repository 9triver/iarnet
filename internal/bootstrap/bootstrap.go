package bootstrap

import (
	"fmt"

	"github.com/9triver/iarnet/internal/config"
	"github.com/sirupsen/logrus"
)

// Initialize 初始化所有模块
// 按照依赖顺序初始化：基础设施 -> Resource -> Application -> Ignis -> Transport
func Initialize(cfg *config.Config) (*Iarnet, error) {
	iarnet := &Iarnet{
		Config:             cfg,
		DockerClient:       nil,
		Channeler:          nil,
		ResourceManager:    nil,
		ApplicationManager: nil,
		ControllerManager:  nil,
		ControllerService:  nil,
	}

	// 1. 初始化基础设施
	if err := bootstrapInfrastructure(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize infrastructure: %w", err)
	}

	// 2. 初始化 Resource 模块
	if err := bootstrapResource(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize resource module: %w", err)
	}

	// 3. 初始化 Application 模块
	if err := bootstrapApplication(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize application module: %w", err)
	}

	// 4. 初始化 Ignis 模块
	if err := bootstrapIgnis(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize ignis module: %w", err)
	}

	// 5. 初始化 Transport 层
	if err := bootstrapTransport(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize transport layer: %w", err)
	}

	logrus.Info("All modules initialized successfully")
	return iarnet, nil
}

// bootstrapInfrastructure 初始化基础设施（ZMQ、Docker 等）
func bootstrapInfrastructure(iarnet *Iarnet) error {
	// 基础设施初始化（ZMQ 将在 Transport 层初始化）
	logrus.Info("Infrastructure initialized")
	return nil
}
