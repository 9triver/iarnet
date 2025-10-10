package task

import (
	"context"

	"github.com/9triver/ignis/actor/functions"
)

type Env string

const (
	EnvPython Env = "python"
)

type Resources struct {
	CPU    int64 // CPU milli Cores
	Memory int64 // memory in Bytes
	GPU    int64 // GPU cores
}

type Deployer interface {
	Deploy(ctx context.Context, res Resources, appId string, funcName string, env Env) error
}

// VenvMgrDeployer 是一个部署器，用于部署 Python 函数到 Venv 环境（原始默认实现）
type VenvMgrDeployer struct {
	vm *functions.VenvManager
}

func NewVenvMgrDeployer(vm *functions.VenvManager) *VenvMgrDeployer {
	return &VenvMgrDeployer{
		vm: vm,
	}
}

func (d *VenvMgrDeployer) Deploy(ctx context.Context, res Resources, appId string, funcName string) error {
	return nil
}
