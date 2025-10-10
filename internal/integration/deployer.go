package integration

import (
	"context"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/platform/task"
)

type Deployer struct {
	am *application.Manager
	rm *resource.Manager
}

func NewDeployer(am *application.Manager, rm *resource.Manager) *Deployer {
	return &Deployer{
		am: am,
		rm: rm,
	}
}

func (d *Deployer) Deploy(ctx context.Context, res task.Resources, appId string, funcName string) error {
	return nil
}
