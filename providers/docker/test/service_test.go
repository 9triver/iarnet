package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/providers/docker/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestService_AssignID 测试 AssignID 功能
func TestService_AssignID(t *testing.T) {
	// 跳过测试如果 Docker 不可用
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	svc, err := createTestService()
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()

	// 测试正常分配 ID
	req := &providerpb.ConnectRequest{
		ProviderId: "test-provider-123",
	}

	resp, err := svc.Connect(ctx, req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Error)
	assert.Equal(t, "test-provider-123", svc.GetProviderID())

	// 测试重复分配 ID（应该失败）
	resp2, err := svc.Connect(ctx, req)
	require.NoError(t, err)
	assert.False(t, resp2.Success)
	assert.Contains(t, resp2.Error, "already connected")

	// 测试空 ID（应该失败）
	req3 := &providerpb.ConnectRequest{
		ProviderId: "",
	}
	resp3, err := svc.Connect(ctx, req3)
	require.NoError(t, err)
	assert.False(t, resp3.Success)
	assert.Contains(t, resp3.Error, "required")

	// 测试 nil 请求（应该失败）
	resp4, err := svc.Connect(ctx, nil)
	require.NoError(t, err)
	assert.False(t, resp4.Success)
	assert.Contains(t, resp4.Error, "nil")
}

// TestService_GetCapacity 测试 GetCapacity 功能
func TestService_GetCapacity(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	svc, err := createTestService()
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()

	req := &providerpb.GetCapacityRequest{}
	resp, err := svc.GetCapacity(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Capacity)

	// 验证容量信息
	assert.NotNil(t, resp.Capacity.Total)
	assert.NotNil(t, resp.Capacity.Used)
	assert.NotNil(t, resp.Capacity.Available)

	// 验证总容量大于 0
	assert.Greater(t, resp.Capacity.Total.Cpu, int64(0))
	assert.Greater(t, resp.Capacity.Total.Memory, int64(0))

	// 验证可用容量计算正确
	assert.Equal(t, resp.Capacity.Available.Cpu, resp.Capacity.Total.Cpu-resp.Capacity.Used.Cpu)
	assert.Equal(t, resp.Capacity.Available.Memory, resp.Capacity.Total.Memory-resp.Capacity.Used.Memory)
}

// TestService_GetAllocated 测试 GetAllocated 功能
func TestService_GetAllocated(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	svc, err := createTestService()
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()

	allocated, err := svc.GetAllocated(ctx)
	require.NoError(t, err)
	require.NotNil(t, allocated)

	// 验证已分配资源不为负数
	assert.GreaterOrEqual(t, allocated.Cpu, int64(0))
	assert.GreaterOrEqual(t, allocated.Memory, int64(0))
	assert.GreaterOrEqual(t, allocated.Gpu, int64(0))
}

// createTestService 创建测试用的 Service 实例
func createTestService() (*provider.Service, error) {
	// 尝试使用本地 Docker socket
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	return provider.NewService(host, "", false, "", "default", []string{"cpu", "memory", "camera"}, nil)
}

// isDockerAvailable 检查 Docker 是否可用
func isDockerAvailable() bool {
	svc, err := createTestService()
	if err != nil {
		return false
	}
	defer svc.Close()
	return true
}

// TestService_RPCIntegration 集成测试：启动 gRPC 服务器并通过 RPC 调用
func TestService_RPCIntegration(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	// 创建测试用的 Service
	svc, err := createTestService()
	require.NoError(t, err)
	defer svc.Close()

	// 启动 gRPC 服务器
	lis, err := net.Listen("tcp", ":0") // 使用随机端口
	require.NoError(t, err)

	port := lis.Addr().(*net.TCPAddr).Port
	address := fmt.Sprintf("localhost:%d", port)

	srv := grpc.NewServer()
	providerpb.RegisterServiceServer(srv, svc)

	// 在 goroutine 中启动服务器
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(lis); err != nil {
			serverErr <- err
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建 gRPC 客户端
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := providerpb.NewServiceClient(conn)

	// 测试 Connect RPC 调用
	t.Run("Connect via RPC", func(t *testing.T) {
		req := &providerpb.ConnectRequest{
			ProviderId: "rpc-test-provider-123",
		}

		resp, err := client.Connect(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Empty(t, resp.Error)
	})

	// 测试重复分配 ID（应该失败）
	t.Run("AssignID duplicate via RPC", func(t *testing.T) {
		req := &providerpb.ConnectRequest{
			ProviderId: "rpc-test-provider-123",
		}

		resp, err := client.Connect(ctx, req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Error, "already connected")
	})

	// 测试 GetCapacity RPC 调用
	t.Run("GetCapacity via RPC", func(t *testing.T) {
		req := &providerpb.GetCapacityRequest{}
		resp, err := client.GetCapacity(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Capacity)

		// 验证容量信息
		assert.NotNil(t, resp.Capacity.Total)
		assert.NotNil(t, resp.Capacity.Used)
		assert.NotNil(t, resp.Capacity.Available)

		// 验证总容量大于 0
		assert.Greater(t, resp.Capacity.Total.Cpu, int64(0))
		assert.Greater(t, resp.Capacity.Total.Memory, int64(0))

		// 验证可用容量计算正确
		assert.Equal(t, resp.Capacity.Available.Cpu, resp.Capacity.Total.Cpu-resp.Capacity.Used.Cpu)
		assert.Equal(t, resp.Capacity.Available.Memory, resp.Capacity.Total.Memory-resp.Capacity.Used.Memory)
	})

	// 测试空 ID（应该失败）
	t.Run("Connect empty via RPC", func(t *testing.T) {
		req := &providerpb.ConnectRequest{
			ProviderId: "",
		}

		resp, err := client.Connect(ctx, req)
		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Error, "required")
	})

	// 停止服务器
	srv.GracefulStop()

	// 检查服务器错误
	select {
	case err := <-serverErr:
		if err != nil {
			t.Logf("Server error (expected after shutdown): %v", err)
		}
	default:
		// 没有错误，正常
	}
}
