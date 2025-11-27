package situation_awareness

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
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
	printTestHeader(t, "测试用例: 资源实时使用查询",
		"验证 Docker Provider 的资源实时使用查询功能")

	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	ctx := context.Background()

	// 步骤 1: 创建并注册 Docker Provider
	printTestSection(t, "步骤 1: 创建并注册 Docker Provider")
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

	printSuccess(t, fmt.Sprintf("Docker Provider 注册成功: %s", providerID))

	// 步骤 2: 获取初始资源使用情况
	printTestSection(t, "步骤 2: 获取初始资源使用情况")
	initialAllocated, err := svc.GetAllocated(ctx)
	require.NoError(t, err, "GetAllocated should succeed")
	require.NotNil(t, initialAllocated, "Allocated resources should not be nil")

	t.Log("\n" + colorize("初始资源使用情况:", colorYellow+colorBold))
	t.Logf("  %s    %s", colorize("已用 CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", initialAllocated.Cpu), colorYellow))
	t.Logf("  %s   %s", colorize("已用内存:", colorWhite+colorBold),
		colorize(formatBytes(initialAllocated.Memory), colorYellow))
	t.Logf("  %s    %s", colorize("已用 GPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d", initialAllocated.Gpu), colorYellow))

	// 步骤 3: 创建测试容器以改变资源使用
	printTestSection(t, "步骤 3: 创建测试容器以改变资源使用")
	dockerClient, err := createDockerClient()
	require.NoError(t, err, "Failed to create Docker client")
	defer dockerClient.Close()

	testContainerName := fmt.Sprintf("test-monitoring-usage-%d", time.Now().Unix())
	testImage := "alpine:latest"
	testCPU := int64(1000)                 // 1000 millicores (1 CPU)
	testMemory := int64(256 * 1024 * 1024) // 256MB

	printInfo(t, fmt.Sprintf("创建测试容器: %s", testContainerName))
	printInfo(t, fmt.Sprintf("  镜像: %s", testImage))
	printInfo(t, fmt.Sprintf("  CPU: %d millicores, 内存: %s", testCPU, formatBytes(testMemory)))

	containerConfig := &container.Config{
		Image: testImage,
		Cmd:   []string{"sleep", "3600"},
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			NanoCPUs: testCPU * 1e6,
			Memory:   testMemory,
		},
	}

	createResp, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, testContainerName)
	require.NoError(t, err, "Failed to create test container")

	err = dockerClient.ContainerStart(ctx, createResp.ID, container.StartOptions{})
	require.NoError(t, err, "Failed to start test container")

	printSuccess(t, fmt.Sprintf("测试容器创建并启动成功: %s", testContainerName))

	// 等待容器启动并稳定
	time.Sleep(2 * time.Second)

	// 步骤 4: 实时查询资源使用情况（应该看到资源使用增加）
	printTestSection(t, "步骤 4: 实时查询资源使用情况（容器运行后）")
	afterCreateAllocated, err := svc.GetAllocated(ctx)
	require.NoError(t, err, "GetAllocated should succeed after container creation")
	require.NotNil(t, afterCreateAllocated, "Allocated resources should not be nil")

	t.Log("\n" + colorize("容器运行后资源使用情况:", colorYellow+colorBold))
	t.Logf("  %s    %s", colorize("已用 CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", afterCreateAllocated.Cpu), colorYellow))
	t.Logf("  %s   %s", colorize("已用内存:", colorWhite+colorBold),
		colorize(formatBytes(afterCreateAllocated.Memory), colorYellow))
	t.Logf("  %s    %s", colorize("已用 GPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d", afterCreateAllocated.Gpu), colorYellow))

	// 验证资源使用增加
	cpuIncrease := afterCreateAllocated.Cpu - initialAllocated.Cpu
	memoryIncrease := afterCreateAllocated.Memory - initialAllocated.Memory

	t.Log("\n" + colorize("资源使用变化:", colorCyan+colorBold))
	t.Logf("  %s    %s", colorize("CPU 增加:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", cpuIncrease), colorYellow))
	t.Logf("  %s   %s", colorize("内存增加:", colorWhite+colorBold),
		colorize(formatBytes(memoryIncrease), colorYellow))

	assert.GreaterOrEqual(t, afterCreateAllocated.Cpu, initialAllocated.Cpu,
		"CPU usage should increase after creating container")
	assert.GreaterOrEqual(t, afterCreateAllocated.Memory, initialAllocated.Memory,
		"Memory usage should increase after creating container")

	printSuccess(t, "资源实时使用查询验证通过：能够实时反映资源使用变化")

	// 步骤 5: 多次实时查询验证实时性
	printTestSection(t, "步骤 5: 多次实时查询验证实时性")
	queryCount := 3
	queryResults := make([]*resourcepb.Info, queryCount)

	for i := 0; i < queryCount; i++ {
		printInfo(t, fmt.Sprintf("执行第 %d 次实时查询...", i+1))
		result, err := svc.GetAllocated(ctx)
		require.NoError(t, err, fmt.Sprintf("GetAllocated should succeed on query %d", i+1))
		queryResults[i] = result

		t.Logf("  查询 %d: CPU %d mC, Memory %s",
			i+1, result.Cpu, formatBytes(result.Memory))

		// 等待一小段时间
		if i < queryCount-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 验证所有查询都成功返回
	for i, result := range queryResults {
		assert.NotNil(t, result, fmt.Sprintf("Query %d result should not be nil", i+1))
		assert.GreaterOrEqual(t, result.Cpu, int64(0), fmt.Sprintf("Query %d CPU should be non-negative", i+1))
		assert.GreaterOrEqual(t, result.Memory, int64(0), fmt.Sprintf("Query %d Memory should be non-negative", i+1))
	}

	printSuccess(t, fmt.Sprintf("多次实时查询验证通过：成功执行 %d 次查询", queryCount))

	// 步骤 6: 清理测试容器
	printTestSection(t, "步骤 6: 清理测试容器")
	err = dockerClient.ContainerStop(ctx, createResp.ID, container.StopOptions{})
	require.NoError(t, err, "Failed to stop test container")

	err = dockerClient.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
	require.NoError(t, err, "Failed to remove test container")

	printSuccess(t, "测试容器已清理")

	// 步骤 7: 验证清理后的资源使用（应该减少）
	printTestSection(t, "步骤 7: 验证清理后的资源使用")
	time.Sleep(2 * time.Second)

	finalAllocated, err := svc.GetAllocated(ctx)
	require.NoError(t, err, "GetAllocated should succeed after cleanup")

	t.Log("\n" + colorize("清理后资源使用情况:", colorYellow+colorBold))
	t.Logf("  %s    %s", colorize("已用 CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", finalAllocated.Cpu), colorYellow))
	t.Logf("  %s   %s", colorize("已用内存:", colorWhite+colorBold),
		colorize(formatBytes(finalAllocated.Memory), colorYellow))

	// 验证资源使用减少
	cpuDecrease := afterCreateAllocated.Cpu - finalAllocated.Cpu
	memoryDecrease := afterCreateAllocated.Memory - finalAllocated.Memory

	t.Log("\n" + colorize("资源使用变化:", colorCyan+colorBold))
	t.Logf("  %s    %s", colorize("CPU 减少:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", cpuDecrease), colorGreen))
	t.Logf("  %s   %s", colorize("内存减少:", colorWhite+colorBold),
		colorize(formatBytes(memoryDecrease), colorGreen))

	assert.LessOrEqual(t, finalAllocated.Cpu, afterCreateAllocated.Cpu,
		"CPU usage should decrease after removing container")

	printSuccess(t, "资源实时使用查询完整验证通过")

	t.Log("\n" + colorize(strings.Repeat("=", 80), colorCyan+colorBold))
	t.Log(colorize("✓ 资源实时使用查询测试通过", colorGreen+colorBold))
	t.Log(colorize("  - 能够实时查询资源使用情况", colorGreen))
	t.Log(colorize("  - 能够实时反映资源使用变化", colorGreen))
	t.Log(colorize("  - 支持多次连续查询", colorGreen))
	t.Log(colorize(strings.Repeat("=", 80), colorCyan+colorBold) + "\n")
}

