package situation_awareness

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	testutil "github.com/9triver/iarnet/test/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	// 初始化测试 logger，时间戳提前6小时
	testutil.InitTestLogger()
}

// TestCase T3-1-002: Kubernetes 资源接入测试
// 测试目的：验证 Kubernetes provider 的资源态势感知能力，包括资源容量、可用资源、
// 已分配资源的获取，以及健康检查中包含的资源态势信息。

// TestK8sProvider_ResourceSituationAwareness 测试 Kubernetes provider 的资源态势感知功能
func TestK8sProvider_ResourceSituationAwareness(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例 T3-1-002: Kubernetes 资源接入测试",
		"验证 Kubernetes provider 的资源态势感知能力")

	if !testutil.IsK8sAvailable() {
		t.Skip("Kubernetes is not available, skipping test")
	}

	svc, err := testutil.CreateK8sTestService()
	require.NoError(t, err, "Failed to create Kubernetes provider service")
	defer svc.Close()

	ctx := context.Background()
	providerID := "test-k8s-provider-situation-awareness"

	// 首先需要连接 provider（注册）
	testutil.PrintTestSection(t, "步骤 1: 注册 Kubernetes Provider")
	testutil.PrintInfo(t, fmt.Sprintf("正在注册 Provider ID: %s", providerID))

	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err, "Connect should succeed")
	require.True(t, connectResp.Success, "Connect should be successful")

	testutil.PrintSuccess(t, fmt.Sprintf("Provider 注册成功: %s", providerID))
	testutil.PrintInfo(t, fmt.Sprintf("Provider 类型: %s", connectResp.ProviderType.Name))

	t.Run("GetCapacity - 获取资源容量信息", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetCapacity - 获取资源容量信息")

		// 测试连接状态下获取容量
		req := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}

		testutil.PrintInfo(t, "正在获取资源容量信息...")
		resp, err := svc.GetCapacity(ctx, req)
		require.NoError(t, err, "GetCapacity should succeed when connected")
		require.NotNil(t, resp, "Response should not be nil")
		require.NotNil(t, resp.Capacity, "Capacity should not be nil")

		testutil.PrintResourceInfo(t, resp.Capacity)

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

		testutil.PrintSuccess(t, "资源容量信息验证通过")
	})

	t.Run("GetAvailable - 获取可用资源信息", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetAvailable - 获取可用资源信息")

		// 测试连接状态下获取可用资源
		req := &providerpb.GetAvailableRequest{
			ProviderId: providerID,
		}

		testutil.PrintInfo(t, "正在获取可用资源信息...")
		resp, err := svc.GetAvailable(ctx, req)
		require.NoError(t, err, "GetAvailable should succeed when connected")
		require.NotNil(t, resp, "Response should not be nil")
		require.NotNil(t, resp.Available, "Available resources should not be nil")

		t.Log("\n" + testutil.Colorize("可用资源信息:", testutil.ColorYellow+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", resp.Available.Cpu), testutil.ColorGreen))
		t.Logf("  %s   %s",
			testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(resp.Available.Memory), testutil.ColorGreen))
		t.Logf("  %s    %s",
			testutil.Colorize("GPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d", resp.Available.Gpu), testutil.ColorGreen))

		// 验证可用资源不为负数
		assert.GreaterOrEqual(t, resp.Available.Cpu, int64(0),
			"Available CPU should not be negative")
		assert.GreaterOrEqual(t, resp.Available.Memory, int64(0),
			"Available Memory should not be negative")
		assert.GreaterOrEqual(t, resp.Available.Gpu, int64(0),
			"Available GPU should not be negative")

		testutil.PrintSuccess(t, "可用资源信息验证通过")
	})

	t.Run("GetAllocated - 获取已分配资源信息", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetAllocated - 获取已分配资源信息")

		testutil.PrintInfo(t, "正在获取已分配资源信息...")
		allocated, err := svc.GetAllocated(ctx)
		require.NoError(t, err, "GetAllocated should succeed")
		require.NotNil(t, allocated, "Allocated resources should not be nil")

		t.Log("\n" + testutil.Colorize("已分配资源信息:", testutil.ColorYellow+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", allocated.Cpu), testutil.ColorYellow))
		t.Logf("  %s   %s",
			testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(allocated.Memory), testutil.ColorYellow))
		t.Logf("  %s    %s",
			testutil.Colorize("GPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d", allocated.Gpu), testutil.ColorYellow))

		// 验证已分配资源不为负数
		assert.GreaterOrEqual(t, allocated.Cpu, int64(0),
			"Allocated CPU should not be negative")
		assert.GreaterOrEqual(t, allocated.Memory, int64(0),
			"Allocated Memory should not be negative")
		assert.GreaterOrEqual(t, allocated.Gpu, int64(0),
			"Allocated GPU should not be negative")

		testutil.PrintSuccess(t, "已分配资源信息验证通过")
	})

	t.Run("HealthCheck - 健康检查包含资源态势信息", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: HealthCheck - 健康检查包含资源态势信息")

		// 测试健康检查（provider 已在测试开始时连接）
		healthReq := &providerpb.HealthCheckRequest{
			ProviderId: providerID,
		}

		testutil.PrintInfo(t, "正在执行健康检查...")
		healthResp, err := svc.HealthCheck(ctx, healthReq)
		require.NoError(t, err, "HealthCheck should succeed")
		require.NotNil(t, healthResp, "HealthCheck response should not be nil")
		require.NotNil(t, healthResp.Capacity, "HealthCheck should include capacity information")
		require.NotNil(t, healthResp.ResourceTags, "HealthCheck should include resource tags")

		testutil.PrintResourceInfo(t, healthResp.Capacity)

		t.Log("\n" + testutil.Colorize("资源标签信息:", testutil.ColorYellow+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.ColorizeBool(healthResp.ResourceTags.Cpu))
		t.Logf("  %s   %s",
			testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.ColorizeBool(healthResp.ResourceTags.Memory))
		t.Logf("  %s    %s",
			testutil.Colorize("GPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.ColorizeBool(healthResp.ResourceTags.Gpu))
		t.Logf("  %s %s",
			testutil.Colorize("摄像头:", testutil.ColorWhite+testutil.ColorBold),
			testutil.ColorizeBool(healthResp.ResourceTags.Camera))

		// 验证容量信息的完整性
		assert.NotNil(t, healthResp.Capacity.Total, "Total capacity should not be nil")
		assert.NotNil(t, healthResp.Capacity.Used, "Used capacity should not be nil")
		assert.NotNil(t, healthResp.Capacity.Available, "Available capacity should not be nil")

		// 验证资源标签
		assert.NotNil(t, healthResp.ResourceTags, "Resource tags should not be nil")
		// Kubernetes provider 通常支持 CPU、Memory 和 GPU
		assert.True(t, healthResp.ResourceTags.Cpu || healthResp.ResourceTags.Memory || healthResp.ResourceTags.Gpu,
			"At least CPU, Memory or GPU should be supported")

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
			// GPU 标签应该与 GPU 容量一致（已启用 GPU 资源类型）
			assert.True(t, healthResp.ResourceTags.Gpu,
				"GPU tag should be true when GPU capacity is available and GPU resource type is enabled")
		}

		// 验证容量计算正确性
		assert.Equal(t, healthResp.Capacity.Total.Cpu,
			healthResp.Capacity.Used.Cpu+healthResp.Capacity.Available.Cpu,
			"Total CPU should equal Used CPU + Available CPU")
		assert.Equal(t, healthResp.Capacity.Total.Memory,
			healthResp.Capacity.Used.Memory+healthResp.Capacity.Available.Memory,
			"Total Memory should equal Used Memory + Available Memory")

		testutil.PrintSuccess(t, "健康检查资源态势信息验证通过")
	})

	t.Run("ResourceSituationConsistency - 资源态势一致性验证", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: ResourceSituationConsistency - 资源态势一致性验证")

		// 获取容量信息
		testutil.PrintInfo(t, "正在获取容量信息...")
		capacityReq := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}
		capacityResp, err := svc.GetCapacity(ctx, capacityReq)
		require.NoError(t, err)

		testutil.PrintInfo(t, "正在获取可用资源信息...")

		// 获取可用资源信息
		availableReq := &providerpb.GetAvailableRequest{
			ProviderId: providerID,
		}
		availableResp, err := svc.GetAvailable(ctx, availableReq)
		require.NoError(t, err)

		// 获取已分配资源信息
		testutil.PrintInfo(t, "正在获取已分配资源信息...")
		allocated, err := svc.GetAllocated(ctx)
		require.NoError(t, err)

		// 验证不同接口返回的资源信息一致性
		testutil.PrintInfo(t, "正在验证资源信息一致性...")
		assert.Equal(t, capacityResp.Capacity.Available.Cpu, availableResp.Available.Cpu,
			"GetCapacity and GetAvailable should return the same available CPU")
		assert.Equal(t, capacityResp.Capacity.Available.Memory, availableResp.Available.Memory,
			"GetCapacity and GetAvailable should return the same available Memory")

		assert.Equal(t, capacityResp.Capacity.Used.Cpu, allocated.Cpu,
			"GetCapacity and GetAllocated should return the same used CPU")
		assert.Equal(t, capacityResp.Capacity.Used.Memory, allocated.Memory,
			"GetCapacity and GetAllocated should return the same used Memory")

		testutil.PrintSuccess(t, "资源态势一致性验证通过")
		t.Logf("  %s %s", testutil.Colorize("✓", testutil.ColorGreen), testutil.Colorize("GetCapacity 和 GetAvailable 返回的可用资源一致", testutil.ColorGreen))
		t.Logf("  %s %s", testutil.Colorize("✓", testutil.ColorGreen), testutil.Colorize("GetCapacity 和 GetAllocated 返回的已用资源一致", testutil.ColorGreen))
	})

	t.Run("ResourceSituationRealTime - 资源态势实时性验证", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: ResourceSituationRealTime - 资源态势实时性验证")

		// 测试 Pod 配置
		testPodName := fmt.Sprintf("test-situation-awareness-%d", time.Now().Unix())
		testNamespace := "default"
		testImage := "busybox:latest"
		testCPU := int64(100)                 // 100 millicores (0.1 CPU)
		testMemory := int64(64 * 1024 * 1024) // 64MB
		testGPU := int64(1)                   // 1 GPU

		// 步骤 1: 获取初始资源状态
		testutil.PrintInfo(t, "步骤 1: 获取初始资源状态...")
		initialCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		initialUsed := initialCapacity.Capacity.Used
		initialAvailable := initialCapacity.Capacity.Available

		t.Log("\n" + testutil.Colorize("初始资源状态:", testutil.ColorYellow+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("已用 CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", initialUsed.Cpu), testutil.ColorYellow))
		t.Logf("  %s   %s",
			testutil.Colorize("已用内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(initialUsed.Memory), testutil.ColorYellow))
		if initialCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				testutil.Colorize("已用 GPU:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", initialUsed.Gpu), testutil.ColorYellow))
		}
		t.Logf("  %s  %s",
			testutil.Colorize("可用 CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", initialAvailable.Cpu), testutil.ColorGreen))
		t.Logf("  %s %s",
			testutil.Colorize("可用内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(initialAvailable.Memory), testutil.ColorGreen))
		if initialCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s  %s",
				testutil.Colorize("可用 GPU:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", initialAvailable.Gpu), testutil.ColorGreen))
		}

		// 步骤 2: 通过 provider API 创建测试 Pod（使用 Deploy 方法）
		testutil.PrintInfo(t, fmt.Sprintf("步骤 2: 通过 Provider API 创建测试 Pod (%s)...", testPodName))
		testutil.PrintInfo(t, fmt.Sprintf("  镜像: %s (使用 PullNever 策略，需要预先加载到 kind 集群)", testImage))
		testutil.PrintInfo(t, fmt.Sprintf("  CPU: %d millicores, 内存: %s, GPU: %d", testCPU, testutil.FormatBytes(testMemory), testGPU))
		testutil.PrintInfo(t, "  提示: 如果镜像未加载，请运行: kind load docker-image busybox:latest")
		testutil.PrintInfo(t, "  重要: 使用 Provider API (Deploy) 创建 Pod，确保资源使用情况被正确跟踪")

		// 使用 provider 的 Deploy API 创建 Pod，这样 provider 才能在内存中跟踪资源使用
		deployReq := &providerpb.DeployRequest{
			ProviderId: providerID,
			InstanceId: testPodName,
			Image:      testImage,
			ResourceRequest: &resourcepb.Info{
				Cpu:    testCPU,
				Memory: testMemory,
				Gpu:    testGPU,
			},
			EnvVars: map[string]string{
				"TEST": "situation-awareness",
			},
		}

		deployResp, err := svc.Deploy(ctx, deployReq)
		require.NoError(t, err, "Failed to deploy Pod via provider API")
		require.Empty(t, deployResp.Error, "Deploy should succeed without error: %s", deployResp.Error)

		testutil.PrintSuccess(t, fmt.Sprintf("测试 Pod 通过 Provider API 部署成功: %s/%s", testNamespace, testPodName))

		// 步骤 3: 立即获取创建 Pod 后的资源状态（资源在 Deploy 时已在内存中更新）
		testutil.PrintInfo(t, "步骤 3: 获取创建 Pod 后的资源状态（Deploy 后立即查询）...")

		afterCreateCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		afterCreateUsed := afterCreateCapacity.Capacity.Used
		afterCreateAvailable := afterCreateCapacity.Capacity.Available

		t.Log("\n" + testutil.Colorize("创建 Pod 后资源状态:", testutil.ColorYellow+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("已用 CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", afterCreateUsed.Cpu), testutil.ColorYellow))
		t.Logf("  %s   %s",
			testutil.Colorize("已用内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(afterCreateUsed.Memory), testutil.ColorYellow))
		if afterCreateCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				testutil.Colorize("已用 GPU:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", afterCreateUsed.Gpu), testutil.ColorYellow))
		}
		t.Logf("  %s  %s",
			testutil.Colorize("可用 CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", afterCreateAvailable.Cpu), testutil.ColorGreen))
		t.Logf("  %s %s",
			testutil.Colorize("可用内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(afterCreateAvailable.Memory), testutil.ColorGreen))
		if afterCreateCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s  %s",
				testutil.Colorize("可用 GPU:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", afterCreateAvailable.Gpu), testutil.ColorGreen))
		}

		// 验证资源使用增加
		cpuIncrease := afterCreateUsed.Cpu - initialUsed.Cpu
		memoryIncrease := afterCreateUsed.Memory - initialUsed.Memory
		gpuIncrease := afterCreateUsed.Gpu - initialUsed.Gpu
		t.Log("\n" + testutil.Colorize("资源变化分析:", testutil.ColorCyan+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("CPU 增加:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", cpuIncrease), testutil.ColorYellow))
		t.Logf("  %s   %s",
			testutil.Colorize("内存增加:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(memoryIncrease), testutil.ColorYellow))
		if afterCreateCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				testutil.Colorize("GPU 增加:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", gpuIncrease), testutil.ColorYellow))
		}

		// 验证资源确实增加了
		// 资源使用量在 Deploy 时已在 provider 内存中立即更新，不依赖 Pod 状态
		assert.GreaterOrEqual(t, afterCreateUsed.Cpu, initialUsed.Cpu,
			"CPU usage should increase immediately after Deploy (tracked in provider memory)")
		assert.GreaterOrEqual(t, afterCreateUsed.Memory, initialUsed.Memory,
			"Memory usage should increase immediately after Deploy (tracked in provider memory)")
		if afterCreateCapacity.Capacity.Total.Gpu > 0 {
			assert.GreaterOrEqual(t, afterCreateUsed.Gpu, initialUsed.Gpu,
				"GPU usage should increase immediately after Deploy (tracked in provider memory)")
		}

		// 验证资源增加量是否正确
		expectedCpuIncrease := testCPU
		expectedMemoryIncrease := testMemory
		expectedGpuIncrease := testGPU

		assert.Equal(t, expectedCpuIncrease, cpuIncrease,
			"CPU increase should match the requested amount")
		assert.Equal(t, expectedMemoryIncrease, memoryIncrease,
			"Memory increase should match the requested amount")
		if afterCreateCapacity.Capacity.Total.Gpu > 0 {
			assert.Equal(t, expectedGpuIncrease, gpuIncrease,
				"GPU increase should match the requested amount")
		}

		// 步骤 4: 通过 provider API 删除 Pod（使用 Undeploy 方法）
		testutil.PrintInfo(t, "步骤 4: 通过 Provider API 删除测试 Pod...")
		testutil.PrintInfo(t, "  重要: 使用 Provider API (Undeploy) 删除 Pod，确保资源使用情况被正确释放")

		undeployReq := &providerpb.UndeployRequest{
			ProviderId: providerID,
			InstanceId: testPodName,
		}

		undeployResp, err := svc.Undeploy(ctx, undeployReq)
		require.NoError(t, err, "Failed to undeploy Pod via provider API")
		require.Empty(t, undeployResp.Error, "Undeploy should succeed without error: %s", undeployResp.Error)

		testutil.PrintSuccess(t, fmt.Sprintf("测试 Pod 通过 Provider API 删除成功: %s/%s", testNamespace, testPodName))
		testutil.PrintInfo(t, "注意: 资源使用量已在 provider 内存中立即释放，无需等待 Pod 删除完成")

		// 步骤 5: 立即获取删除 Pod 后的资源状态（资源在 Undeploy 时已在内存中释放）
		testutil.PrintInfo(t, "步骤 5: 获取删除 Pod 后的资源状态（Undeploy 后立即查询）...")
		afterDeleteCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		afterDeleteUsed := afterDeleteCapacity.Capacity.Used
		afterDeleteAvailable := afterDeleteCapacity.Capacity.Available

		t.Log("\n" + testutil.Colorize("删除 Pod 后资源状态:", testutil.ColorYellow+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("已用 CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", afterDeleteUsed.Cpu), testutil.ColorYellow))
		t.Logf("  %s   %s",
			testutil.Colorize("已用内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(afterDeleteUsed.Memory), testutil.ColorYellow))
		if afterDeleteCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				testutil.Colorize("已用 GPU:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", afterDeleteUsed.Gpu), testutil.ColorYellow))
		}
		t.Logf("  %s  %s",
			testutil.Colorize("可用 CPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", afterDeleteAvailable.Cpu), testutil.ColorGreen))
		t.Logf("  %s %s",
			testutil.Colorize("可用内存:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(afterDeleteAvailable.Memory), testutil.ColorGreen))
		if afterDeleteCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s  %s",
				testutil.Colorize("可用 GPU:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", afterDeleteAvailable.Gpu), testutil.ColorGreen))
		}

		// 验证资源使用减少
		cpuDecrease := afterCreateUsed.Cpu - afterDeleteUsed.Cpu
		memoryDecrease := afterCreateUsed.Memory - afterDeleteUsed.Memory
		gpuDecrease := afterCreateUsed.Gpu - afterDeleteUsed.Gpu
		t.Log("\n" + testutil.Colorize("资源变化分析:", testutil.ColorCyan+testutil.ColorBold))
		t.Logf("  %s    %s",
			testutil.Colorize("CPU 减少:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d millicores", cpuDecrease), testutil.ColorGreen))
		t.Logf("  %s   %s",
			testutil.Colorize("内存减少:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(testutil.FormatBytes(memoryDecrease), testutil.ColorGreen))
		if afterDeleteCapacity.Capacity.Total.Gpu > 0 {
			t.Logf("  %s    %s",
				testutil.Colorize("GPU 减少:", testutil.ColorWhite+testutil.ColorBold),
				testutil.Colorize(fmt.Sprintf("%d", gpuDecrease), testutil.ColorGreen))
		}

		// 验证资源确实减少了
		// 资源使用量在 Undeploy 时已在 provider 内存中立即释放，不依赖 Pod 删除状态
		assert.LessOrEqual(t, afterDeleteUsed.Cpu, afterCreateUsed.Cpu,
			"CPU usage should decrease immediately after Undeploy (tracked in provider memory)")
		assert.LessOrEqual(t, afterDeleteUsed.Memory, afterCreateUsed.Memory,
			"Memory usage should decrease immediately after Undeploy (tracked in provider memory)")
		if afterDeleteCapacity.Capacity.Total.Gpu > 0 {
			assert.LessOrEqual(t, afterDeleteUsed.Gpu, afterCreateUsed.Gpu,
				"GPU usage should decrease immediately after Undeploy (tracked in provider memory)")
		}

		// 验证资源减少量是否正确（应该等于创建时增加的量）
		assert.Equal(t, cpuIncrease, cpuDecrease,
			"CPU decrease should match the increase amount")
		assert.Equal(t, memoryIncrease, memoryDecrease,
			"Memory decrease should match the increase amount")
		if afterDeleteCapacity.Capacity.Total.Gpu > 0 {
			assert.Equal(t, gpuIncrease, gpuDecrease,
				"GPU decrease should match the increase amount")
		}

		// 验证资源恢复到初始状态
		assert.Equal(t, initialUsed.Cpu, afterDeleteUsed.Cpu,
			"CPU usage should return to initial state after undeploy")
		assert.Equal(t, initialUsed.Memory, afterDeleteUsed.Memory,
			"Memory usage should return to initial state after undeploy")
		if afterDeleteCapacity.Capacity.Total.Gpu > 0 {
			assert.Equal(t, initialUsed.Gpu, afterDeleteUsed.Gpu,
				"GPU usage should return to initial state after undeploy")
		}

		// 最终验证：资源状态能够实时更新
		testutil.PrintSuccess(t, "资源态势实时性验证通过")
		t.Logf("  %s %s", testutil.Colorize("✓", testutil.ColorGreen), testutil.Colorize("创建 Pod 后资源使用增加", testutil.ColorGreen))
		t.Logf("  %s %s", testutil.Colorize("✓", testutil.ColorGreen), testutil.Colorize("删除 Pod 后资源使用减少", testutil.ColorGreen))
		t.Logf("  %s %s", testutil.Colorize("✓", testutil.ColorGreen), testutil.Colorize("接口能够实时反映资源状态变化", testutil.ColorGreen))
	})

	t.Log("\n" + testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold))
	t.Log(testutil.Colorize("✓ 所有 Kubernetes 资源态势感知测试通过", testutil.ColorGreen+testutil.ColorBold))
	t.Log(testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold) + "\n")
}

