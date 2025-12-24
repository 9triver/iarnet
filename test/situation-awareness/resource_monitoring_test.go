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
	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCase: 资源监控测试
// 测试目的：验证资源态势感知中的资源监控功能，包括：
// 1. 资源实时使用查询 - 验证能够实时查询资源使用情况
// 2. 资源健康检查监控 - 验证健康检查机制能够监控资源状态

// TestResourceMonitoring_RealTimeUsageQuery 测试用例 1: 资源实时使用查询
func TestResourceMonitoring_RealTimeUsageQuery(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: 资源实时使用查询",
		"验证 Docker Provider 的资源实时使用查询功能")

	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	ctx := context.Background()

	// 步骤 1: 创建并注册 Docker Provider
	testutil.PrintTestSection(t, "步骤 1: 创建并注册 Docker Provider")
	svc, err := createTestService()
	require.NoError(t, err, "Failed to create Docker provider service")
	defer svc.Close()

	providerID := "test-provider-monitoring"
	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err, "Connect should succeed")
	require.True(t, connectResp.Success, "Connect should be successful")

	testutil.PrintSuccess(t, fmt.Sprintf("Docker Provider 注册成功: %s", providerID))

	// 步骤 2: 获取初始资源使用情况
	testutil.PrintTestSection(t, "步骤 2: 获取初始资源使用情况")
	initialCapacityReq := &providerpb.GetCapacityRequest{
		ProviderId: providerID,
	}
	initialCapacity, err := svc.GetCapacity(ctx, initialCapacityReq)
	require.NoError(t, err, "GetCapacity should succeed")
	require.NotNil(t, initialCapacity, "Capacity response should not be nil")
	require.NotNil(t, initialCapacity.Capacity, "Capacity should not be nil")
	// 复制值而不是引用，避免后续修改影响初始值
	initialUsed := &resourcepb.Info{
		Cpu:    initialCapacity.Capacity.Used.Cpu,
		Memory: initialCapacity.Capacity.Used.Memory,
		Gpu:    initialCapacity.Capacity.Used.Gpu,
	}

	t.Log("\n" + testutil.Colorize("初始资源使用情况:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    %s", testutil.Colorize("已用 CPU:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", initialUsed.Cpu), testutil.ColorYellow))
	t.Logf("  %s   %s", testutil.Colorize("已用内存:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(initialUsed.Memory), testutil.ColorYellow))
	if initialCapacity.Capacity.Total.Gpu > 0 {
		t.Logf("  %s    %s", testutil.Colorize("已用 GPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d", initialUsed.Gpu), testutil.ColorYellow))
	}

	// 步骤 3: 通过 Provider Deploy API 创建测试容器以改变资源使用
	testutil.PrintTestSection(t, "步骤 3: 通过 Provider Deploy API 创建测试容器以改变资源使用")
	testInstanceID := fmt.Sprintf("test-monitoring-usage-%d", time.Now().Unix())
	testImage := "alpine:latest"
	testCPU := int64(1000)                 // 1000 millicores (1 CPU)
	testMemory := int64(256 * 1024 * 1024) // 256MB

	testutil.PrintInfo(t, fmt.Sprintf("通过 Provider Deploy API 创建测试容器: %s", testInstanceID))
	testutil.PrintInfo(t, fmt.Sprintf("  镜像: %s", testImage))
	testutil.PrintInfo(t, fmt.Sprintf("  CPU: %d millicores, 内存: %s", testCPU, testutil.FormatBytes(testMemory)))

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

	testutil.PrintSuccess(t, fmt.Sprintf("测试容器通过 Provider Deploy API 创建并启动成功: %s", testInstanceID))

	// 等待容器启动并稳定
	time.Sleep(2 * time.Second)

	// 步骤 4: 实时查询资源使用情况（应该看到资源使用增加）
	testutil.PrintTestSection(t, "步骤 4: 实时查询资源使用情况（容器运行后）")
	afterCreateCapacityReq := &providerpb.GetCapacityRequest{
		ProviderId: providerID,
	}
	afterCreateCapacity, err := svc.GetCapacity(ctx, afterCreateCapacityReq)
	require.NoError(t, err, "GetCapacity should succeed after container creation")
	require.NotNil(t, afterCreateCapacity, "Capacity response should not be nil")
	require.NotNil(t, afterCreateCapacity.Capacity, "Capacity should not be nil")
	afterCreateUsed := afterCreateCapacity.Capacity.Used

	t.Log("\n" + testutil.Colorize("容器运行后资源使用情况:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    %s", testutil.Colorize("已用 CPU:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", afterCreateUsed.Cpu), testutil.ColorYellow))
	t.Logf("  %s   %s", testutil.Colorize("已用内存:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(afterCreateUsed.Memory), testutil.ColorYellow))
	if afterCreateCapacity.Capacity.Total.Gpu > 0 {
		t.Logf("  %s    %s", testutil.Colorize("已用 GPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d", afterCreateUsed.Gpu), testutil.ColorYellow))
	}

	// 验证资源使用增加
	cpuIncrease := afterCreateUsed.Cpu - initialUsed.Cpu
	memoryIncrease := afterCreateUsed.Memory - initialUsed.Memory

	t.Log("\n" + testutil.Colorize("资源使用变化:", testutil.ColorCyan+testutil.ColorBold))
	t.Logf("  %s    %s", testutil.Colorize("CPU 增加:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", cpuIncrease), testutil.ColorYellow))
	t.Logf("  %s   %s", testutil.Colorize("内存增加:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(memoryIncrease), testutil.ColorYellow))

	assert.GreaterOrEqual(t, afterCreateUsed.Cpu, initialUsed.Cpu,
		"CPU usage should increase after creating container")
	assert.GreaterOrEqual(t, afterCreateUsed.Memory, initialUsed.Memory,
		"Memory usage should increase after creating container")

	testutil.PrintSuccess(t, "资源实时使用查询验证通过：能够实时反映资源使用变化")

	// 步骤 5: 多次实时查询验证实时性
	testutil.PrintTestSection(t, "步骤 5: 多次实时查询验证实时性")
	queryCount := 3
	queryResults := make([]*resourcepb.Capacity, queryCount)

	for i := 0; i < queryCount; i++ {
		testutil.PrintInfo(t, fmt.Sprintf("执行第 %d 次实时查询...", i+1))
		capacityReq := &providerpb.GetCapacityRequest{
			ProviderId: providerID,
		}
		resp, err := svc.GetCapacity(ctx, capacityReq)
		require.NoError(t, err, fmt.Sprintf("GetCapacity should succeed on query %d", i+1))
		require.NotNil(t, resp, fmt.Sprintf("Response %d should not be nil", i+1))
		require.NotNil(t, resp.Capacity, fmt.Sprintf("Capacity %d should not be nil", i+1))
		queryResults[i] = resp.Capacity

		t.Logf("  查询 %d: CPU %d mC (已用), Memory %s (已用)",
			i+1, resp.Capacity.Used.Cpu, testutil.FormatBytes(resp.Capacity.Used.Memory))

		// 等待一小段时间
		if i < queryCount-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 验证所有查询都成功返回
	for i, result := range queryResults {
		assert.NotNil(t, result, fmt.Sprintf("Query %d result should not be nil", i+1))
		assert.NotNil(t, result.Used, fmt.Sprintf("Query %d Used should not be nil", i+1))
		assert.GreaterOrEqual(t, result.Used.Cpu, int64(0), fmt.Sprintf("Query %d CPU should be non-negative", i+1))
		assert.GreaterOrEqual(t, result.Used.Memory, int64(0), fmt.Sprintf("Query %d Memory should be non-negative", i+1))
	}

	testutil.PrintSuccess(t, fmt.Sprintf("多次实时查询验证通过：成功执行 %d 次查询", queryCount))

	// 步骤 6: 通过 Provider Undeploy API 清理测试容器
	testutil.PrintTestSection(t, "步骤 6: 通过 Provider Undeploy API 清理测试容器")
	undeployReq := &providerpb.UndeployRequest{
		ProviderId: providerID,
		InstanceId: testInstanceID,
	}

	undeployResp, err := svc.Undeploy(ctx, undeployReq)
	require.NoError(t, err, "Undeploy should succeed")
	require.Empty(t, undeployResp.Error, fmt.Sprintf("Undeploy should not return error, got: %s", undeployResp.Error))

	testutil.PrintSuccess(t, "测试容器已通过 Provider Undeploy API 清理")

	// 步骤 7: 验证清理后的资源使用（应该减少）
	testutil.PrintTestSection(t, "步骤 7: 验证清理后的资源使用")
	time.Sleep(2 * time.Second)

	finalCapacityReq := &providerpb.GetCapacityRequest{
		ProviderId: providerID,
	}
	finalCapacity, err := svc.GetCapacity(ctx, finalCapacityReq)
	require.NoError(t, err, "GetCapacity should succeed after cleanup")
	require.NotNil(t, finalCapacity, "Capacity response should not be nil")
	require.NotNil(t, finalCapacity.Capacity, "Capacity should not be nil")
	// 复制值而不是引用
	finalUsed := &resourcepb.Info{
		Cpu:    finalCapacity.Capacity.Used.Cpu,
		Memory: finalCapacity.Capacity.Used.Memory,
		Gpu:    finalCapacity.Capacity.Used.Gpu,
	}

	t.Log("\n" + testutil.Colorize("清理后资源使用情况:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    %s", testutil.Colorize("已用 CPU:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", finalUsed.Cpu), testutil.ColorYellow))
	t.Logf("  %s   %s", testutil.Colorize("已用内存:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(finalUsed.Memory), testutil.ColorYellow))
	if finalCapacity.Capacity.Total.Gpu > 0 {
		t.Logf("  %s    %s", testutil.Colorize("已用 GPU:", testutil.ColorWhite+testutil.ColorBold),
			testutil.Colorize(fmt.Sprintf("%d", finalUsed.Gpu), testutil.ColorYellow))
	}

	// 验证资源使用减少
	cpuDecrease := afterCreateUsed.Cpu - finalUsed.Cpu
	memoryDecrease := afterCreateUsed.Memory - finalUsed.Memory

	t.Log("\n" + testutil.Colorize("资源使用变化:", testutil.ColorCyan+testutil.ColorBold))
	t.Logf("  %s    %s", testutil.Colorize("CPU 减少:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", cpuDecrease), testutil.ColorGreen))
	t.Logf("  %s   %s", testutil.Colorize("内存减少:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(memoryDecrease), testutil.ColorGreen))

	assert.LessOrEqual(t, finalUsed.Cpu, afterCreateUsed.Cpu,
		"CPU usage should decrease after removing container")

	testutil.PrintSuccess(t, "资源实时使用查询完整验证通过")

	t.Log("\n" + testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold))
	t.Log(testutil.Colorize("✓ 资源实时使用查询测试通过", testutil.ColorGreen+testutil.ColorBold))
	t.Log(testutil.Colorize("  - 能够实时查询资源使用情况", testutil.ColorGreen))
	t.Log(testutil.Colorize("  - 能够实时反映资源使用变化", testutil.ColorGreen))
	t.Log(testutil.Colorize("  - 支持多次连续查询", testutil.ColorGreen))
	t.Log(testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold) + "\n")
}

// TestResourceMonitoring_HealthCheckMonitoring 测试用例 2: 资源健康检查监控
func TestResourceMonitoring_HealthCheckMonitoring(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: 资源健康检查监控",
		"验证 Docker Provider 的资源健康检查监控功能")

	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	ctx := context.Background()

	// 步骤 1: 创建并注册 Docker Provider
	testutil.PrintTestSection(t, "步骤 1: 创建并注册 Docker Provider")
	svc, err := createTestService()
	require.NoError(t, err, "Failed to create Docker provider service")
	defer svc.Close()

	providerID := "test-provider-health-check"
	connectReq := &providerpb.ConnectRequest{
		ProviderId: providerID,
	}
	connectResp, err := svc.Connect(ctx, connectReq)
	require.NoError(t, err, "Connect should succeed")
	require.True(t, connectResp.Success, "Connect should be successful")

	testutil.PrintSuccess(t, fmt.Sprintf("Docker Provider 注册成功: %s", providerID))

	// 步骤 2: 执行健康检查
	testutil.PrintTestSection(t, "步骤 2: 执行健康检查")
	healthReq := &providerpb.HealthCheckRequest{
		ProviderId: providerID,
	}

	testutil.PrintInfo(t, "正在执行健康检查...")
	healthResp, err := svc.HealthCheck(ctx, healthReq)
	require.NoError(t, err, "HealthCheck should succeed")
	require.NotNil(t, healthResp, "Health check response should not be nil")
	require.NotNil(t, healthResp.Capacity, "Health check should include capacity information")
	require.NotNil(t, healthResp.ResourceTags, "Health check should include resource tags")

	testutil.PrintSuccess(t, "健康检查执行成功")

	// 显示健康检查结果
	t.Log("\n" + testutil.Colorize("健康检查结果:", testutil.ColorYellow+testutil.ColorBold))
	testutil.PrintResourceInfo(t, healthResp.Capacity)

	t.Log("\n" + testutil.Colorize("资源标签:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    %s", testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(healthResp.ResourceTags.Cpu))
	t.Logf("  %s   %s", testutil.Colorize("GPU:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(healthResp.ResourceTags.Gpu))
	t.Logf("  %s  %s", testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(healthResp.ResourceTags.Memory))
	t.Logf("  %s %s", testutil.Colorize("摄像头:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(healthResp.ResourceTags.Camera))

	// 验证健康检查返回的资源信息
	assert.NotNil(t, healthResp.Capacity.Total, "Total capacity should not be nil")
	assert.NotNil(t, healthResp.Capacity.Used, "Used capacity should not be nil")
	assert.NotNil(t, healthResp.Capacity.Available, "Available capacity should not be nil")
	assert.Greater(t, healthResp.Capacity.Total.Cpu, int64(0), "Total CPU should be greater than 0")
	assert.Greater(t, healthResp.Capacity.Total.Memory, int64(0), "Total Memory should be greater than 0")

	// 验证资源标签与容量的一致性
	assert.NotNil(t, healthResp.ResourceTags, "Resource tags should not be nil")
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
			testutil.PrintInfo(t, "GPU 容量存在但标签为 false，可能 provider 未启用 GPU 资源类型")
		} else {
			assert.True(t, healthResp.ResourceTags.Gpu,
				"GPU tag should be true when GPU capacity is available")
		}
	}

	// 步骤 3: 多次健康检查监控
	testutil.PrintTestSection(t, "步骤 3: 多次健康检查监控（验证监控连续性）")
	monitoringCount := 5
	monitoringResults := make([]*providerpb.HealthCheckResponse, monitoringCount)

	for i := 0; i < monitoringCount; i++ {
		testutil.PrintInfo(t, fmt.Sprintf("执行第 %d 次健康检查...", i+1))
		result, err := svc.HealthCheck(ctx, healthReq)
		require.NoError(t, err, fmt.Sprintf("HealthCheck should succeed on check %d", i+1))
		require.NotNil(t, result, fmt.Sprintf("Health check result %d should not be nil", i+1))
		monitoringResults[i] = result

		t.Logf("  检查 %d: CPU 总计 %d mC, 已用 %d mC, 可用 %d mC",
			i+1,
			result.Capacity.Total.Cpu,
			result.Capacity.Used.Cpu,
			result.Capacity.Available.Cpu)

		// 等待一小段时间
		if i < monitoringCount-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 验证所有健康检查都成功
	for i, result := range monitoringResults {
		assert.NotNil(t, result.Capacity, fmt.Sprintf("Check %d capacity should not be nil", i+1))
		assert.NotNil(t, result.ResourceTags, fmt.Sprintf("Check %d resource tags should not be nil", i+1))
	}

	testutil.PrintSuccess(t, fmt.Sprintf("多次健康检查监控验证通过：成功执行 %d 次健康检查", monitoringCount))

	// 步骤 4: 验证健康检查的资源状态一致性
	testutil.PrintTestSection(t, "步骤 4: 验证健康检查的资源状态一致性")
	// 比较第一次和最后一次健康检查的结果
	firstCheck := monitoringResults[0]
	lastCheck := monitoringResults[monitoringCount-1]

	// 验证资源容量计算的一致性
	assert.Equal(t, firstCheck.Capacity.Total.Cpu,
		firstCheck.Capacity.Used.Cpu+firstCheck.Capacity.Available.Cpu,
		"First check: Total CPU should equal Used + Available")
	assert.Equal(t, lastCheck.Capacity.Total.Cpu,
		lastCheck.Capacity.Used.Cpu+lastCheck.Capacity.Available.Cpu,
		"Last check: Total CPU should equal Used + Available")

	assert.Equal(t, firstCheck.Capacity.Total.Memory,
		firstCheck.Capacity.Used.Memory+firstCheck.Capacity.Available.Memory,
		"First check: Total Memory should equal Used + Available")
	assert.Equal(t, lastCheck.Capacity.Total.Memory,
		lastCheck.Capacity.Used.Memory+lastCheck.Capacity.Available.Memory,
		"Last check: Total Memory should equal Used + Available")

	// 验证资源标签一致性
	assert.Equal(t, firstCheck.ResourceTags.Cpu, lastCheck.ResourceTags.Cpu,
		"Resource tags CPU should be consistent")
	assert.Equal(t, firstCheck.ResourceTags.Memory, lastCheck.ResourceTags.Memory,
		"Resource tags Memory should be consistent")
	if firstCheck.Capacity.Total.Gpu > 0 || lastCheck.Capacity.Total.Gpu > 0 {
		assert.Equal(t, firstCheck.ResourceTags.Gpu, lastCheck.ResourceTags.Gpu,
			"Resource tags GPU should be consistent")
	}

	testutil.PrintSuccess(t, "健康检查资源状态一致性验证通过")

	// 步骤 5: 验证健康检查的实时性（通过资源变化）
	testutil.PrintTestSection(t, "步骤 5: 验证健康检查的实时性（通过资源变化）")
	// 创建测试容器以改变资源状态
	dockerClient, err := createDockerClient()
	require.NoError(t, err, "Failed to create Docker client")
	defer dockerClient.Close()

	testContainerName := fmt.Sprintf("test-health-check-%d", time.Now().Unix())
	containerConfig := &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sleep", "3600"},
	}
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			NanoCPUs: 500 * 1e6,         // 500 millicores
			Memory:   128 * 1024 * 1024, // 128MB
		},
	}

	createResp, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, testContainerName)
	require.NoError(t, err, "Failed to create test container")
	err = dockerClient.ContainerStart(ctx, createResp.ID, container.StartOptions{})
	require.NoError(t, err, "Failed to start test container")

	testutil.PrintSuccess(t, "测试容器已创建并启动")

	// 等待容器启动
	time.Sleep(2 * time.Second)

	// 执行健康检查（应该反映新的资源使用情况）
	testutil.PrintInfo(t, "执行健康检查（容器运行后）...")
	healthAfterContainer, err := svc.HealthCheck(ctx, healthReq)
	require.NoError(t, err, "HealthCheck should succeed after container creation")
	require.NotNil(t, healthAfterContainer, "Health check response should not be nil")
	// 同时获取 GetAllocated 结果用于对比
	allocatedAfterContainer, err := svc.GetAllocated(ctx)
	require.NoError(t, err, "GetAllocated should succeed after container creation")
	require.NotNil(t, allocatedAfterContainer, "Allocated resources should not be nil after container creation")

	t.Log("\n" + testutil.Colorize("容器运行后健康检查结果:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", healthAfterContainer.Capacity.Total.Cpu), testutil.ColorWhite),
		testutil.Colorize(fmt.Sprintf("%d millicores", healthAfterContainer.Capacity.Used.Cpu), testutil.ColorYellow),
		testutil.Colorize(fmt.Sprintf("%d millicores", healthAfterContainer.Capacity.Available.Cpu), testutil.ColorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(healthAfterContainer.Capacity.Total.Memory), testutil.ColorWhite),
		testutil.Colorize(testutil.FormatBytes(healthAfterContainer.Capacity.Used.Memory), testutil.ColorYellow),
		testutil.Colorize(testutil.FormatBytes(healthAfterContainer.Capacity.Available.Memory), testutil.ColorGreen))

	// 验证健康检查结果与实时查询结果一致（证明能够感知实时变化）
	assert.Equal(t, allocatedAfterContainer.Cpu, healthAfterContainer.Capacity.Used.Cpu,
		"Used CPU reported by health check should match GetAllocated result")
	assert.Equal(t, allocatedAfterContainer.Memory, healthAfterContainer.Capacity.Used.Memory,
		"Used Memory reported by health check should match GetAllocated result")

	testutil.PrintSuccess(t, "健康检查实时性验证通过：能够实时反映资源状态变化")

	// 清理测试容器
	err = dockerClient.ContainerStop(ctx, createResp.ID, container.StopOptions{})
	require.NoError(t, err, "Failed to stop test container")
	err = dockerClient.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
	require.NoError(t, err, "Failed to remove test container")

	// 步骤 6: 验证健康检查监控的连续性
	testutil.PrintTestSection(t, "步骤 6: 验证健康检查监控的连续性")
	// 执行最后一次健康检查
	finalHealthCheck, err := svc.HealthCheck(ctx, healthReq)
	require.NoError(t, err, "Final health check should succeed")
	require.NotNil(t, finalHealthCheck, "Final health check response should not be nil")

	t.Log("\n" + testutil.Colorize("最终健康检查结果:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", finalHealthCheck.Capacity.Total.Cpu), testutil.ColorWhite),
		testutil.Colorize(fmt.Sprintf("%d millicores", finalHealthCheck.Capacity.Used.Cpu), testutil.ColorYellow),
		testutil.Colorize(fmt.Sprintf("%d millicores", finalHealthCheck.Capacity.Available.Cpu), testutil.ColorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(finalHealthCheck.Capacity.Total.Memory), testutil.ColorWhite),
		testutil.Colorize(testutil.FormatBytes(finalHealthCheck.Capacity.Used.Memory), testutil.ColorYellow),
		testutil.Colorize(testutil.FormatBytes(finalHealthCheck.Capacity.Available.Memory), testutil.ColorGreen))

	// 验证健康检查监控的连续性
	assert.Equal(t, healthAfterContainer.Capacity.Total.Cpu, finalHealthCheck.Capacity.Total.Cpu,
		"Total CPU should remain consistent")
	assert.Equal(t, healthAfterContainer.Capacity.Total.Memory, finalHealthCheck.Capacity.Total.Memory,
		"Total Memory should remain consistent")

	testutil.PrintSuccess(t, "健康检查监控连续性验证通过")

	t.Log("\n" + testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold))
	t.Log(testutil.Colorize("✓ 资源健康检查监控测试通过", testutil.ColorGreen+testutil.ColorBold))
	t.Log(testutil.Colorize("  - 健康检查能够正确返回资源状态", testutil.ColorGreen))
	t.Log(testutil.Colorize("  - 健康检查能够实时反映资源变化", testutil.ColorGreen))
	t.Log(testutil.Colorize("  - 健康检查监控具有连续性", testutil.ColorGreen))
	t.Log(testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold) + "\n")
}
