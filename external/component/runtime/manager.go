package runtime

import (
	"fmt"

	"github.com/9triver/iarnet/component/py"
	"github.com/9triver/ignis/objects"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils"
)

// Manager 抽象不同语言函数的运行时管理器
// 负责根据函数定义进行环境准备与可选的执行器启动
type Manager interface {
	// Language 返回该运行时支持的语言
	Language() proto.Language
	// Setup 根据函数定义准备运行环境（如安装依赖、启动执行器）
	Setup(fn *cluster.Function) error
	// Execute 执行指定对象的指定方法，返回 Future[objects.Interface]
	Execute(name, method string, args map[string]objects.Interface) utils.Future[objects.Interface]
}

// GetManager 返回指定语言的运行时管理器
func GetManager(language proto.Language) (Manager, error) {
	switch language {
	case proto.Language_LANG_PYTHON:
		return py.NewRuntimeManager(), nil
	default:
		return nil, fmt.Errorf("unsupported language: %v", language)
	}
}
