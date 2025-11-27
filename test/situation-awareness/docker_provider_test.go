package situation_awareness

import (
	"context"
	"os"
	"testing"
	"time"

	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/providers/docker/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCase T3-1-001: Docker 资源接入测试
// 测试目的：验证 Docker provider 的资源态势感知能力，包括资源容量、可用资源、
// 已分配资源的获取，以及健康检查中包含的资源态势信息。

// TestDockerProvider_ResourceSituationAwareness 测试 Docker provider 的资源态势感知功能
func TestDockerProvider_ResourceSituationAwareness(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	svc, err := createTestService()
	require.NoError(t, err, "Failed to create Docker provider service")
	defer svc.Close()

	ctx := context.Background()

	t.Run("GetCapacity - 获取资源容量信息", func(t *testing.T) {
		// 测试未连接状态下获取容量（应该允许）
		req := &providerpb.GetCapacityRequest{
			ProviderId: "", // 未连接状态
		}

		resp, err := svc.GetCapacity(ctx, req)
		require.NoError(t, err, "GetCapacity should succeed even when not connected")
		require.NotNil(t, resp, "Response should not be nil")
		require.NotNil(t, resp.Capacity, "Capacity should not be nil")

		// 验证容量信息的完整性
		assert.NotNil(t, resp.Capacity.Total, "Total capacity should not be nil")
		assert.NotNil(t, resp.Capacity.Used, "Used capacity should not be nil")
		assert.NotNil(t, resp.Capacity.Available, "Available capacity should not be nil")

		// 验证总容量大于 0
		assert.Greater(t, resp.Capacity.Total.Cpu, int64(0), "Total CPU should be greater than 0")
		assert.Greater(t, resp.Capacity.Total.Memory, int64(0), "Total Memory should be greater than 0")

		// 验证容量计算正确性：Total = Used + Available
		assert.Equal(t, resp.Capacity.Total.Cpu, resp.Capacity.Used.Cpu+resp.Capacity.Available.Cpu,
			"Total CPU should equal Used CPU + Available CPU")
		assert.Equal(t, resp.Capacity.Total.Memory, resp.Capacity.Used.Memory+resp.Capacity.Available.Memory,
			"Total Memory should equal Used Memory + Available Memory")

		// 验证已使用资源不超过总资源
		assert.LessOrEqual(t, resp.Capacity.Used.Cpu, resp.Capacity.Total.Cpu,
			"Used CPU should not exceed Total CPU")
		assert.LessOrEqual(t, resp.Capacity.Used.Memory, resp.Capacity.Total.Memory,
			"Used Memory should not exceed Total Memory")

		// 验证可用资源不为负数
		assert.GreaterOrEqual(t, resp.Capacity.Available.Cpu, int64(0),
			"Available CPU should not be negative")
		assert.GreaterOrEqual(t, resp.Capacity.Available.Memory, int64(0),
			"Available Memory should not be negative")
	})

	t.Run("GetAvailable - 获取可用资源信息", func(t *testing.T) {
		// 测试未连接状态下获取可用资源（应该允许）
		req := &providerpb.GetAvailableRequest{
			ProviderId: "", // 未连接状态
		}

		resp, err := svc.GetAvailable(ctx, req)
		require.NoError(t, err, "GetAvailable should succeed even when not connected")
		require.NotNil(t, resp, "Response should not be nil")
		require.NotNil(t, resp.Available, "Available resources should not be nil")

		// 验证可用资源不为负数
		assert.GreaterOrEqual(t, resp.Available.Cpu, int64(0),
			"Available CPU should not be negative")
		assert.GreaterOrEqual(t, resp.Available.Memory, int64(0),
			"Available Memory should not be negative")
		assert.GreaterOrEqual(t, resp.Available.Gpu, int64(0),
			"Available GPU should not be negative")
	})

	t.Run("GetAllocated - 获取已分配资源信息", func(t *testing.T) {
		allocated, err := svc.GetAllocated(ctx)
		require.NoError(t, err, "GetAllocated should succeed")
		require.NotNil(t, allocated, "Allocated resources should not be nil")

		// 验证已分配资源不为负数
		assert.GreaterOrEqual(t, allocated.Cpu, int64(0),
			"Allocated CPU should not be negative")
		assert.GreaterOrEqual(t, allocated.Memory, int64(0),
			"Allocated Memory should not be negative")
		assert.GreaterOrEqual(t, allocated.Gpu, int64(0),
			"Allocated GPU should not be negative")
	})

	t.Run("HealthCheck - 健康检查包含资源态势信息", func(t *testing.T) {
		// 首先需要连接 provider
		connectReq := &providerpb.ConnectRequest{
			ProviderId: "test-provider-situation-awareness",
		}
		connectResp, err := svc.Connect(ctx, connectReq)
		require.NoError(t, err, "Connect should succeed")
		require.True(t, connectResp.Success, "Connect should be successful")

		// 测试健康检查
		healthReq := &providerpb.HealthCheckRequest{
			ProviderId: "test-provider-situation-awareness",
		}

		healthResp, err := svc.HealthCheck(ctx, healthReq)
		require.NoError(t, err, "HealthCheck should succeed")
		require.NotNil(t, healthResp, "HealthCheck response should not be nil")
		require.NotNil(t, healthResp.Capacity, "HealthCheck should include capacity information")
		require.NotNil(t, healthResp.ResourceTags, "HealthCheck should include resource tags")

		// 验证容量信息的完整性
		assert.NotNil(t, healthResp.Capacity.Total, "Total capacity should not be nil")
		assert.NotNil(t, healthResp.Capacity.Used, "Used capacity should not be nil")
		assert.NotNil(t, healthResp.Capacity.Available, "Available capacity should not be nil")

		// 验证资源标签
		assert.NotNil(t, healthResp.ResourceTags, "Resource tags should not be nil")
		// Docker provider 通常支持 CPU 和 Memory
		assert.True(t, healthResp.ResourceTags.Cpu || healthResp.ResourceTags.Memory,
			"At least CPU or Memory should be supported")

		// 验证容量计算正确性
		assert.Equal(t, healthResp.Capacity.Total.Cpu,
			healthResp.Capacity.Used.Cpu+healthResp.Capacity.Available.Cpu,
			"Total CPU should equal Used CPU + Available CPU")
		assert.Equal(t, healthResp.Capacity.Total.Memory,
			healthResp.Capacity.Used.Memory+healthResp.Capacity.Available.Memory,
			"Total Memory should equal Used Memory + Available Memory")
	})

	t.Run("ResourceSituationConsistency - 资源态势一致性验证", func(t *testing.T) {
		// 获取容量信息
		capacityReq := &providerpb.GetCapacityRequest{
			ProviderId: "",
		}
		capacityResp, err := svc.GetCapacity(ctx, capacityReq)
		require.NoError(t, err)

		// 获取可用资源信息
		availableReq := &providerpb.GetAvailableRequest{
			ProviderId: "",
		}
		availableResp, err := svc.GetAvailable(ctx, availableReq)
		require.NoError(t, err)

		// 获取已分配资源信息
		allocated, err := svc.GetAllocated(ctx)
		require.NoError(t, err)

		// 验证不同接口返回的资源信息一致性
		assert.Equal(t, capacityResp.Capacity.Available.Cpu, availableResp.Available.Cpu,
			"GetCapacity and GetAvailable should return the same available CPU")
		assert.Equal(t, capacityResp.Capacity.Available.Memory, availableResp.Available.Memory,
			"GetCapacity and GetAvailable should return the same available Memory")

		assert.Equal(t, capacityResp.Capacity.Used.Cpu, allocated.Cpu,
			"GetCapacity and GetAllocated should return the same used CPU")
		assert.Equal(t, capacityResp.Capacity.Used.Memory, allocated.Memory,
			"GetCapacity and GetAllocated should return the same used Memory")
	})

	t.Run("ResourceSituationRealTime - 资源态势实时性验证", func(t *testing.T) {
		// 获取初始资源状态
		initialCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{})
		require.NoError(t, err)
		_ = initialCapacity.Capacity.Used // 记录初始状态用于参考

		// 等待一小段时间
		time.Sleep(100 * time.Millisecond)

		// 再次获取资源状态
		secondCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{})
		require.NoError(t, err)
		secondUsed := secondCapacity.Capacity.Used

		// 验证资源状态能够实时更新（至少接口调用成功）
		// 注意：在实际环境中，如果资源没有变化，used 值应该相同
		// 这里主要验证接口能够正常返回实时数据
		assert.NotNil(t, secondUsed, "Second capacity check should return valid used resources")
		assert.GreaterOrEqual(t, secondUsed.Cpu, int64(0),
			"Used CPU should be non-negative")
		assert.GreaterOrEqual(t, secondUsed.Memory, int64(0),
			"Used Memory should be non-negative")
	})
}

