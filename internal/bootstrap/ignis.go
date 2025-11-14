package bootstrap

import (
	"github.com/9triver/iarnet/internal/domain/ignis/controller"
	"github.com/sirupsen/logrus"
)

// BootstrapIgnis 初始化 Ignis 模块
func bootstrapIgnis(iarnet *Iarnet) error {
	// 初始化 Controller Manager
	controllerManager := controller.NewManager(iarnet.ResourceManager)

	// 初始化 Controller Service
	controllerService := controller.NewService(
		controllerManager,
		iarnet.ResourceManager,
		iarnet.ResourceManager,
	)

	iarnet.ControllerManager = controllerManager
	iarnet.ControllerService = controllerService

	logrus.Info("Ignis module initialized")
	return nil
}
