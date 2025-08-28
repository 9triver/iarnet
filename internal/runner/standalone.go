package runner

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/yourusername/container-peer-service/internal/resource"
)

type StandaloneRunner struct {
	docker *client.Client
}

func NewStandaloneRunner() (*StandaloneRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	return &StandaloneRunner{docker: cli}, err
}

func (r *StandaloneRunner) Run(ctx context.Context, spec ContainerSpec) error {
	resp, err := r.docker.ContainerCreate(ctx, &container.Config{
		Image: spec.Image,
		Cmd:   spec.Command,
	}, &container.HostConfig{
		Resources: container.Resources{
			CPUQuota: int64(spec.CPU * 100000), // Rough conversion
			Memory:   int64(spec.Memory * 1024 * 1024 * 1024),
			// GPU: Docker GPU support requires nvidia-docker, assume configured.
		},
	}, nil, nil, "")
	if err != nil {
		return err
	}
	return r.docker.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
}

func (r *StandaloneRunner) Stop(containerID string) error {
	return r.docker.ContainerStop(context.Background(), containerID, container.StopOptions{})
}

func (r *StandaloneRunner) GetUsage() resource.ResourceUsage {
	// TODO: Poll docker stats for all managed containers.
	return resource.ResourceUsage{} // Placeholder
}
