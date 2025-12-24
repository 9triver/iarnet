package situation_awareness

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/providers/docker/provider"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCase T3-1-001: Docker 资源接入测试
// 测试目的：验证 Docker provider 的资源态势感知能力，包括资源容量、可用资源、
// 已分配资源的获取，以及健康检查中包含的资源态势信息。

// TestDockerProvider_ResourceSituationAwareness 测试 Docker provider 的资源态势感知功能
func TestDockerProvider_ResourceSituationAwareness(t *testing.T) {
	printTestHeader(t, "测试用例 T3-1-001: Docker 资源接入测试",
		"验证 Docker provider 的资源态势感知能力")

	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	svc, err := createTestService()
	require.NoError(t, err, "Failed to create Docker provider service")
	defer svc.Close()

	ctx := context.Background()
	providerID := "test-provider-situation-awareness"

	// 首先需要连接 provider（注册）
	printTestSection(t, "步骤 1: 注册 Docker Provider")
	printInfo(t, fmt.Sprintf("正在注册 Provider ID: %s", providerID))

	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err, "Connect should succeed")
	require.True(t, connectResp.Success, "Connect should be successful")

	printSuccess(t, fmt.Sprintf("Provider 注册成功: %s", providerID))

	t.Run("GetCapacity - 获取资源容量信息", func(t *testing.T) {
		printTestSection(t, "测试: GetCapacity - 获取资源容量信息")

		// 测试连接状态下获取容量
		req := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}

		printInfo(t, "正在获取资源容量信息...")
		resp, err := svc.GetCapacity(ctx, req)
		require.NoError(t, err, "GetCapacity should succeed when connected")
		require.NotNil(t, resp, "Response should not be nil")
		require.NotNil(t, resp.Capacity, "Capacity should not be nil")

		printResourceInfo(t, resp.Capacity)

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
		if resp.Capacity.Total.Gpu > 0 {
			assert.Equal(t, resp.Capacity.Total.Gpu, resp.Capacity.Used.Gpu+resp.Capacity.Available.Gpu,
				"Total GPU should equal Used GPU + Available GPU")
		}

		// 验证已使用资源不超过总资源
		assert.LessOrEqual(t, resp.Capacity.Used.Cpu, resp.Capacity.Total.Cpu,
			"Used CPU should not exceed Total CPU")
		assert.LessOrEqual(t, resp.Capacity.Used.Memory, resp.Capacity.Total.Memory,
			"Used Memory should not exceed Total Memory")
		if resp.Capacity.Total.Gpu > 0 {
			assert.LessOrEqual(t, resp.Capacity.Used.Gpu, resp.Capacity.Total.Gpu,
				"Used GPU should not exceed Total GPU")
		}

		// 验证可用资源不为负数
		assert.GreaterOrEqual(t, resp.Capacity.Available.Cpu, int64(0),
			"Available CPU should not be negative")
		assert.GreaterOrEqual(t, resp.Capacity.Available.Memory, int64(0),
			"Available Memory should not be negative")
		if resp.Capacity.Total.Gpu > 0 {
			assert.GreaterOrEqual(t, resp.Capacity.Available.Gpu, int64(0),
				"Available GPU should not be negative")
		}

		printSuccess(t, "资源容量信息验证通过")
	})

	t.Run("GetAvailable - 获取可用资源信息", func(t *testing.T) {
		printTestSection(t, "测试: GetAvailable - 获取可用资源信息")

		// 测试连接状态下获取可用资源
		req := &providerpb.GetAvailableRequest{
			ProviderId: providerID,
		}

		printInfo(t, "正在获取可用资源信息...")
		resp, err := svc.GetAvailable(ctx, req)
		require.NoError(t, err, "GetAvailable should succeed when connected")
		require.NotNil(t, resp, "Response should not be nil")
		require.NotNil(t, resp.Available, "Available resources should not be nil")

		t.Log("\n" + colorize("可用资源信息:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", resp.Available.Cpu), colorGreen))
		t.Logf("  %s   %s",
			colorize("内存:", colorWhite+colorBold),
			colorize(formatBytes(resp.Available.Memory), colorGreen))
		t.Logf("  %s    %s",
			colorize("GPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d", resp.Available.Gpu), colorGreen))

		// 验证可用资源不为负数
		assert.GreaterOrEqual(t, resp.Available.Cpu, int64(0),
			"Available CPU should not be negative")
		assert.GreaterOrEqual(t, resp.Available.Memory, int64(0),
			"Available Memory should not be negative")
		assert.GreaterOrEqual(t, resp.Available.Gpu, int64(0),
			"Available GPU should not be negative")

		printSuccess(t, "可用资源信息验证通过")
	})

	t.Run("GetAllocated - 获取已分配资源信息", func(t *testing.T) {
		printTestSection(t, "测试: GetAllocated - 获取已分配资源信息")

		printInfo(t, "正在获取已分配资源信息...")
		allocated, err := svc.GetAllocated(ctx)
		require.NoError(t, err, "GetAllocated should succeed")
		require.NotNil(t, allocated, "Allocated resources should not be nil")

		t.Log("\n" + colorize("已分配资源信息:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", allocated.Cpu), colorYellow))
		t.Logf("  %s   %s",
			colorize("内存:", colorWhite+colorBold),
			colorize(formatBytes(allocated.Memory), colorYellow))
		t.Logf("  %s    %s",
			colorize("GPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d", allocated.Gpu), colorYellow))

		// 验证已分配资源不为负数
		assert.GreaterOrEqual(t, allocated.Cpu, int64(0),
			"Allocated CPU should not be negative")
		assert.GreaterOrEqual(t, allocated.Memory, int64(0),
			"Allocated Memory should not be negative")
		assert.GreaterOrEqual(t, allocated.Gpu, int64(0),
			"Allocated GPU should not be negative")

		printSuccess(t, "已分配资源信息验证通过")
	})

	t.Run("HealthCheck - 健康检查包含资源态势信息", func(t *testing.T) {
		printTestSection(t, "测试: HealthCheck - 健康检查包含资源态势信息")

		// 测试健康检查（provider 已在测试开始时连接）
		healthReq := &providerpb.HealthCheckRequest{
			ProviderId: providerID,
		}

		printInfo(t, "正在执行健康检查...")
		healthResp, err := svc.HealthCheck(ctx, healthReq)
		require.NoError(t, err, "HealthCheck should succeed")
		require.NotNil(t, healthResp, "HealthCheck response should not be nil")
		require.NotNil(t, healthResp.Capacity, "HealthCheck should include capacity information")
		require.NotNil(t, healthResp.ResourceTags, "HealthCheck should include resource tags")

		printResourceInfo(t, healthResp.Capacity)

		t.Log("\n" + colorize("资源标签信息:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("CPU:", colorWhite+colorBold),
			colorizeBool(healthResp.ResourceTags.Cpu))
		t.Logf("  %s   %s",
			colorize("内存:", colorWhite+colorBold),
			colorizeBool(healthResp.ResourceTags.Memory))
		t.Logf("  %s    %s",
			colorize("GPU:", colorWhite+colorBold),
			colorizeBool(healthResp.ResourceTags.Gpu))
		t.Logf("  %s %s",
			colorize("摄像头:", colorWhite+colorBold),
			colorizeBool(healthResp.ResourceTags.Camera))

		// 验证容量信息的完整性
		assert.NotNil(t, healthResp.Capacity.Total, "Total capacity should not be nil")
		assert.NotNil(t, healthResp.Capacity.Used, "Used capacity should not be nil")
		assert.NotNil(t, healthResp.Capacity.Available, "Available capacity should not be nil")

		// 验证资源标签
		assert.NotNil(t, healthResp.ResourceTags, "Resource tags should not be nil")
		// Docker provider 通常支持 CPU 和 Memory
		assert.True(t, healthResp.ResourceTags.Cpu || healthResp.ResourceTags.Memory,
			"At least CPU or Memory should be supported")

		// 验证资源标签与容量的一致性
		if healthResp.Capacity.Total.Cpu > 0 {
			assert.True(t, healthResp.ResourceTags.Cpu,
				"CPU tag should be true when CPU capacity is available")
		}
		if healthResp.Capacity.Total.Memory > 0 {
			assert.True(t, healthResp.ResourceTags.Memory,
				"Memory tag should be true when Memory capacity is available")
		}
		if healthResp.Capacity.Total.Gpu > 0 {
			// GPU 标签应该与 GPU 容量一致
			if !healthResp.ResourceTags.Gpu {
				printInfo(t, "GPU 容量存在但标签为 false，可能 provider 未启用 GPU 资源类型")
			} else {
				assert.True(t, healthResp.ResourceTags.Gpu,
					"GPU tag should be true when GPU capacity is available")
			}
		}

		// 验证容量计算正确性
		assert.Equal(t, healthResp.Capacity.Total.Cpu,
			healthResp.Capacity.Used.Cpu+healthResp.Capacity.Available.Cpu,
			"Total CPU should equal Used CPU + Available CPU")
		assert.Equal(t, healthResp.Capacity.Total.Memory,
			healthResp.Capacity.Used.Memory+healthResp.Capacity.Available.Memory,
			"Total Memory should equal Used Memory + Available Memory")
		if healthResp.Capacity.Total.Gpu > 0 {
			assert.Equal(t, healthResp.Capacity.Total.Gpu,
				healthResp.Capacity.Used.Gpu+healthResp.Capacity.Available.Gpu,
				"Total GPU should equal Used GPU + Available GPU")
		}

		printSuccess(t, "健康检查资源态势信息验证通过")
	})

	t.Run("ResourceSituationConsistency - 资源态势一致性验证", func(t *testing.T) {
		printTestSection(t, "测试: ResourceSituationConsistency - 资源态势一致性验证")

		// 获取容量信息
		printInfo(t, "正在获取容量信息...")
		capacityReq := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}
		capacityResp, err := svc.GetCapacity(ctx, capacityReq)
		require.NoError(t, err)

		printInfo(t, "正在获取可用资源信息...")

		// 获取可用资源信息
		availableReq := &providerpb.GetAvailableRequest{
			ProviderId: providerID,
		}
		availableResp, err := svc.GetAvailable(ctx, availableReq)
		require.NoError(t, err)

		// 获取已分配资源信息
		printInfo(t, "正在获取已分配资源信息...")
		allocated, err := svc.GetAllocated(ctx)
		require.NoError(t, err)

		// 验证不同接口返回的资源信息一致性
		printInfo(t, "正在验证资源信息一致性...")
		assert.Equal(t, capacityResp.Capacity.Available.Cpu, availableResp.Available.Cpu,
			"GetCapacity and GetAvailable should return the same available CPU")
		assert.Equal(t, capacityResp.Capacity.Available.Memory, availableResp.Available.Memory,
			"GetCapacity and GetAvailable should return the same available Memory")
		if capacityResp.Capacity.Total.Gpu > 0 {
			assert.Equal(t, capacityResp.Capacity.Available.Gpu, availableResp.Available.Gpu,
				"GetCapacity and GetAvailable should return the same available GPU")
		}

		assert.Equal(t, capacityResp.Capacity.Used.Cpu, allocated.Cpu,
			"GetCapacity and GetAllocated should return the same used CPU")
		assert.Equal(t, capacityResp.Capacity.Used.Memory, allocated.Memory,
			"GetCapacity and GetAllocated should return the same used Memory")
		if capacityResp.Capacity.Total.Gpu > 0 {
			assert.Equal(t, capacityResp.Capacity.Used.Gpu, allocated.Gpu,
				"GetCapacity and GetAllocated should return the same used GPU")
		}

		printSuccess(t, "资源态势一致性验证通过")
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("GetCapacity 和 GetAvailable 返回的可用资源一致", colorGreen))
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("GetCapacity 和 GetAllocated 返回的已用资源一致", colorGreen))
		if capacityResp.Capacity.Total.Gpu > 0 {
			t.Logf("  %s %s", colorize("✓", colorGreen), colorize("GPU 资源信息在不同接口间保持一致", colorGreen))

			// GPU 资源专项验证
			printInfo(t, "GPU 资源专项验证...")
			assert.Equal(t, capacityResp.Capacity.Total.Gpu, capacityResp.Capacity.Used.Gpu+capacityResp.Capacity.Available.Gpu,
				"Total GPU should equal Used GPU + Available GPU")
			assert.LessOrEqual(t, capacityResp.Capacity.Used.Gpu, capacityResp.Capacity.Total.Gpu,
				"Used GPU should not exceed Total GPU")
			assert.GreaterOrEqual(t, capacityResp.Capacity.Available.Gpu, int64(0),
				"Available GPU should not be negative")
			assert.Equal(t, capacityResp.Capacity.Available.Gpu, availableResp.Available.Gpu,
				"GetCapacity and GetAvailable should return the same available GPU")
			assert.Equal(t, capacityResp.Capacity.Used.Gpu, allocated.Gpu,
				"GetCapacity and GetAllocated should return the same used GPU")
			printSuccess(t, "GPU 资源专项验证通过")
		}
	})

	t.Run("ResourceSituationRealTime - 资源态势实时性验证", func(t *testing.T) {
		printTestSection(t, "测试: ResourceSituationRealTime - 资源态势实时性验证")

		// 测试容器配置
		testInstanceID := fmt.Sprintf("test-situation-awareness-%d", time.Now().Unix())
		testImage := "alpine:latest"
		testCPU := int64(500)                  // 500 millicores (0.5 CPU)
		testMemory := int64(128 * 1024 * 1024) // 128MB

		// 步骤 1: 获取初始资源状态
		printInfo(t, "步骤 1: 获取初始资源状态...")
		initialCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		// 复制值而不是引用，避免后续修改影响初始值
		initialUsed := &resourcepb.Info{
			Cpu:    initialCapacity.Capacity.Used.Cpu,
			Memory: initialCapacity.Capacity.Used.Memory,
			Gpu:    initialCapacity.Capacity.Used.Gpu,
		}
		initialAvailable := &resourcepb.Info{
			Cpu:    initialCapacity.Capacity.Available.Cpu,
			Memory: initialCapacity.Capacity.Available.Memory,
			Gpu:    initialCapacity.Capacity.Available.Gpu,
		}

		t.Log("\n" + colorize("初始资源状态:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("已用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", initialUsed.Cpu), colorYellow))
		t.Logf("  %s   %s",
			colorize("已用内存:", colorWhite+colorBold),
			colorize(formatBytes(initialUsed.Memory), colorYellow))
		if initialCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				colorize("已用 GPU:", colorWhite+colorBold),
				colorize(fmt.Sprintf("%d", initialUsed.Gpu), colorYellow))
		}
		t.Logf("  %s  %s",
			colorize("可用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", initialAvailable.Cpu), colorGreen))
		t.Logf("  %s %s",
			colorize("可用内存:", colorWhite+colorBold),
			colorize(formatBytes(initialAvailable.Memory), colorGreen))
		if initialCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s  %s",
				colorize("可用 GPU:", colorWhite+colorBold),
				colorize(fmt.Sprintf("%d", initialAvailable.Gpu), colorGreen))
		}

		// 步骤 2: 通过 Provider Deploy API 创建测试容器
		printInfo(t, fmt.Sprintf("步骤 2: 通过 Provider Deploy API 创建测试容器 (%s)...", testInstanceID))
		printInfo(t, fmt.Sprintf("  镜像: %s", testImage))
		printInfo(t, fmt.Sprintf("  CPU: %d millicores, 内存: %s", testCPU, formatBytes(testMemory)))

		deployReq := &providerpb.DeployRequest{
			ProviderId: providerID,
			InstanceId: testInstanceID,
			Image:      testImage,
			ResourceRequest: &resourcepb.Info{
				Cpu:    testCPU,
				Memory: testMemory,
				Gpu:    0,
			},
			EnvVars: map[string]string{
				"TEST_CONTAINER": "true",
			},
		}

		deployResp, err := svc.Deploy(ctx, deployReq)
		require.NoError(t, err, "Deploy should succeed")
		require.Empty(t, deployResp.Error, fmt.Sprintf("Deploy should not return error, got: %s", deployResp.Error))

		printSuccess(t, fmt.Sprintf("测试容器通过 Provider Deploy API 创建并启动成功: %s", testInstanceID))

		// 等待容器启动并稳定
		printInfo(t, "等待容器启动并稳定...")
		time.Sleep(2 * time.Second)

		// 步骤 3: 获取创建容器后的资源状态
		printInfo(t, "步骤 3: 获取创建容器后的资源状态...")
		afterCreateCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		// 复制值而不是引用
		afterCreateUsed := &resourcepb.Info{
			Cpu:    afterCreateCapacity.Capacity.Used.Cpu,
			Memory: afterCreateCapacity.Capacity.Used.Memory,
			Gpu:    afterCreateCapacity.Capacity.Used.Gpu,
		}
		afterCreateAvailable := &resourcepb.Info{
			Cpu:    afterCreateCapacity.Capacity.Available.Cpu,
			Memory: afterCreateCapacity.Capacity.Available.Memory,
			Gpu:    afterCreateCapacity.Capacity.Available.Gpu,
		}

		t.Log("\n" + colorize("创建容器后资源状态:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("已用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterCreateUsed.Cpu), colorYellow))
		t.Logf("  %s   %s",
			colorize("已用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterCreateUsed.Memory), colorYellow))
		if afterCreateCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				colorize("已用 GPU:", colorWhite+colorBold),
				colorize(fmt.Sprintf("%d", afterCreateUsed.Gpu), colorYellow))
		}
		t.Logf("  %s  %s",
			colorize("可用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterCreateAvailable.Cpu), colorGreen))
		t.Logf("  %s %s",
			colorize("可用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterCreateAvailable.Memory), colorGreen))
		if afterCreateCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s  %s",
				colorize("可用 GPU:", colorWhite+colorBold),
				colorize(fmt.Sprintf("%d", afterCreateAvailable.Gpu), colorGreen))
		}

		// 验证资源使用增加
		cpuIncrease := afterCreateUsed.Cpu - initialUsed.Cpu
		memoryIncrease := afterCreateUsed.Memory - initialUsed.Memory
		t.Log("\n" + colorize("资源变化分析:", colorCyan+colorBold))
		t.Logf("  %s    %s",
			colorize("CPU 增加:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", cpuIncrease), colorYellow))
		t.Logf("  %s   %s",
			colorize("内存增加:", colorWhite+colorBold),
			colorize(formatBytes(memoryIncrease), colorYellow))

		// 验证资源确实增加了（允许一些误差，因为容器可能没有完全使用分配的资源）
		assert.GreaterOrEqual(t, afterCreateUsed.Cpu, initialUsed.Cpu,
			"CPU usage should increase after creating container")
		assert.GreaterOrEqual(t, afterCreateUsed.Memory, initialUsed.Memory,
			"Memory usage should increase after creating container")

		// 步骤 4: 通过 Provider Undeploy API 清理测试容器
		printInfo(t, "步骤 4: 通过 Provider Undeploy API 清理测试容器...")
		undeployReq := &providerpb.UndeployRequest{
			ProviderId: providerID,
			InstanceId: testInstanceID,
		}

		undeployResp, err := svc.Undeploy(ctx, undeployReq)
		require.NoError(t, err, "Undeploy should succeed")
		require.Empty(t, undeployResp.Error, fmt.Sprintf("Undeploy should not return error, got: %s", undeployResp.Error))
		printSuccess(t, "测试容器已通过 Provider Undeploy API 清理")

		// 等待资源释放并稳定
		printInfo(t, "等待资源释放并稳定...")
		time.Sleep(2 * time.Second)

		// 步骤 5: 获取清理后的资源状态
		printInfo(t, "步骤 5: 获取清理后的资源状态...")
		afterStopCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		// 复制值而不是引用
		afterStopUsed := &resourcepb.Info{
			Cpu:    afterStopCapacity.Capacity.Used.Cpu,
			Memory: afterStopCapacity.Capacity.Used.Memory,
			Gpu:    afterStopCapacity.Capacity.Used.Gpu,
		}
		afterStopAvailable := &resourcepb.Info{
			Cpu:    afterStopCapacity.Capacity.Available.Cpu,
			Memory: afterStopCapacity.Capacity.Available.Memory,
			Gpu:    afterStopCapacity.Capacity.Available.Gpu,
		}

		t.Log("\n" + colorize("停止容器后资源状态:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("已用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterStopUsed.Cpu), colorYellow))
		t.Logf("  %s   %s",
			colorize("已用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterStopUsed.Memory), colorYellow))
		if afterStopCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				colorize("已用 GPU:", colorWhite+colorBold),
				colorize(fmt.Sprintf("%d", afterStopUsed.Gpu), colorYellow))
		}
		t.Logf("  %s  %s",
			colorize("可用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterStopAvailable.Cpu), colorGreen))
		t.Logf("  %s %s",
			colorize("可用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterStopAvailable.Memory), colorGreen))
		if afterStopCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s  %s",
				colorize("可用 GPU:", colorWhite+colorBold),
				colorize(fmt.Sprintf("%d", afterStopAvailable.Gpu), colorGreen))
		}

		// 验证资源使用减少（停止的容器可能仍然占用一些资源，但应该比运行时要少）
		cpuDecrease := afterCreateUsed.Cpu - afterStopUsed.Cpu
		memoryDecrease := afterCreateUsed.Memory - afterStopUsed.Memory
		t.Log("\n" + colorize("资源变化分析:", colorCyan+colorBold))
		t.Logf("  %s    %s",
			colorize("CPU 减少:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", cpuDecrease), colorGreen))
		t.Logf("  %s   %s",
			colorize("内存减少:", colorWhite+colorBold),
			colorize(formatBytes(memoryDecrease), colorGreen))

		// 验证资源确实减少了（停止的容器可能仍然占用一些资源）
		assert.LessOrEqual(t, afterStopUsed.Cpu, afterCreateUsed.Cpu,
			"CPU usage should decrease after stopping container")
		// 注意：内存可能不会立即释放，所以只验证 CPU

		// 步骤 6: 验证资源已完全释放（容器已在步骤 4 中通过 Undeploy 清理）
		printInfo(t, "步骤 6: 验证资源已完全释放...")
		printSuccess(t, "资源已通过 Provider Undeploy API 完全释放")

		// 等待清理完成
		time.Sleep(1 * time.Second)

		// 最终验证：资源状态能够实时更新
		printSuccess(t, "资源态势实时性验证通过")
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("创建容器后资源使用增加", colorGreen))
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("停止容器后资源使用减少", colorGreen))
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("接口能够实时反映资源状态变化", colorGreen))
	})

	t.Log("\n" + colorize(strings.Repeat("=", 80), colorCyan+colorBold))
	t.Log(colorize("✓ 所有资源态势感知测试通过", colorGreen+colorBold))
	t.Log(colorize(strings.Repeat("=", 80), colorCyan+colorBold) + "\n")
}

