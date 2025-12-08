package situation_awareness

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/9triver/iarnet/providers/k8s/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCase: 远程 Kubernetes 资源感知测试
// 测试目的：验证远程节点接入 Kubernetes provider 后，当前节点可以通过 gossip 协议
// 感知远程节点的 Kubernetes provider 所提供的资源容量

// TestRemoteK8sResourcePerception 测试远程 Kubernetes 资源感知
func TestRemoteK8sResourcePerception(t *testing.T) {
	printTestHeader(t, "测试用例: 远程 Kubernetes 资源感知",
		"验证通过 Gossip 协议感知远程节点的 Kubernetes Provider 资源容量")

	ctx := context.Background()

	// 步骤 1: 创建当前节点（本地节点）
	printTestSection(t, "步骤 1: 创建当前节点（本地节点）")
	currentNodeID := "test-node-current"
	currentNodeName := "test-current"
	currentAddress := "localhost:50005"
	currentSchedulerAddress := "localhost:50006"
	domainID := "test-domain"

	currentManager := discovery.NewNodeDiscoveryManager(
		currentNodeID,
		currentNodeName,
		currentAddress,
		currentSchedulerAddress,
		domainID,
		[]string{},
		30*time.Second,
		180*time.Second,
	)

	// 设置当前节点的资源信息（没有 Kubernetes provider）
	currentManager.UpdateLocalNode(
		&types.Capacity{
			Total:     &types.Info{CPU: 4000, Memory: 4 * 1024 * 1024 * 1024, GPU: 0},
			Used:      &types.Info{CPU: 1000, Memory: 1 * 1024 * 1024 * 1024, GPU: 0},
			Available: &types.Info{CPU: 3000, Memory: 3 * 1024 * 1024 * 1024, GPU: 0},
		},
		discovery.NewResourceTags(true, false, true, false),
	)

	err := currentManager.Start(ctx)
	require.NoError(t, err, "Failed to start current node manager")
	defer currentManager.Stop()

	printSuccess(t, "当前节点创建并启动成功")
	printNetworkTopology(t, currentManager, "当前节点初始状态")

	// 步骤 2: 创建远程节点并接入 Kubernetes Provider
	printTestSection(t, "步骤 2: 创建远程节点并接入 Kubernetes Provider")
	if !isK8sAvailable() {
		t.Skip("Kubernetes is not available, skipping test")
	}

	remoteNodeID := "test-node-remote"
	remoteNodeName := "test-remote"
	remoteAddress := "192.168.1.200:50005"
	remoteSchedulerAddress := "192.168.1.200:50006"

	// 创建远程节点的 Kubernetes Provider
	remoteK8sProvider, err := createK8sTestService()
	require.NoError(t, err, "Failed to create remote Kubernetes provider")

	// 确保类型正确（使用 provider.Service）
	var _ *provider.Service = remoteK8sProvider

	defer func() {
		if remoteK8sProvider != nil {
			remoteK8sProvider.Close()
		}
	}()

	// 注册远程节点的 Kubernetes Provider
	remoteProviderID := "remote-k8s-provider"
	connectReq := &providerpb.ConnectRequest{
		ProviderId: remoteProviderID,
	}
	connectResp, err := remoteK8sProvider.Connect(ctx, connectReq)
	require.NoError(t, err, "Failed to connect remote Kubernetes provider")
	require.True(t, connectResp.Success, "Remote Kubernetes provider connection should succeed")

	printSuccess(t, fmt.Sprintf("远程节点 Kubernetes Provider 注册成功: %s", remoteProviderID))
	printInfo(t, fmt.Sprintf("Provider 类型: %s", connectResp.ProviderType.Name))

	// 步骤 3: 获取远程节点的 Kubernetes Provider 资源容量
	printTestSection(t, "步骤 3: 获取远程节点的 Kubernetes Provider 资源容量")
	capacityReq := &providerpb.GetCapacityRequest{
		ProviderId: remoteProviderID,
	}

	capacityResp, err := remoteK8sProvider.GetCapacity(ctx, capacityReq)
	require.NoError(t, err, "Failed to get remote Kubernetes provider capacity")
	require.NotNil(t, capacityResp, "Capacity response should not be nil")
	require.NotNil(t, capacityResp.Capacity, "Capacity should not be nil")

	remoteCapacity := capacityResp.Capacity
	printSuccess(t, "成功获取远程节点 Kubernetes Provider 资源容量")

	t.Log("\n" + colorize("远程节点 Kubernetes Provider 资源容量:", colorYellow+colorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		colorize("CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", remoteCapacity.Total.Cpu), colorWhite),
		colorize(fmt.Sprintf("%d millicores", remoteCapacity.Used.Cpu), colorYellow),
		colorize(fmt.Sprintf("%d millicores", remoteCapacity.Available.Cpu), colorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		colorize("内存:", colorWhite+colorBold),
		colorize(formatBytes(remoteCapacity.Total.Memory), colorWhite),
		colorize(formatBytes(remoteCapacity.Used.Memory), colorYellow),
		colorize(formatBytes(remoteCapacity.Available.Memory), colorGreen))
	if remoteCapacity.Total.Gpu > 0 {
		t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
			colorize("GPU:", colorWhite+colorBold),
			colorize(fmt.Sprintf("%d", remoteCapacity.Total.Gpu), colorWhite),
			colorize(fmt.Sprintf("%d", remoteCapacity.Used.Gpu), colorYellow),
			colorize(fmt.Sprintf("%d", remoteCapacity.Available.Gpu), colorGreen))
	}

	// 获取资源标签
	healthReq := &providerpb.HealthCheckRequest{
		ProviderId: remoteProviderID,
	}
	healthResp, err := remoteK8sProvider.HealthCheck(ctx, healthReq)
	require.NoError(t, err, "Failed to get remote Kubernetes provider health check")
	require.NotNil(t, healthResp, "Health check response should not be nil")
	require.NotNil(t, healthResp.ResourceTags, "Resource tags should not be nil")

	remoteResourceTags := discovery.NewResourceTags(
		healthResp.ResourceTags.Cpu,
		healthResp.ResourceTags.Gpu,
		healthResp.ResourceTags.Memory,
		healthResp.ResourceTags.Camera,
	)

	t.Log("\n" + colorize("远程节点 Kubernetes Provider 资源标签:", colorYellow+colorBold))
	t.Logf("  %s    %s", colorize("CPU:", colorWhite+colorBold), colorizeBool(remoteResourceTags.CPU))
	t.Logf("  %s   %s", colorize("GPU:", colorWhite+colorBold), colorizeBool(remoteResourceTags.GPU))
	t.Logf("  %s  %s", colorize("内存:", colorWhite+colorBold), colorizeBool(remoteResourceTags.Memory))
	t.Logf("  %s %s", colorize("摄像头:", colorWhite+colorBold), colorizeBool(remoteResourceTags.Camera))

	// 步骤 4: 构建远程节点信息（包含 Kubernetes Provider 资源容量）
	printTestSection(t, "步骤 4: 构建远程节点信息（包含 Kubernetes Provider 资源容量）")

	// 转换资源容量格式
	remoteNodeCapacity := &types.Capacity{
		Total: &types.Info{
			CPU:    remoteCapacity.Total.Cpu,
			Memory: remoteCapacity.Total.Memory,
			GPU:    remoteCapacity.Total.Gpu,
		},
		Used: &types.Info{
			CPU:    remoteCapacity.Used.Cpu,
			Memory: remoteCapacity.Used.Memory,
			GPU:    remoteCapacity.Used.Gpu,
		},
		Available: &types.Info{
			CPU:    remoteCapacity.Available.Cpu,
			Memory: remoteCapacity.Available.Memory,
			GPU:    remoteCapacity.Available.Gpu,
		},
	}

	remotePeerNode := &discovery.PeerNode{
		NodeID:           remoteNodeID,
		NodeName:         remoteNodeName,
		Address:          remoteAddress,
		SchedulerAddress: remoteSchedulerAddress,
		DomainID:         domainID,           // 同域节点
		ResourceCapacity: remoteNodeCapacity, // 包含 Kubernetes Provider 的资源容量
		ResourceTags:     remoteResourceTags, // Kubernetes Provider 的资源标签
		Status:           discovery.NodeStatusOnline,
		LastSeen:         time.Now(),
		LastUpdated:      time.Now(),
		DiscoveredAt:     time.Now(),
		SourcePeer:       currentAddress,
		Version:          1,
		GossipCount:      0,
	}

	printSuccess(t, "远程节点信息构建完成（包含 Kubernetes Provider 资源容量）")
	t.Logf("  节点 ID: %s", remotePeerNode.NodeID)
	t.Logf("  节点名称: %s", remotePeerNode.NodeName)
	t.Logf("  节点地址: %s", remotePeerNode.Address)
	t.Logf("  资源来源: Kubernetes Provider")

	// 步骤 5: 通过 Gossip 协议传播远程节点信息到当前节点
	printTestSection(t, "步骤 5: 通过 Gossip 协议传播远程节点信息到当前节点")

	// 设置节点发现回调
	discoveredNodes := make(chan string, 10)
	currentManager.SetOnNodeDiscovered(func(node *discovery.PeerNode) {
		discoveredNodes <- node.NodeID
		printSuccess(t, fmt.Sprintf("远程节点发现回调触发: %s (%s)", node.NodeName, node.NodeID))
	})

	// 模拟 Gossip 消息：当前节点接收到远程节点的信息
	currentManager.ProcessNodeInfo(remotePeerNode, currentAddress)
	printSuccess(t, "远程节点信息已通过 Gossip 协议传播到当前节点")

	// 等待回调执行
	select {
	case nodeID := <-discoveredNodes:
		assert.Equal(t, remoteNodeID, nodeID, "Discovered node ID should match")
	case <-time.After(1 * time.Second):
		t.Logf("  %s 回调可能未触发或已触发", colorize("警告:", colorYellow))
	}

	// 步骤 6: 验证当前节点能够感知远程节点的 Kubernetes Provider 资源容量
	printTestSection(t, "步骤 6: 验证当前节点能够感知远程节点的 Kubernetes Provider 资源容量")

	knownNodes := currentManager.GetKnownNodes()
	require.Equal(t, 1, len(knownNodes), "Current node should know about the remote node")

	discoveredRemoteNode := knownNodes[0]
	assert.Equal(t, remoteNodeID, discoveredRemoteNode.NodeID, "Remote node ID should match")
	assert.Equal(t, remoteNodeName, discoveredRemoteNode.NodeName, "Remote node name should match")
	assert.Equal(t, remoteAddress, discoveredRemoteNode.Address, "Remote node address should match")

	// 验证资源容量感知
	require.NotNil(t, discoveredRemoteNode.ResourceCapacity, "Remote node resource capacity should not be nil")
	require.NotNil(t, discoveredRemoteNode.ResourceCapacity.Total, "Remote node total capacity should not be nil")

	// 验证资源容量与 Kubernetes Provider 的资源容量一致
	assert.Equal(t, remoteCapacity.Total.Cpu, discoveredRemoteNode.ResourceCapacity.Total.CPU,
		"Remote node total CPU should match Kubernetes provider capacity")
	assert.Equal(t, remoteCapacity.Total.Memory, discoveredRemoteNode.ResourceCapacity.Total.Memory,
		"Remote node total Memory should match Kubernetes provider capacity")
	assert.Equal(t, remoteCapacity.Total.Gpu, discoveredRemoteNode.ResourceCapacity.Total.GPU,
		"Remote node total GPU should match Kubernetes provider capacity")

	assert.Equal(t, remoteCapacity.Used.Cpu, discoveredRemoteNode.ResourceCapacity.Used.CPU,
		"Remote node used CPU should match Kubernetes provider capacity")
	assert.Equal(t, remoteCapacity.Used.Memory, discoveredRemoteNode.ResourceCapacity.Used.Memory,
		"Remote node used Memory should match Kubernetes provider capacity")
	assert.Equal(t, remoteCapacity.Used.Gpu, discoveredRemoteNode.ResourceCapacity.Used.GPU,
		"Remote node used GPU should match Kubernetes provider capacity")

	assert.Equal(t, remoteCapacity.Available.Cpu, discoveredRemoteNode.ResourceCapacity.Available.CPU,
		"Remote node available CPU should match Kubernetes provider capacity")
	assert.Equal(t, remoteCapacity.Available.Memory, discoveredRemoteNode.ResourceCapacity.Available.Memory,
		"Remote node available Memory should match Kubernetes provider capacity")
	assert.Equal(t, remoteCapacity.Available.Gpu, discoveredRemoteNode.ResourceCapacity.Available.GPU,
		"Remote node available GPU should match Kubernetes provider capacity")

	printSuccess(t, "当前节点成功感知到远程节点的 Kubernetes Provider 资源容量")

	t.Log("\n" + colorize("感知到的远程节点资源容量:", colorYellow+colorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		colorize("CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", discoveredRemoteNode.ResourceCapacity.Total.CPU), colorWhite),
		colorize(fmt.Sprintf("%d millicores", discoveredRemoteNode.ResourceCapacity.Used.CPU), colorYellow),
		colorize(fmt.Sprintf("%d millicores", discoveredRemoteNode.ResourceCapacity.Available.CPU), colorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		colorize("内存:", colorWhite+colorBold),
		colorize(formatBytes(discoveredRemoteNode.ResourceCapacity.Total.Memory), colorWhite),
		colorize(formatBytes(discoveredRemoteNode.ResourceCapacity.Used.Memory), colorYellow),
		colorize(formatBytes(discoveredRemoteNode.ResourceCapacity.Available.Memory), colorGreen))

	// 验证资源标签感知
	require.NotNil(t, discoveredRemoteNode.ResourceTags, "Remote node resource tags should not be nil")
	assert.Equal(t, remoteResourceTags.CPU, discoveredRemoteNode.ResourceTags.CPU,
		"Remote node CPU tag should match Kubernetes provider")
	assert.Equal(t, remoteResourceTags.GPU, discoveredRemoteNode.ResourceTags.GPU,
		"Remote node GPU tag should match Kubernetes provider")
	assert.Equal(t, remoteResourceTags.Memory, discoveredRemoteNode.ResourceTags.Memory,
		"Remote node Memory tag should match Kubernetes provider")
	assert.Equal(t, remoteResourceTags.Camera, discoveredRemoteNode.ResourceTags.Camera,
		"Remote node Camera tag should match Kubernetes provider")

	printSuccess(t, "当前节点成功感知到远程节点的 Kubernetes Provider 资源标签")

	// 步骤 7: 验证聚合视图包含远程节点的 Kubernetes Provider 资源
	printTestSection(t, "步骤 7: 验证聚合视图包含远程节点的 Kubernetes Provider 资源")

	aggregateView := currentManager.GetAggregateView()
	require.NotNil(t, aggregateView, "Aggregate view should not be nil")

	aggregatedCapacity := aggregateView.GetAggregatedCapacity()
	require.NotNil(t, aggregatedCapacity, "Aggregated capacity should not be nil")

	// 验证聚合资源包含远程节点的资源
	// 当前节点: CPU 4000, Memory 4GB
	// 远程节点: CPU (来自 Kubernetes Provider), Memory (来自 Kubernetes Provider)
	expectedTotalCPU := int64(4000) + remoteCapacity.Total.Cpu
	expectedTotalMemory := int64(4*1024*1024*1024) + remoteCapacity.Total.Memory

	assert.Equal(t, expectedTotalCPU, aggregatedCapacity.Total.CPU,
		"Aggregated total CPU should include remote node Kubernetes provider capacity")
	assert.Equal(t, expectedTotalMemory, aggregatedCapacity.Total.Memory,
		"Aggregated total Memory should include remote node Kubernetes provider capacity")

	t.Log("\n" + colorize("聚合资源容量（包含远程节点 Kubernetes Provider）:", colorYellow+colorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		colorize("CPU:", colorWhite+colorBold),
		colorize(fmt.Sprintf("%d millicores", aggregatedCapacity.Total.CPU), colorWhite),
		colorize(fmt.Sprintf("%d millicores", aggregatedCapacity.Used.CPU), colorYellow),
		colorize(fmt.Sprintf("%d millicores", aggregatedCapacity.Available.CPU), colorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		colorize("内存:", colorWhite+colorBold),
		colorize(formatBytes(aggregatedCapacity.Total.Memory), colorWhite),
		colorize(formatBytes(aggregatedCapacity.Used.Memory), colorYellow),
		colorize(formatBytes(aggregatedCapacity.Available.Memory), colorGreen))

	printSuccess(t, "聚合视图成功包含远程节点的 Kubernetes Provider 资源")

	// 打印最终网络拓扑
	printNetworkTopology(t, currentManager, "远程 Kubernetes 资源感知测试后的网络拓扑")

	t.Log("\n" + colorize(strings.Repeat("=", 80), colorCyan+colorBold))
	t.Log(colorize("✓ 远程 Kubernetes 资源感知测试通过", colorGreen+colorBold))
	t.Log(colorize("  - 远程节点成功接入 Kubernetes Provider", colorGreen))
	t.Log(colorize("  - 当前节点通过 Gossip 协议感知到远程节点的 Kubernetes Provider 资源容量", colorGreen))
	t.Log(colorize("  - 聚合视图成功包含远程节点的 Kubernetes Provider 资源", colorGreen))
	t.Log(colorize(strings.Repeat("=", 80), colorCyan+colorBold) + "\n")
}


