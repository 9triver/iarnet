package integration

import (
	"context"
	"fmt"
	"strconv"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/controller"
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

func (d *Deployer) DeployPyFunc(ctx context.Context, appId string, f *controller.AppendPyFunc) (*proto.ActorRef, error) {
	image, ok := d.cfg.ComponentImages["python"]
	if !ok {
		return nil, fmt.Errorf("actor image not found for environment: %s", "python")
	}
	cf, err := d.rm.Deploy(ctx, resource.ContainerSpec{
		Image:   image,
		Ports:   []int{},
		Command: []string{},
		Requirements: resource.Info{
			CPU:    f.Resources.CPU,
			Memory: f.Resources.Memory,
			GPU:    f.Resources.GPU,
		},
		Env: map[string]string{
			"APP_ID":      appId,
			"IGNIS_PORT":  strconv.Itoa(int(d.cfg.Ignis.Port)),
			"FUNC_NAME":   f.Name,
			"VENV_NAME":   f.Venv,
			"PYTHON_EXEC": "python",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deploy: %w", err)
	}
	logrus.Infof("deployed to provider: %s, container ID: %s", cf.Provider.GetID(), cf.ID)
	d.am.RegisterComponent(appId, f.Name, cf)
	return nil, nil
}
