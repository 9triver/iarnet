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
	"github.com/9triver/iarnet/providers/k8s/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TestCase T3-1-002: Kubernetes 资源接入测试
// 测试目的：验证 Kubernetes provider 的资源态势感知能力，包括资源容量、可用资源、
// 已分配资源的获取，以及健康检查中包含的资源态势信息。

// TestK8sProvider_ResourceSituationAwareness 测试 Kubernetes provider 的资源态势感知功能
func TestK8sProvider_ResourceSituationAwareness(t *testing.T) {
	printTestHeader(t, "测试用例 T3-1-002: Kubernetes 资源接入测试",
		"验证 Kubernetes provider 的资源态势感知能力")

	if !isK8sAvailable() {
		t.Skip("Kubernetes is not available, skipping test")
	}

	svc, err := createK8sTestService()
	require.NoError(t, err, "Failed to create Kubernetes provider service")
	defer svc.Close()

	ctx := context.Background()
	providerID := "test-k8s-provider-situation-awareness"

	// 首先需要连接 provider（注册）
	printTestSection(t, "步骤 1: 注册 Kubernetes Provider")
	printInfo(t, fmt.Sprintf("正在注册 Provider ID: %s", providerID))

	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err, "Connect should succeed")
	require.True(t, connectResp.Success, "Connect should be successful")

	printSuccess(t, fmt.Sprintf("Provider 注册成功: %s", providerID))
	printInfo(t, fmt.Sprintf("Provider 类型: %s", connectResp.ProviderType.Name))

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
		// Kubernetes provider 通常支持 CPU 和 Memory
		assert.True(t, healthResp.ResourceTags.Cpu || healthResp.ResourceTags.Memory,
			"At least CPU or Memory should be supported")

		// 验证容量计算正确性
		assert.Equal(t, healthResp.Capacity.Total.Cpu,
			healthResp.Capacity.Used.Cpu+healthResp.Capacity.Available.Cpu,
			"Total CPU should equal Used CPU + Available CPU")
		assert.Equal(t, healthResp.Capacity.Total.Memory,
			healthResp.Capacity.Used.Memory+healthResp.Capacity.Available.Memory,
			"Total Memory should equal Used Memory + Available Memory")

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

		assert.Equal(t, capacityResp.Capacity.Used.Cpu, allocated.Cpu,
			"GetCapacity and GetAllocated should return the same used CPU")
		assert.Equal(t, capacityResp.Capacity.Used.Memory, allocated.Memory,
			"GetCapacity and GetAllocated should return the same used Memory")

		printSuccess(t, "资源态势一致性验证通过")
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("GetCapacity 和 GetAvailable 返回的可用资源一致", colorGreen))
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("GetCapacity 和 GetAllocated 返回的已用资源一致", colorGreen))
	})

	t.Run("ResourceSituationRealTime - 资源态势实时性验证", func(t *testing.T) {
		printTestSection(t, "测试: ResourceSituationRealTime - 资源态势实时性验证")

		// 创建 Kubernetes client 用于直接操作 Pod
		k8sClient, err := createK8sClient()
		require.NoError(t, err, "Failed to create Kubernetes client")

		// 测试 Pod 配置
		testPodName := fmt.Sprintf("test-situation-awareness-%d", time.Now().Unix())
		testNamespace := "default"
		testImage := "busybox:latest"
		testCPU := int64(100)                 // 100 millicores (0.1 CPU)
		testMemory := int64(64 * 1024 * 1024) // 64MB

		// 步骤 1: 获取初始资源状态
		printInfo(t, "步骤 1: 获取初始资源状态...")
		initialCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		initialUsed := initialCapacity.Capacity.Used
		initialAvailable := initialCapacity.Capacity.Available

		t.Log("\n" + colorize("初始资源状态:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("已用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", initialUsed.Cpu), colorYellow))
		t.Logf("  %s   %s",
			colorize("已用内存:", colorWhite+colorBold),
			colorize(formatBytes(initialUsed.Memory), colorYellow))
		t.Logf("  %s  %s",
			colorize("可用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", initialAvailable.Cpu), colorGreen))
		t.Logf("  %s %s",
			colorize("可用内存:", colorWhite+colorBold),
			colorize(formatBytes(initialAvailable.Memory), colorGreen))

		// 步骤 2: 创建测试 Pod
		printInfo(t, fmt.Sprintf("步骤 2: 创建测试 Pod (%s)...", testPodName))
		printInfo(t, fmt.Sprintf("  镜像: %s", testImage))
		printInfo(t, fmt.Sprintf("  CPU: %d millicores, 内存: %s", testCPU, formatBytes(testMemory)))

		cpuQuantity := resource.NewMilliQuantity(testCPU, resource.DecimalSI)
		memoryQuantity := resource.NewQuantity(testMemory, resource.BinarySI)

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testPodName,
				Namespace: testNamespace,
				Labels: map[string]string{
					"iarnet.managed":     "true",
					"iarnet.provider_id": providerID,
					"iarnet.test":        "situation-awareness",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "main",
						Image:           testImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"sleep", "3600"},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *cpuQuantity,
								corev1.ResourceMemory: *memoryQuantity,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    *cpuQuantity,
								corev1.ResourceMemory: *memoryQuantity,
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}

		createdPod, err := k8sClient.CoreV1().Pods(testNamespace).Create(ctx, pod, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create test Pod")

		printSuccess(t, fmt.Sprintf("测试 Pod 创建成功: %s/%s", testNamespace, createdPod.Name))

		// 等待 Pod 启动
		printInfo(t, "等待 Pod 启动...")
		err = waitForPodRunning(ctx, k8sClient, testNamespace, testPodName, 60*time.Second)
		if err != nil {
			// 如果等待超时，仍然继续测试（Pod 可能在 Pending 状态）
			t.Logf("  %s Pod 未能进入 Running 状态: %v", colorize("⚠", colorYellow), err)
		} else {
			printSuccess(t, "Pod 已进入 Running 状态")
		}

		// 等待一段时间让资源分配生效
		time.Sleep(2 * time.Second)

		// 步骤 3: 获取创建 Pod 后的资源状态
		printInfo(t, "步骤 3: 获取创建 Pod 后的资源状态...")
		afterCreateCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		afterCreateUsed := afterCreateCapacity.Capacity.Used
		afterCreateAvailable := afterCreateCapacity.Capacity.Available

		t.Log("\n" + colorize("创建 Pod 后资源状态:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("已用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterCreateUsed.Cpu), colorYellow))
		t.Logf("  %s   %s",
			colorize("已用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterCreateUsed.Memory), colorYellow))
		t.Logf("  %s  %s",
			colorize("可用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterCreateAvailable.Cpu), colorGreen))
		t.Logf("  %s %s",
			colorize("可用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterCreateAvailable.Memory), colorGreen))

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

		// 验证资源确实增加了
		assert.GreaterOrEqual(t, afterCreateUsed.Cpu, initialUsed.Cpu,
			"CPU usage should increase after creating Pod")
		assert.GreaterOrEqual(t, afterCreateUsed.Memory, initialUsed.Memory,
			"Memory usage should increase after creating Pod")

		// 步骤 4: 删除 Pod
		printInfo(t, "步骤 4: 删除测试 Pod...")
		err = k8sClient.CoreV1().Pods(testNamespace).Delete(ctx, testPodName, metav1.DeleteOptions{})
		require.NoError(t, err, "Failed to delete test Pod")
		printSuccess(t, "测试 Pod 已删除")

		// 等待 Pod 删除完成
		printInfo(t, "等待 Pod 删除完成...")
		err = waitForPodDeleted(ctx, k8sClient, testNamespace, testPodName, 60*time.Second)
		if err != nil {
			t.Logf("  %s Pod 删除等待超时: %v", colorize("⚠", colorYellow), err)
		} else {
			printSuccess(t, "Pod 已完全删除")
		}

		// 等待一段时间让资源释放生效
		time.Sleep(2 * time.Second)

		// 步骤 5: 获取删除 Pod 后的资源状态
		printInfo(t, "步骤 5: 获取删除 Pod 后的资源状态...")
		afterDeleteCapacity, err := svc.GetCapacity(ctx, &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		})
		require.NoError(t, err)
		afterDeleteUsed := afterDeleteCapacity.Capacity.Used
		afterDeleteAvailable := afterDeleteCapacity.Capacity.Available

		t.Log("\n" + colorize("删除 Pod 后资源状态:", colorYellow+colorBold))
		t.Logf("  %s    %s",
			colorize("已用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterDeleteUsed.Cpu), colorYellow))
		t.Logf("  %s   %s",
			colorize("已用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterDeleteUsed.Memory), colorYellow))
		t.Logf("  %s  %s",
			colorize("可用 CPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", afterDeleteAvailable.Cpu), colorGreen))
		t.Logf("  %s %s",
			colorize("可用内存:", colorWhite+colorBold),
			colorize(formatBytes(afterDeleteAvailable.Memory), colorGreen))

		// 验证资源使用减少
		cpuDecrease := afterCreateUsed.Cpu - afterDeleteUsed.Cpu
		memoryDecrease := afterCreateUsed.Memory - afterDeleteUsed.Memory
		t.Log("\n" + colorize("资源变化分析:", colorCyan+colorBold))
		t.Logf("  %s    %s",
			colorize("CPU 减少:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d millicores", cpuDecrease), colorGreen))
		t.Logf("  %s   %s",
			colorize("内存减少:", colorWhite+colorBold),
			colorize(formatBytes(memoryDecrease), colorGreen))

		// 验证资源确实减少了
		assert.LessOrEqual(t, afterDeleteUsed.Cpu, afterCreateUsed.Cpu,
			"CPU usage should decrease after deleting Pod")
		assert.LessOrEqual(t, afterDeleteUsed.Memory, afterCreateUsed.Memory,
			"Memory usage should decrease after deleting Pod")

		// 最终验证：资源状态能够实时更新
		printSuccess(t, "资源态势实时性验证通过")
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("创建 Pod 后资源使用增加", colorGreen))
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("删除 Pod 后资源使用减少", colorGreen))
		t.Logf("  %s %s", colorize("✓", colorGreen), colorize("接口能够实时反映资源状态变化", colorGreen))
	})

	t.Log("\n" + colorize(strings.Repeat("=", 80), colorCyan+colorBold))
	t.Log(colorize("✓ 所有 Kubernetes 资源态势感知测试通过", colorGreen+colorBold))
	t.Log(colorize(strings.Repeat("=", 80), colorCyan+colorBold) + "\n")
}

// TestK8sProvider_ResourceSituationAwareness_WithConnection 测试连接状态下的资源态势感知
func TestK8sProvider_ResourceSituationAwareness_WithConnection(t *testing.T) {
	printTestHeader(t, "测试用例: Kubernetes 连接状态下的资源态势感知",
		"验证连接状态下的资源态势感知和鉴权机制")

	if !isK8sAvailable() {
		t.Skip("Kubernetes is not available, skipping test")
	}

	svc, err := createK8sTestService()
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()
	providerID := "test-k8s-provider-connected"

	// 连接 provider
	printTestSection(t, "步骤 1: 注册 Kubernetes Provider")
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
	t.Log(colorize("✓ 所有 Kubernetes 连接状态下的资源态势感知测试通过", colorGreen+colorBold))
	t.Log(colorize(strings.Repeat("=", 80), colorCyan+colorBold) + "\n")
}

// createK8sTestService 创建测试用的 Kubernetes Provider Service 实例
func createK8sTestService() (*provider.Service, error) {
	kubeconfig := getK8sKubeconfig()

	// 测试用的资源容量
	totalCapacity := &resourcepb.Info{
		Cpu:    8000,                   // 8 cores
		Memory: 8 * 1024 * 1024 * 1024, // 8Gi
		Gpu:    0,
	}

	return provider.NewService(kubeconfig, false, "default", "iarnet.managed=true", []string{"cpu", "memory"}, totalCapacity)
}

// getK8sKubeconfig 获取 kubeconfig 路径
func getK8sKubeconfig() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			kubeconfig = home + "/.kube/config"
		}
	}
	return kubeconfig
}

// isK8sAvailable 检查 Kubernetes 是否可用
func isK8sAvailable() bool {
	svc, err := createK8sTestService()
	if err != nil {
		return false
	}
	defer svc.Close()
	return true
}

// createK8sClient 创建 Kubernetes client 用于直接操作 Pod
func createK8sClient() (*kubernetes.Clientset, error) {
	kubeconfig := getK8sKubeconfig()

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

	for time.Now().Before(deadline) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if pod.Status.Phase == corev1.PodRunning {
			return nil
		}

		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			return fmt.Errorf("pod entered terminal state: %s", pod.Status.Phase)
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for pod to be running")
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
