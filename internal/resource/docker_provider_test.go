package resource_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/9triver/iarnet/internal/resource"
)

func TestDeploy(t *testing.T) {
	dp, err := resource.GetLocalDockerProvider()
	if err != nil {
		fmt.Println(err)
		return
	}

	ctx := context.Background()

	_, err = dp.Deploy(ctx, resource.ContainerSpec{
		Image: "hello-world:latest",
		Ports: []int{8080},
		Requirements: resource.Info{
			CPU:    500,
			Memory: 128 * 1024 * 1024,
		},
	})
	if err != nil {
		t.Errorf("Deploy failed: %v", err)
	}
}