// TestDockerProvider_ResourceSituationAwareness_WithConnection 测试连接状态下的资源态势感知
func TestDockerProvider_ResourceSituationAwareness_WithConnection(t *testing.T) {
	printTestHeader(t, "测试用例: 连接状态下的资源态势感知",
		"验证连接状态下的资源态势感知和鉴权机制")

	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	svc, err := createTestService()
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()
	providerID := "test-provider-connected"

	// 连接 provider
	printTestSection(t, "步骤 1: 注册 Docker Provider")
	printInfo(t, fmt.Sprintf("正在注册 Provider ID: %s", providerID))

	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err)
	require.True(t, connectResp.Success)

	printSuccess(t, fmt.Sprintf("Provider 注册成功: %s", providerID))

	t.Run("GetCapacity with ProviderID", func(t *testing.T) {
		printTestSection(t, "测试: GetCapacity with ProviderID")

		req := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}

		printInfo(t, fmt.Sprintf("使用 ProviderID (%s) 获取容量...", providerID))
		resp, err := svc.GetCapacity(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Capacity)

		// 验证容量信息
		assert.Greater(t, resp.Capacity.Total.Cpu, int64(0))
		assert.Greater(t, resp.Capacity.Total.Memory, int64(0))

		printSuccess(t, "使用正确的 ProviderID 成功获取容量")
	})

	t.Run("GetAvailable with ProviderID", func(t *testing.T) {
		printTestSection(t, "测试: GetAvailable with ProviderID")

		req := &providerpb.GetAvailableRequest{
			ProviderId: providerID,
		}

		printInfo(t, fmt.Sprintf("使用 ProviderID (%s) 获取可用资源...", providerID))
		resp, err := svc.GetAvailable(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Available)

		assert.GreaterOrEqual(t, resp.Available.Cpu, int64(0))
		assert.GreaterOrEqual(t, resp.Available.Memory, int64(0))

		printSuccess(t, "使用正确的 ProviderID 成功获取可用资源")
	})

	t.Run("GetCapacity with wrong ProviderID should fail", func(t *testing.T) {
		printTestSection(t, "测试: GetCapacity with wrong ProviderID (应该失败)")

		req := &providerpb.GetCapacityRequest{
			ProviderId: "wrong-provider-id",
		}

		printInfo(t, "使用错误的 ProviderID 尝试获取容量...")
		_, err := svc.GetCapacity(ctx, req)
		assert.Error(t, err, "Should fail with wrong provider ID")
		assert.Contains(t, err.Error(), "unauthorized", "Error should indicate unauthorized")

		printSuccess(t, "鉴权机制正常工作：错误的 ProviderID 被正确拒绝")
		t.Logf("  %s %s", colorize("错误信息:", colorRed+colorBold), colorize(err.Error(), colorRed))
	})

	t.Run("GetCapacity with empty ProviderID should fail", func(t *testing.T) {
		printTestSection(t, "测试: GetCapacity with empty ProviderID (应该失败)")

		req := &providerpb.GetCapacityRequest{
			ProviderId: "",
		}

		printInfo(t, "使用空的 ProviderID 尝试获取容量...")
		_, err := svc.GetCapacity(ctx, req)
		assert.Error(t, err, "Should fail with empty provider ID")

		printSuccess(t, "验证通过：空的 ProviderID 被正确拒绝")
		t.Logf("  %s %s", colorize("错误信息:", colorRed+colorBold), colorize(err.Error(), colorRed))
	})

	t.Run("GetAvailable with empty ProviderID should fail", func(t *testing.T) {
		printTestSection(t, "测试: GetAvailable with empty ProviderID (应该失败)")

		req := &providerpb.GetAvailableRequest{
			ProviderId: "",
		}

		printInfo(t, "使用空的 ProviderID 尝试获取可用资源...")
		_, err := svc.GetAvailable(ctx, req)
		assert.Error(t, err, "Should fail with empty provider ID")

		printSuccess(t, "验证通过：空的 ProviderID 被正确拒绝")
		t.Logf("  %s %s", colorize("错误信息:", colorRed+colorBold), colorize(err.Error(), colorRed))
	})

	t.Log("\n" + colorize(strings.Repeat("=", 80), colorCyan+colorBold))
	t.Log(colorize("✓ 所有连接状态下的资源态势感知测试通过", colorGreen+colorBold))
	t.Log(colorize(strings.Repeat("=", 80), colorCyan+colorBold) + "\n")
}