// TestDockerProvider_ResourceSituationAwareness_WithConnection 测试连接状态下的资源态势感知
func TestDockerProvider_ResourceSituationAwareness_WithConnection(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	svc, err := createTestService()
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()
	providerID := "test-provider-connected"

	// 连接 provider
	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err)
	require.True(t, connectResp.Success)

	t.Run("GetCapacity with ProviderID", func(t *testing.T) {
		req := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}

		resp, err := svc.GetCapacity(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Capacity)

		// 验证容量信息
		assert.Greater(t, resp.Capacity.Total.Cpu, int64(0))
		assert.Greater(t, resp.Capacity.Total.Memory, int64(0))
	})

	t.Run("GetAvailable with ProviderID", func(t *testing.T) {
		req := &providerpb.GetAvailableRequest{
			ProviderId: providerID,
		}

		resp, err := svc.GetAvailable(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Available)

		assert.GreaterOrEqual(t, resp.Available.Cpu, int64(0))
		assert.GreaterOrEqual(t, resp.Available.Memory, int64(0))
	})

	t.Run("GetCapacity with wrong ProviderID should fail", func(t *testing.T) {
		req := &providerpb.GetCapacityRequest{
			ProviderId: "wrong-provider-id",
		}

		_, err := svc.GetCapacity(ctx, req)
		assert.Error(t, err, "Should fail with wrong provider ID")
		assert.Contains(t, err.Error(), "unauthorized", "Error should indicate unauthorized")
	})
}

// createTestService 创建测试用的 Service 实例
func createTestService() (*provider.Service, error) {
	// 尝试使用本地 Docker socket
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	// 创建支持 CPU 和 Memory 的 provider
	return provider.NewService(host, "", false, "", []string{"cpu", "memory"})
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
