package resource_test

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/9triver/iarnet/internal/domain/resource/provider"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"google.golang.org/grpc"
)

// TestRemoteDockerProviderOperations
// 覆盖测试大纲1.4：通过远程 Docker Provider 的 RPC 接口完成 Connect、GetCapacity、Deploy。
func TestRemoteDockerProviderOperations(t *testing.T) {
	server := newFakeProviderRPCServer("docker", &resourcepb.Capacity{
		Total:     &resourcepb.Info{Cpu: 4000, Memory: 8 << 30},
		Used:      &resourcepb.Info{Cpu: 500, Memory: 1 << 30},
		Available: &resourcepb.Info{Cpu: 3500, Memory: 7 << 30},
	})
	host, port, stop := startProviderRPCServer(t, server)
	defer stop()

	env := &provider.EnvVariables{IarnetHost: "10.0.0.2", ZMQPort: 7001, StorePort: 7011}
	p := provider.NewProvider("remote-docker", host, port, env)

	ctx := context.Background()
	if err := p.Connect(ctx); err != nil {
		t.Fatalf("connect remote docker provider failed: %v", err)
	}
	if p.GetType() != types.ProviderType("docker") {
		t.Fatalf("expected provider type docker, got %s", p.GetType())
	}

	capacity, err := p.GetCapacity(ctx)
	if err != nil {
		t.Fatalf("GetCapacity failed: %v", err)
	}
	if capacity.Total.CPU != 4000 || capacity.Available.Memory != 7<<30 {
		t.Fatalf("unexpected capacity: %+v", capacity)
	}

	req := &types.Info{CPU: 1000, Memory: 512 * 1024 * 1024}
	if err := p.Deploy(ctx, "component.docker", "nginx:alpine", req); err != nil {
		t.Fatalf("deploy via remote docker provider failed: %v", err)
	}
	if server.lastDeploy == nil || server.lastDeploy.Image != "nginx:alpine" {
		t.Fatalf("deploy request not recorded: %+v", server.lastDeploy)
	}
	if server.lastDeploy.EnvVars["COMPONENT_ID"] != "component.docker" {
		t.Fatalf("expected COMPONENT_ID env var propagated")
	}
}

// TestRemoteK8sProviderOperations
// 覆盖测试大纲1.5：通过远程 K8s Provider 获取集群容量并校验 provider 类型。
func TestRemoteK8sProviderOperations(t *testing.T) {
	server := newFakeProviderRPCServer("k8s", &resourcepb.Capacity{
		Total:     &resourcepb.Info{Cpu: 8000, Memory: 16 << 30},
		Used:      &resourcepb.Info{Cpu: 1500, Memory: 2 << 30},
		Available: &resourcepb.Info{Cpu: 6500, Memory: 14 << 30},
	})
	host, port, stop := startProviderRPCServer(t, server)
	defer stop()

	env := &provider.EnvVariables{IarnetHost: "10.0.0.2", ZMQPort: 7001, StorePort: 7011}
	p := provider.NewProvider("remote-k8s", host, port, env)

	ctx := context.Background()
	if err := p.Connect(ctx); err != nil {
		t.Fatalf("connect remote k8s provider failed: %v", err)
	}
	if p.GetType() != types.ProviderType("k8s") {
		t.Fatalf("expected provider type k8s, got %s", p.GetType())
	}

	capacity, err := p.GetCapacity(ctx)
	if err != nil {
		t.Fatalf("GetCapacity failed: %v", err)
	}
	if capacity.Available.CPU != 6500 {
		t.Fatalf("unexpected available CPU: %+v", capacity.Available)
	}
}

