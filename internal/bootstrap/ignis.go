package bootstrap

import (
	"github.com/9triver/iarnet/internal/domain/execution"
	"github.com/9triver/iarnet/internal/domain/execution/controller"
	"github.com/sirupsen/logrus"
)

// BootstrapIgnis 初始化 Execution 模块
func bootstrapIgnis(iarnet *Iarnet) error {
	// 初始化 Controller Manager
	controllerManager := controller.NewManager(iarnet.ResourceManager)
	controllerService := controller.NewService(controllerManager, iarnet.ResourceManager, iarnet.ResourceManager)

	// 初始化 Execution Platform
	iarnet.IgnisPlatform = execution.NewPlatform(controllerService)

	logrus.Info("Execution module initialized")
	return nil
}