// TestK8sProvider_ResourceSituationAwareness_WithConnection 测试连接状态下的资源态势感知
func TestK8sProvider_ResourceSituationAwareness_WithConnection(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: Kubernetes 连接状态下的资源态势感知",
		"验证连接状态下的资源态势感知和鉴权机制")

	if !testutil.IsK8sAvailable() {
		t.Skip("Kubernetes is not available, skipping test")
	}

	svc, err := testutil.CreateK8sTestService()
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()
	providerID := "test-k8s-provider-connected"

	// 连接 provider
	testutil.PrintTestSection(t, "步骤 1: 注册 Kubernetes Provider")
	testutil.PrintInfo(t, fmt.Sprintf("正在注册 Provider ID: %s", providerID))

	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err)
	require.True(t, connectResp.Success)

	testutil.PrintSuccess(t, fmt.Sprintf("Provider 注册成功: %s", providerID))

	t.Run("GetCapacity with ProviderID", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetCapacity with ProviderID")

		req := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}

		testutil.PrintInfo(t, fmt.Sprintf("使用 ProviderID (%s) 获取容量...", providerID))
		resp, err := svc.GetCapacity(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Capacity)

		// 验证容量信息
		assert.Greater(t, resp.Capacity.Total.Cpu, int64(0))
		assert.Greater(t, resp.Capacity.Total.Memory, int64(0))

		testutil.PrintSuccess(t, "使用正确的 ProviderID 成功获取容量")
	})

	t.Run("GetAvailable with ProviderID", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetAvailable with ProviderID")

		req := &providerpb.GetAvailableRequest{
			ProviderId: providerID,
		}

		testutil.PrintInfo(t, fmt.Sprintf("使用 ProviderID (%s) 获取可用资源...", providerID))
		resp, err := svc.GetAvailable(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Available)

		assert.GreaterOrEqual(t, resp.Available.Cpu, int64(0))
		assert.GreaterOrEqual(t, resp.Available.Memory, int64(0))

		testutil.PrintSuccess(t, "使用正确的 ProviderID 成功获取可用资源")
	})

	t.Run("GetCapacity with wrong ProviderID should fail", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetCapacity with wrong ProviderID (应该失败)")

		req := &providerpb.GetCapacityRequest{
			ProviderId: "wrong-provider-id",
		}

		testutil.PrintInfo(t, "使用错误的 ProviderID 尝试获取容量...")
		_, err := svc.GetCapacity(ctx, req)
		assert.Error(t, err, "Should fail with wrong provider ID")
		assert.Contains(t, err.Error(), "unauthorized", "Error should indicate unauthorized")

		testutil.PrintSuccess(t, "鉴权机制正常工作：错误的 ProviderID 被正确拒绝")
		t.Logf("  %s %s", testutil.Colorize("错误信息:", testutil.ColorRed+testutil.ColorBold), testutil.Colorize(err.Error(), testutil.ColorRed))
	})

	t.Run("GetCapacity with empty ProviderID should fail", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetCapacity with empty ProviderID (应该失败)")

		req := &providerpb.GetCapacityRequest{
			ProviderId: "",
		}

		testutil.PrintInfo(t, "使用空的 ProviderID 尝试获取容量...")
		_, err := svc.GetCapacity(ctx, req)
		assert.Error(t, err, "Should fail with empty provider ID")

		testutil.PrintSuccess(t, "验证通过：空的 ProviderID 被正确拒绝")
		t.Logf("  %s %s", testutil.Colorize("错误信息:", testutil.ColorRed+testutil.ColorBold), testutil.Colorize(err.Error(), testutil.ColorRed))
	})

	t.Run("GetAvailable with empty ProviderID should fail", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: GetAvailable with empty ProviderID (应该失败)")

		req := &providerpb.GetAvailableRequest{
			ProviderId: "",
		}

		testutil.PrintInfo(t, "使用空的 ProviderID 尝试获取可用资源...")
		_, err := svc.GetAvailable(ctx, req)
		assert.Error(t, err, "Should fail with empty provider ID")

		testutil.PrintSuccess(t, "验证通过：空的 ProviderID 被正确拒绝")
		t.Logf("  %s %s", testutil.Colorize("错误信息:", testutil.ColorRed+testutil.ColorBold), testutil.Colorize(err.Error(), testutil.ColorRed))
	})

	t.Log("\n" + testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold))
	t.Log(testutil.Colorize("✓ 所有 Kubernetes 连接状态下的资源态势感知测试通过", testutil.ColorGreen+testutil.ColorBold))
	t.Log(testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold) + "\n")
}