// TestProviderManagerAggregateCapacity
// 对应测试大纲1.6：Resource Manager 可聚合多 Provider 的实时容量数据。
func TestProviderManagerAggregateCapacity(t *testing.T) {
	dockerServer := newFakeProviderRPCServer("docker", &resourcepb.Capacity{
		Total:     &resourcepb.Info{Cpu: 4000, Memory: 8 << 30},
		Used:      &resourcepb.Info{Cpu: 500, Memory: 1 << 30},
		Available: &resourcepb.Info{Cpu: 3500, Memory: 7 << 30},
	})
	dockerHost, dockerPort, stopDocker := startProviderRPCServer(t, dockerServer)
	defer stopDocker()

	k8sServer := newFakeProviderRPCServer("k8s", &resourcepb.Capacity{
		Total:     &resourcepb.Info{Cpu: 6000, Memory: 24 << 30},
		Used:      &resourcepb.Info{Cpu: 1000, Memory: 4 << 30},
		Available: &resourcepb.Info{Cpu: 5000, Memory: 20 << 30},
	})
	k8sHost, k8sPort, stopK8s := startProviderRPCServer(t, k8sServer)
	defer stopK8s()

	ctx := context.Background()
	env := &provider.EnvVariables{IarnetHost: "10.0.0.2", ZMQPort: 7001, StorePort: 7011}
	dockerProvider := provider.NewProvider("remote-docker", dockerHost, dockerPort, env)
	if err := dockerProvider.Connect(ctx); err != nil {
		t.Fatalf("connect docker provider failed: %v", err)
	}
	k8sProvider := provider.NewProvider("remote-k8s", k8sHost, k8sPort, env)
	if err := k8sProvider.Connect(ctx); err != nil {
		t.Fatalf("connect k8s provider failed: %v", err)
	}

	manager := provider.NewManager()
	manager.Add(dockerProvider)
	manager.Add(k8sProvider)

	agg, err := manager.AggregateCapacity(ctx)
	if err != nil {
		t.Fatalf("aggregate capacity failed: %v", err)
	}
	if agg.Total.CPU != 10000 || agg.Total.Memory != 32<<30 {
		t.Fatalf("unexpected total capacity: %+v", agg.Total)
	}
	if agg.Used.CPU != 1500 || agg.Used.Memory != 5<<30 {
		t.Fatalf("unexpected used capacity: %+v", agg.Used)
	}
	if agg.Available.CPU != agg.Total.CPU-agg.Used.CPU ||
		agg.Available.Memory != agg.Total.Memory-agg.Used.Memory {
		t.Fatalf("available capacity mismatch: %+v", agg.Available)
	}
}

// --- Fake provider RPC server helpers ---------------------------------------

type fakeProviderRPCServer struct {
	providerpb.UnimplementedServiceServer
	providerType string
	capacity     *resourcepb.Capacity
	lastDeploy   *providerpb.DeployRequest
}

func newFakeProviderRPCServer(providerType string, capacity *resourcepb.Capacity) *fakeProviderRPCServer {
	return &fakeProviderRPCServer{
		providerType: providerType,
		capacity:     capacity,
	}
}

func (s *fakeProviderRPCServer) Connect(ctx context.Context, req *providerpb.ConnectRequest) (*providerpb.ConnectResponse, error) {
	if req == nil || req.ProviderId == "" {
		return &providerpb.ConnectResponse{Success: false, Error: "provider id required"}, nil
	}
	return &providerpb.ConnectResponse{
		Success: true,
		ProviderType: &providerpb.ProviderType{
			Name: s.providerType,
		},
	}, nil
}

func (s *fakeProviderRPCServer) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	return &providerpb.GetCapacityResponse{
		Capacity: cloneCapacity(s.capacity),
	}, nil
}

func (s *fakeProviderRPCServer) GetAvailable(ctx context.Context, req *providerpb.GetAvailableRequest) (*providerpb.GetAvailableResponse, error) {
	return &providerpb.GetAvailableResponse{
		Available: cloneInfo(s.capacity.Available),
	}, nil
}

func (s *fakeProviderRPCServer) Deploy(ctx context.Context, req *providerpb.DeployRequest) (*providerpb.DeployResponse, error) {
	s.lastDeploy = req
	return &providerpb.DeployResponse{}, nil
}

func (s *fakeProviderRPCServer) HealthCheck(ctx context.Context, req *providerpb.HealthCheckRequest) (*providerpb.HealthCheckResponse, error) {
	return &providerpb.HealthCheckResponse{}, nil
}

func startProviderRPCServer(t *testing.T, svc providerpb.ServiceServer) (string, int, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer()
	providerpb.RegisterServiceServer(server, svc)
	go func() {
		if err := server.Serve(lis); err != nil {
			t.Logf("provider server stopped: %v", err)
		}
	}()
	host, portStr, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}
	return host, port, func() {
		server.GracefulStop()
		_ = lis.Close()
	}
}