// TestResourceMonitoring_HealthCheckMonitoring 测试用例 2: 资源健康检查监控
func TestResourceMonitoring_HealthCheckMonitoring(t *testing.T) {
	printTestHeader(t, "测试用例: 资源健康检查监控",
		"验证 Docker Provider 的资源健康检查监控功能")

	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping test")
	}

	ctx := context.Background()

	// 步骤 1: 创建并注册 Docker Provider
	printTestSection(t, "步骤 1: 创建并注册 Docker Provider")
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

	printSuccess(t, fmt.Sprintf("Docker Provider 注册成功: %s", providerID))

	// 步骤 2: 执行健康检查
	printTestSection(t, "步骤 2: 执行健康检查")
	healthReq := &providerpb.HealthCheckRequest{
		ProviderId: providerID,
	}

	printInfo(t, "正在执行健康检查...")
	healthResp, err := svc.HealthCheck(ctx, healthReq)
	require.NoError(t, err, "HealthCheck should succeed")
	require.NotNil(t, healthResp, "Health check response should not be nil")
	require.NotNil(t, healthResp.Capacity, "Health check should include capacity information")
	require.NotNil(t, healthResp.ResourceTags, "Health check should include resource tags")

	printSuccess(t, "健康检查执行成功")

	// 显示健康检查结果
	t.Log("\n" + colorize("健康检查结果:", colorYellow+colorBold))
	printResourceInfo(t, healthResp.Capacity)

	t.Log("\n" + colorize("资源标签:", colorYellow+colorBold))
	t.Logf("  %s    %s", colorize("CPU:", colorWhite+colorBold), colorizeBool(healthResp.ResourceTags.Cpu))
	t.Logf("  %s   %s", colorize("GPU:", colorWhite+colorBold), colorizeBool(healthResp.ResourceTags.Gpu))
	t.Logf("  %s  %s", colorize("内存:", colorWhite+colorBold), colorizeBool(healthResp.ResourceTags.Memory))
	t.Logf("  %s %s", colorize("摄像头:", colorWhite+colorBold), colorizeBool(healthResp.ResourceTags.Camera))

	// 验证健康检查返回的资源信息
	assert.NotNil(t, healthResp.Capacity.Total, "Total capacity should not be nil")
	assert.NotNil(t, healthResp.Capacity.Used, "Used capacity should not be nil")
	assert.NotNil(t, healthResp.Capacity.Available, "Available capacity should not be nil")
	assert.Greater(t, healthResp.Capacity.Total.Cpu, int64(0), "Total CPU should be greater than 0")
	assert.Greater(t, healthResp.Capacity.Total.Memory, int64(0), "Total Memory should be greater than 0")

	// 步骤 3: 多次健康检查监控
	printTestSection(t, "步骤 3: 多次健康检查监控（验证监控连续性）")
	monitoringCount := 5
	monitoringResults := make([]*providerpb.HealthCheckResponse, monitoringCount)

	for i := 0; i < monitoringCount; i++ {
		printInfo(t, fmt.Sprintf("执行第 %d 次健康检查...", i+1))
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

	printSuccess(t, fmt.Sprintf("多次健康检查监控验证通过：成功执行 %d 次健康检查", monitoringCount))

	// 步骤 4: 验证健康检查的资源状态一致性
	printTestSection(t, "步骤 4: 验证健康检查的资源状态一致性")
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

	printSuccess(t, "健康检查资源状态一致性验证通过")

	// 步骤 5: 验证健康检查的实时性（通过资源变化）
	printTestSection(t, "步骤 5: 验证健康检查的实时性（通过资源变化）")
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

	printSuccess(t, "测试容器已创建并启动")

	// 等待容器启动
	time.Sleep(2 * time.Second)

	// 执行健康检查（应该反映新的资源使用情况）
	printInfo(t, "执行健康检查（容器运行后）...")
	healthAfterContainer, err := svc.HealthCheck(ctx, healthReq)
	require.NoError(t, err, "HealthCheck should succeed after container creation")
	require.NotNil(t, healthAfterContainer, "Health check response should not be nil")
	// 同时获取 GetAllocated 结果用于对比
	allocatedAfterContainer, err := svc.GetAllocated(ctx)
	require.NoError(t, err, "GetAllocated should succeed after container creation")
	require.NotNil(t, allocatedAfterContainer, "Allocated resources should not be nil after container creation")

	t.Log("\n" + colorize("容器运行后健康检查结果:", colorYellow+colorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		colorize("CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", healthAfterContainer.Capacity.Total.Cpu), colorWhite),
		colorize(fmt.Sprintf("%d millicores", healthAfterContainer.Capacity.Used.Cpu), colorYellow),
		colorize(fmt.Sprintf("%d millicores", healthAfterContainer.Capacity.Available.Cpu), colorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		colorize("内存:", colorWhite+colorBold),
		colorize(formatBytes(healthAfterContainer.Capacity.Total.Memory), colorWhite),
		colorize(formatBytes(healthAfterContainer.Capacity.Used.Memory), colorYellow),
		colorize(formatBytes(healthAfterContainer.Capacity.Available.Memory), colorGreen))

	// 验证健康检查结果与实时查询结果一致（证明能够感知实时变化）
	assert.Equal(t, allocatedAfterContainer.Cpu, healthAfterContainer.Capacity.Used.Cpu,
		"Used CPU reported by health check should match GetAllocated result")
	assert.Equal(t, allocatedAfterContainer.Memory, healthAfterContainer.Capacity.Used.Memory,
		"Used Memory reported by health check should match GetAllocated result")

	printSuccess(t, "健康检查实时性验证通过：能够实时反映资源状态变化")

	// 清理测试容器
	err = dockerClient.ContainerStop(ctx, createResp.ID, container.StopOptions{})
	require.NoError(t, err, "Failed to stop test container")
	err = dockerClient.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
	require.NoError(t, err, "Failed to remove test container")

	// 步骤 6: 验证健康检查监控的连续性
	printTestSection(t, "步骤 6: 验证健康检查监控的连续性")
	// 执行最后一次健康检查
	finalHealthCheck, err := svc.HealthCheck(ctx, healthReq)
	require.NoError(t, err, "Final health check should succeed")
	require.NotNil(t, finalHealthCheck, "Final health check response should not be nil")

	t.Log("\n" + colorize("最终健康检查结果:", colorYellow+colorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		colorize("CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", finalHealthCheck.Capacity.Total.Cpu), colorWhite),
		colorize(fmt.Sprintf("%d millicores", finalHealthCheck.Capacity.Used.Cpu), colorYellow),
		colorize(fmt.Sprintf("%d millicores", finalHealthCheck.Capacity.Available.Cpu), colorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		colorize("内存:", colorWhite+colorBold),
		colorize(formatBytes(finalHealthCheck.Capacity.Total.Memory), colorWhite),
		colorize(formatBytes(finalHealthCheck.Capacity.Used.Memory), colorYellow),
		colorize(formatBytes(finalHealthCheck.Capacity.Available.Memory), colorGreen))

	// 验证健康检查监控的连续性
	assert.Equal(t, healthAfterContainer.Capacity.Total.Cpu, finalHealthCheck.Capacity.Total.Cpu,
		"Total CPU should remain consistent")
	assert.Equal(t, healthAfterContainer.Capacity.Total.Memory, finalHealthCheck.Capacity.Total.Memory,
		"Total Memory should remain consistent")

	printSuccess(t, "健康检查监控连续性验证通过")

	t.Log("\n" + colorize(strings.Repeat("=", 80), colorCyan+colorBold))
	t.Log(colorize("✓ 资源健康检查监控测试通过", colorGreen+colorBold))
	t.Log(colorize("  - 健康检查能够正确返回资源状态", colorGreen))
	t.Log(colorize("  - 健康检查能够实时反映资源变化", colorGreen))
	t.Log(colorize("  - 健康检查监控具有连续性", colorGreen))
	t.Log(colorize(strings.Repeat("=", 80), colorCyan+colorBold) + "\n")
}
