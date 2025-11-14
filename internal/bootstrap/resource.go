package bootstrap

import (
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	"github.com/sirupsen/logrus"
)

// BootstrapResource 初始化 Resource 模块
func bootstrapResource(iarnet *Iarnet) error {
	// 初始化 Store
	storeInstance := store.NewStore()

	// 初始化 Resource Manager
	resourceManager := resource.NewManager(
		iarnet.Channeler,
		storeInstance,
		iarnet.Config.Resource.ComponentImages,
	)
	iarnet.ResourceManager = resourceManager

	logrus.Info("Resource module initialized")
	return nil
}