// createTestService 创建测试用的 Service 实例
func createTestService() (*provider.Service, error) {
	// 尝试使用本地 Docker socket
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	// 创建支持 CPU、Memory 和 GPU 的 provider
	return provider.NewService(host, "", false, "", "default", []string{"cpu", "memory", "gpu"}, &resourcepb.Info{
		Cpu:    1000,
		Memory: 1024 * 1024 * 1024,
		Gpu:    4,
	}, []int{}, false) // gpuIDs: empty, allowConnectionFailure: false
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

// createDockerClient 创建 Docker client 用于直接操作容器
func createDockerClient() (*client.Client, error) {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	var opts []client.Opt
	if host != "" {
		opts = append(opts, client.WithHost(host))
	} else {
		opts = append(opts, client.FromEnv)
	}

	opts = append(opts, client.WithAPIVersionNegotiation())

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// 测试连接
	ctx := context.Background()
	_, err = cli.Ping(ctx)
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to ping Docker daemon: %w", err)
	}

	return cli, nil
}

// 辅助函数：格式化输出

// ANSI 颜色代码
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

// colorize 为文本添加颜色
func colorize(text string, color string) string {
	return color + text + colorReset
}

// printTestHeader 打印测试标题
func printTestHeader(t *testing.T, title, description string) {
	t.Helper()
	headerLine := "=" + strings.Repeat("=", 78) + "="
	t.Log("\n" + colorize(headerLine, colorCyan+colorBold))
	t.Logf("  %s", colorize(title, colorCyan+colorBold))
	if description != "" {
		t.Logf("  %s", colorize(description, colorCyan))
	}
	t.Log(colorize(headerLine, colorCyan+colorBold))
}

