package integration

import (
	"context"
	"fmt"
	"strconv"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/platform/task"
	"github.com/sirupsen/logrus"
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
	image, ok := d.cfg.ComponentImages[string(env)]
	if !ok {
		return fmt.Errorf("actor image not found for environment: %s", env)
	}
	cf, err := d.rm.Deploy(ctx, resource.ContainerSpec{
		Image:   image,
		Ports:   []int{},
		Command: []string{},
		Requirements: resource.Info{
			CPU:    res.CPU,
			Memory: res.Memory,
			GPU:    res.GPU,
		},
		Env: map[string]string{
			"APP_ID":     appId,
			"IGNIS_PORT": strconv.Itoa(int(d.cfg.Ignis.Port)),
			"FUNC_NAME":  funcName,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to deploy: %w", err)
	}
	logrus.Infof("deployed to provider: %s, container ID: %s", cf.Provider.GetID(), cf.ID)
	d.am.RegisterComponent(appId, funcName, cf)
	return nil
}
