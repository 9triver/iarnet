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
	"github.com/asynkron/protoactor-go/actor"
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

func (d *Deployer) DeployPyFunc(ctx actor.Context, appId string, f *controller.AppendPyFunc) ([]*proto.ActorInfo, error) {
	image, ok := d.cfg.ComponentImages["python"]
	if !ok {
		return nil, fmt.Errorf("actor image not found for environment: %s", "python")
	}
	cf, err := d.rm.Deploy(context.Background(), resource.ContainerSpec{
		Image: image,
		Requirements: resource.Info{
			CPU:    f.Resources.CPU,
			Memory: f.Resources.Memory,
			GPU:    f.Resources.GPU,
		},
		Env: map[string]string{
			"IGNIG_ADDR": d.cfg.ExternalAddr + ":" + strconv.Itoa(int(d.cfg.Ignis.Port)),
			// "CONN_ID":    fmt.Sprintf("%s_%s", appId, f.Name),
			"APP_ID":    appId,
			"FUNC_NAME": f.Name,
			// "PYTHON_EXEC": "python",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deploy: %w", err)
	}
	logrus.Infof("deployed to provider: %s, container ID: %s", cf.Provider.GetID(), cf.ID)
	d.am.RegisterComponent(appId, f.Name, cf)
	return nil, nil
}