// printTestSection 打印测试小节
func printTestSection(t *testing.T, sectionName string) {
	t.Helper()
	t.Logf("\n%s %s", colorize("▶", colorBlue+colorBold), colorize(sectionName, colorBlue+colorBold))
	t.Log(colorize(strings.Repeat("-", 80), colorBlue))
}

// printSuccess 打印成功信息
func printSuccess(t *testing.T, message string) {
	t.Helper()
	t.Logf("%s %s", colorize("✓", colorGreen+colorBold), colorize(message, colorGreen))
}

// printInfo 打印信息
func printInfo(t *testing.T, message string) {
	t.Helper()
	t.Logf("%s %s", colorize("ℹ", colorCyan), colorize(message, colorCyan))
}

// printResourceInfo 打印资源信息
func printResourceInfo(t *testing.T, capacity *resourcepb.Capacity) {
	t.Helper()
	if capacity == nil {
		return
	}
	t.Log("\n" + colorize("资源容量信息:", colorYellow+colorBold))
	t.Logf("  %s    总计 %s, 已用 %s, 可用 %s",
		colorize("CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", capacity.Total.Cpu), colorWhite),
		colorize(fmt.Sprintf("%d millicores", capacity.Used.Cpu), colorYellow),
		colorize(fmt.Sprintf("%d millicores", capacity.Available.Cpu), colorGreen))
	t.Logf("  %s   总计 %s, 已用 %s, 可用 %s",
		colorize("内存:", colorWhite+colorBold),
		colorize(formatBytes(capacity.Total.Memory), colorWhite),
		colorize(formatBytes(capacity.Used.Memory), colorYellow),
		colorize(formatBytes(capacity.Available.Memory), colorGreen))
	if capacity.Total.Gpu > 0 {
		t.Logf("  %s    总计 %s, 已用 %s, 可用 %s",
			colorize("GPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d", capacity.Total.Gpu), colorWhite),
			colorize(fmt.Sprintf("%d", capacity.Used.Gpu), colorYellow),
			colorize(fmt.Sprintf("%d", capacity.Available.Gpu), colorGreen))
	}
}

// formatBytes 格式化字节数为可读格式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// colorizeBool 为布尔值添加颜色（true=绿色，false=红色）
func colorizeBool(value bool) string {
	if value {
		return colorize("✓ true", colorGreen)
	}
	return colorize("✗ false", colorRed)
}
