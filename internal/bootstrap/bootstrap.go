package bootstrap

import (
	"fmt"

	"github.com/9triver/iarnet/internal/config"
	"github.com/sirupsen/logrus"
)

// Initialize 初始化所有模块
// 按照依赖顺序初始化：基础设施 -> Resource -> Application -> Execution -> Transport
func Initialize(cfg *config.Config) (*Iarnet, error) {
	iarnet := &Iarnet{
		Config:             cfg,
		DockerClient:       nil,
		Channeler:          nil,
		ResourceManager:    nil,
		ApplicationManager: nil,
		IgnisPlatform:      nil,
		HTTPServer:         nil,
	}

	// 1. 初始化 Resource 模块
	if err := bootstrapResource(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize resource module: %w", err)
	}

	// 2. 初始化 Execution 模块
	if err := bootstrapIgnis(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize execution module: %w", err)
	}

	// 3. 初始化 Application 模块
	if err := bootstrapApplication(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize application module: %w", err)
	}

	// 4. 初始化 Transport 层
	if err := bootstrapTransport(iarnet); err != nil {
		return nil, fmt.Errorf("failed to initialize transport layer: %w", err)
	}

	logrus.Info("All modules initialized successfully")
	return iarnet, nil
}