// createK8sClient 创建 Kubernetes client 用于直接操作 Pod
func createK8sClient() (*kubernetes.Clientset, error) {
	kubeconfig := testutil.GetK8sKubeconfig()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

// waitForPodRunning 等待 Pod 进入 Running 状态
func waitForPodRunning(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastPhase := ""

	for time.Now().Before(deadline) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod: %w", err)
		}

		currentPhase := string(pod.Status.Phase)
		if currentPhase != lastPhase {
			lastPhase = currentPhase
		}

		if pod.Status.Phase == corev1.PodRunning {
			return nil
		}

		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			// 收集错误信息
			var errMsg strings.Builder
			errMsg.WriteString(fmt.Sprintf("pod entered terminal state: %s", pod.Status.Phase))

			// 添加容器状态信息
			for _, status := range pod.Status.ContainerStatuses {
				if status.State.Waiting != nil {
					errMsg.WriteString(fmt.Sprintf(", container %s waiting: %s (reason: %s)",
						status.Name, status.State.Waiting.Message, status.State.Waiting.Reason))
				}
				if status.State.Terminated != nil {
					errMsg.WriteString(fmt.Sprintf(", container %s terminated: %s (reason: %s, exit code: %d)",
						status.Name, status.State.Terminated.Message, status.State.Terminated.Reason, status.State.Terminated.ExitCode))
				}
			}

			// 添加 Pod 事件信息
			events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
			})
			if err == nil && len(events.Items) > 0 {
				// 获取最近的事件
				recentEvents := events.Items
				if len(recentEvents) > 3 {
					recentEvents = recentEvents[len(recentEvents)-3:]
				}
				for _, event := range recentEvents {
					errMsg.WriteString(fmt.Sprintf(", event: %s - %s", event.Reason, event.Message))
				}
			}

			return fmt.Errorf("%s", errMsg.String())
		}

		// 如果 Pod 处于 Pending 状态，检查调度问题
		if pod.Status.Phase == corev1.PodPending {
			// 检查是否有调度问题
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
					// Pod 无法调度，收集详细信息
					var errMsg strings.Builder
					errMsg.WriteString(fmt.Sprintf("pod is pending and cannot be scheduled: %s", condition.Message))

					// 检查容器状态
					for _, status := range pod.Status.ContainerStatuses {
						if status.State.Waiting != nil {
							errMsg.WriteString(fmt.Sprintf(", container %s waiting: %s (reason: %s)",
								status.Name, status.State.Waiting.Message, status.State.Waiting.Reason))
						}
					}

					// 获取调度相关事件
					events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
						FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
					})
					if err == nil {
						for _, event := range events.Items {
							if event.Type == "Warning" && (strings.Contains(event.Reason, "Failed") ||
								strings.Contains(event.Reason, "Scheduling") ||
								strings.Contains(event.Reason, "FailedScheduling")) {
								errMsg.WriteString(fmt.Sprintf(", event: %s - %s", event.Reason, event.Message))
							}
						}
					}

					// 如果接近超时，返回详细信息
					if time.Until(deadline) < 5*time.Second {
						return fmt.Errorf("%s", errMsg.String())
					}
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	// 超时，返回详细信息
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("timeout waiting for pod to be running (last phase: %s), failed to get pod status: %w", lastPhase, err)
	}

	var errMsg strings.Builder
	errMsg.WriteString(fmt.Sprintf("timeout waiting for pod to be running (current phase: %s)", pod.Status.Phase))

	// 添加容器状态
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			errMsg.WriteString(fmt.Sprintf(", container %s waiting: %s (reason: %s)",
				status.Name, status.State.Waiting.Message, status.State.Waiting.Reason))
		}
	}

	// 添加 Pod 条件
	for _, condition := range pod.Status.Conditions {
		if condition.Status == corev1.ConditionFalse {
			errMsg.WriteString(fmt.Sprintf(", condition %s: %s", condition.Type, condition.Message))
		}
	}

	// 获取最近的事件
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
	})
	if err == nil && len(events.Items) > 0 {
		recentEvents := events.Items
		if len(recentEvents) > 5 {
			recentEvents = recentEvents[len(recentEvents)-5:]
		}
		for _, event := range recentEvents {
			if event.Type == "Warning" {
				errMsg.WriteString(fmt.Sprintf(", event: %s - %s", event.Reason, event.Message))
			}
		}
	}

	return fmt.Errorf("%s", errMsg.String())
}

// waitForPodDeleted 等待 Pod 被删除
func waitForPodDeleted(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		_, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// Pod 不存在了，删除完成
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for pod to be deleted")
}
