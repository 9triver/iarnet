package integration

import (
	"context"
	"fmt"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/platform/task"
)

type Deployer struct {
	am  *application.Manager
	rm  *resource.Manager
	cfg *config.Config
}

func NewDeployer(am *application.Manager, rm *resource.Manager, cfg *config.Config) *Deployer {
	return &Deployer{
		am:  am,
		rm:  rm,
		cfg: cfg,
	}
}

func (d *Deployer) Deploy(ctx context.Context, res task.Resources, appId string, funcName string, env task.Env) error {
	image, ok := d.cfg.ActorImages[string(env)]
	if !ok {
		return fmt.Errorf("actor image not found for environment: %s", env)
	}
	d.rm.Deploy(ctx, resource.ContainerSpec{
		Image:   image,
		Ports:   []int{},
		Command: []string{},
		Requirements: resource.Info{
			CPU:    res.CPU,
			Memory: res.Memory,
			GPU:    res.GPU,
		},
	})
	return nil
}
