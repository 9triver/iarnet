package bootstrap

import (
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	"github.com/sirupsen/logrus"
)

// BootstrapResource 初始化 Resource 模块
func bootstrapResource(iarnet *Iarnet) error {
	// 初始化 Store
	storeInstance := store.NewStore()

	// 使用占位符 channeler 初始化 Resource Manager
	// 真正的 channeler 会在 Transport 层创建后注入
	nullChanneler := component.NewNullChanneler()
	resourceManager := resource.NewManager(
		nullChanneler,
		storeInstance,
		iarnet.Config.Resource.ComponentImages,
	)
	iarnet.ResourceManager = resourceManager

	logrus.Info("Resource module initialized")
	return nil
}
