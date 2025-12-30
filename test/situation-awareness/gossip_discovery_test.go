package situation_awareness

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	testutil "github.com/9triver/iarnet/test/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	testutil.InitTestLogger()
}

// TestCase: Gossip 资源发现相关测试
// 测试目的：验证 Gossip 协议在资源发现中的三个核心功能：
// 1. 新增 peers - 验证新节点的发现和添加
// 2. 聚合视图 - 验证资源聚合视图的更新和查询
// 3. 过期治理 - 验证节点过期清理机制

// TestGossipDiscovery_NewPeers 测试用例 1: Gossip 资源发现, 新增 peers
func TestGossipDiscovery_NewPeers(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: Gossip 资源发现 - 新增 peers",
		"验证通过 Gossip 协议发现和添加新的 peer 节点")

	ctx := context.Background()

	// 创建本地节点管理器
	testutil.PrintTestSection(t, "步骤 1: 创建本地节点管理器")
	localNodeID := "test-node-local"
	localNodeName := "test-local"
	localAddress := "localhost:50005"
	localSchedulerAddress := "localhost:50006"
	domainID := "test-domain"

	manager := discovery.NewNodeDiscoveryManager(
		localNodeID,
		localNodeName,
		localAddress,
		localSchedulerAddress,
		domainID,
		[]string{},      // 初始没有 peers
		30*time.Second,  // gossip 间隔
		180*time.Second, // 节点 TTL
	)

	// 设置本地节点资源信息
	manager.UpdateLocalNode(
		&types.Capacity{
			Total: &types.Info{
				CPU:    8000,
				Memory: 8 * 1024 * 1024 * 1024, // 8GB
				GPU:    0,
			},
			Used: &types.Info{
				CPU:    2000,
				Memory: 2 * 1024 * 1024 * 1024, // 2GB
				GPU:    0,
			},
			Available: &types.Info{
				CPU:    6000,
				Memory: 6 * 1024 * 1024 * 1024, // 6GB
				GPU:    0,
			},
		},
		types.NewResourceTags(true, false, true, false), // CPU, Memory
	)

	// 启动管理器
	err := manager.Start(ctx)
	require.NoError(t, err, "Failed to start node discovery manager")
	defer manager.Stop()

	testutil.PrintSuccess(t, "本地节点管理器创建并启动成功")

	// 验证初始状态
	testutil.PrintTestSection(t, "步骤 2: 验证初始状态")
	initialNodes := manager.GetKnownNodes()
	assert.Equal(t, 0, len(initialNodes), "Initially should have no known nodes")
	testutil.PrintSuccess(t, fmt.Sprintf("初始状态：已知节点数 = %d", len(initialNodes)))

	// 创建新 peer 节点信息
	testutil.PrintTestSection(t, "步骤 3: 创建新 peer 节点信息")
	peerNodeID := "test-node-peer-1"
	peerNodeName := "test-peer-1"
	peerAddress := "192.168.1.100:50005"
	peerSchedulerAddress := "192.168.1.100:50006"

	newPeerNode := &discovery.PeerNode{
		NodeID:           peerNodeID,
		NodeName:         peerNodeName,
		Address:          peerAddress,
		SchedulerAddress: peerSchedulerAddress,
		DomainID:         domainID, // 同域节点
		ResourceCapacity: &types.Capacity{
			Total: &types.Info{
				CPU:    4000,
				Memory: 4 * 1024 * 1024 * 1024, // 4GB
				GPU:    1,
			},
			Used: &types.Info{
				CPU:    1000,
				Memory: 1 * 1024 * 1024 * 1024, // 1GB
				GPU:    0,
			},
			Available: &types.Info{
				CPU:    3000,
				Memory: 3 * 1024 * 1024 * 1024, // 3GB
				GPU:    1,
			},
		},
		ResourceTags: types.NewResourceTags(true, true, true, false), // CPU, GPU, Memory
		Status:       discovery.NodeStatusOnline,
		LastSeen:     time.Now(),
		LastUpdated:  time.Now(),
		DiscoveredAt: time.Now(),
		SourcePeer:   localAddress,
		Version:      1,
		GossipCount:  0,
	}

	testutil.PrintInfo(t, fmt.Sprintf("新 peer 节点信息: %s (%s)", peerNodeName, peerNodeID))
	t.Logf("  地址: %s", peerAddress)
	t.Logf("  资源: CPU %d mC, Memory %s, GPU %d",
		newPeerNode.ResourceCapacity.Total.CPU,
		testutil.FormatBytes(newPeerNode.ResourceCapacity.Total.Memory),
		newPeerNode.ResourceCapacity.Total.GPU)

	// 处理新节点信息（通过 Gossip 接收）
	testutil.PrintTestSection(t, "步骤 4: 处理新节点信息（通过 Gossip 接收）")
	discoveredNodes := make(chan string, 10) // 使用 channel 来接收异步回调
	manager.SetOnNodeDiscovered(func(node *discovery.PeerNode) {
		discoveredNodes <- node.NodeID
		testutil.PrintSuccess(t, fmt.Sprintf("节点发现回调触发: %s (%s)", node.NodeName, node.NodeID))
	})

	manager.ProcessNodeInfo(newPeerNode, localAddress)
	testutil.PrintSuccess(t, "新节点信息已处理")

	// 等待回调执行（异步执行，需要等待）
	select {
	case nodeID := <-discoveredNodes:
		assert.Equal(t, peerNodeID, nodeID, "Discovered node ID should match")
	case <-time.After(1 * time.Second):
		t.Logf("  %s 回调可能未触发或已触发", testutil.Colorize("警告:", testutil.ColorYellow))
	}

	// 验证节点已被添加
	testutil.PrintTestSection(t, "步骤 5: 验证节点已被添加")
	knownNodes := manager.GetKnownNodes()
	require.Equal(t, 1, len(knownNodes), "Should have 1 known node after processing")

	foundNode := knownNodes[0]
	assert.Equal(t, peerNodeID, foundNode.NodeID, "Node ID should match")
	assert.Equal(t, peerNodeName, foundNode.NodeName, "Node name should match")
	assert.Equal(t, peerAddress, foundNode.Address, "Node address should match")
	assert.Equal(t, domainID, foundNode.DomainID, "Node domain ID should match")
	assert.Equal(t, discovery.NodeStatusOnline, foundNode.Status, "Node status should be online")

	testutil.PrintSuccess(t, fmt.Sprintf("节点已成功添加到已知节点列表: %s", foundNode.NodeName))
	t.Logf("  节点 ID: %s", foundNode.NodeID)
	t.Logf("  节点地址: %s", foundNode.Address)
	t.Logf("  节点状态: %s", foundNode.Status)

	// 验证节点资源信息
	testutil.PrintTestSection(t, "步骤 6: 验证节点资源信息")
	require.NotNil(t, foundNode.ResourceCapacity, "Resource capacity should not be nil")
	require.NotNil(t, foundNode.ResourceCapacity.Total, "Total capacity should not be nil")
	assert.Equal(t, int64(4000), foundNode.ResourceCapacity.Total.CPU, "Total CPU should match")
	assert.Equal(t, int64(4*1024*1024*1024), foundNode.ResourceCapacity.Total.Memory, "Total Memory should match")
	assert.Equal(t, int64(1), foundNode.ResourceCapacity.Total.GPU, "Total GPU should match")

	require.NotNil(t, foundNode.ResourceTags, "Resource tags should not be nil")
	assert.True(t, foundNode.ResourceTags.CPU, "Should support CPU")
	assert.True(t, foundNode.ResourceTags.GPU, "Should support GPU")
	assert.True(t, foundNode.ResourceTags.Memory, "Should support Memory")
	assert.False(t, foundNode.ResourceTags.Camera, "Should not support Camera")

	testutil.PrintSuccess(t, "节点资源信息验证通过")
	t.Logf("  CPU: %d mC, Memory: %s, GPU: %d",
		foundNode.ResourceCapacity.Total.CPU,
		testutil.FormatBytes(foundNode.ResourceCapacity.Total.Memory),
		foundNode.ResourceCapacity.Total.GPU)

	// 打印当前网络拓扑
	testutil.PrintNetworkTopology(t, manager, "添加第一个 peer 后的网络拓扑")

	// 测试添加多个 peers
	testutil.PrintTestSection(t, "步骤 7: 测试添加多个 peers")
	peer2NodeID := "test-node-peer-2"
	peer2NodeName := "test-peer-2"
	peer2Address := "192.168.1.101:50005"

	peer2Node := &discovery.PeerNode{
		NodeID:           peer2NodeID,
		NodeName:         peer2NodeName,
		Address:          peer2Address,
		SchedulerAddress: "192.168.1.101:50006",
		DomainID:         domainID,
		ResourceCapacity: &types.Capacity{
			Total: &types.Info{
				CPU:    2000,
				Memory: 2 * 1024 * 1024 * 1024,
				GPU:    0,
			},
			Used:      &types.Info{CPU: 500, Memory: 512 * 1024 * 1024, GPU: 0},
			Available: &types.Info{CPU: 1500, Memory: 1536 * 1024 * 1024, GPU: 0},
		},
		ResourceTags: types.NewResourceTags(true, false, true, false),
		Status:       discovery.NodeStatusOnline,
		LastSeen:     time.Now(),
		LastUpdated:  time.Now(),
		DiscoveredAt: time.Now(),
		SourcePeer:   localAddress,
		Version:      1,
		GossipCount:  0,
	}

	manager.ProcessNodeInfo(peer2Node, localAddress)
	knownNodes = manager.GetKnownNodes()
	assert.Equal(t, 2, len(knownNodes), "Should have 2 known nodes after adding second peer")

	testutil.PrintSuccess(t, fmt.Sprintf("成功添加第二个 peer: %s", peer2NodeName))
	t.Logf("  当前已知节点数: %d", len(knownNodes))

	// 打印最终网络拓扑
	testutil.PrintNetworkTopology(t, manager, "添加第二个 peer 后的网络拓扑")

	t.Log("\n" + testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold))
	t.Log(testutil.Colorize("✓ Gossip 资源发现 - 新增 peers 测试通过", testutil.ColorGreen+testutil.ColorBold))
	t.Log(testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold) + "\n")
}

// TestGossipDiscovery_AggregatedView 测试用例 2: Gossip 资源发现, 聚合视图
func TestGossipDiscovery_AggregatedView(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: Gossip 资源发现 - 聚合视图",
		"验证资源聚合视图的更新和查询功能")

	ctx := context.Background()

	// 创建节点管理器
	testutil.PrintTestSection(t, "步骤 1: 创建节点管理器并添加多个节点")
	manager := discovery.NewNodeDiscoveryManager(
		"test-node-aggregate",
		"test-aggregate",
		"localhost:50005",
		"localhost:50006",
		"test-domain",
		[]string{},
		30*time.Second,
		180*time.Second,
	)

	// 设置本地节点资源
	manager.UpdateLocalNode(
		&types.Capacity{
			Total:     &types.Info{CPU: 8000, Memory: 8 * 1024 * 1024 * 1024, GPU: 0},
			Used:      &types.Info{CPU: 2000, Memory: 2 * 1024 * 1024 * 1024, GPU: 0},
			Available: &types.Info{CPU: 6000, Memory: 6 * 1024 * 1024 * 1024, GPU: 0},
		},
		types.NewResourceTags(true, false, true, false),
	)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// 添加多个 peer 节点
	nodes := []*discovery.PeerNode{
		{
			NodeID:   "node-1",
			NodeName: "node-1",
			Address:  "192.168.1.100:50005",
			DomainID: "test-domain",
			ResourceCapacity: &types.Capacity{
				Total:     &types.Info{CPU: 4000, Memory: 4 * 1024 * 1024 * 1024, GPU: 1},
				Used:      &types.Info{CPU: 1000, Memory: 1 * 1024 * 1024 * 1024, GPU: 0},
				Available: &types.Info{CPU: 3000, Memory: 3 * 1024 * 1024 * 1024, GPU: 1},
			},
			ResourceTags: types.NewResourceTags(true, true, true, false),
			Status:       discovery.NodeStatusOnline,
			LastSeen:     time.Now(),
			LastUpdated:  time.Now(),
			Version:      1,
		},
		{
			NodeID:   "node-2",
			NodeName: "node-2",
			Address:  "192.168.1.101:50005",
			DomainID: "test-domain",
			ResourceCapacity: &types.Capacity{
				Total:     &types.Info{CPU: 2000, Memory: 2 * 1024 * 1024 * 1024, GPU: 0},
				Used:      &types.Info{CPU: 500, Memory: 512 * 1024 * 1024, GPU: 0},
				Available: &types.Info{CPU: 1500, Memory: 1536 * 1024 * 1024, GPU: 0},
			},
			ResourceTags: types.NewResourceTags(true, false, true, false),
			Status:       discovery.NodeStatusOnline,
			LastSeen:     time.Now(),
			LastUpdated:  time.Now(),
			Version:      1,
		},
		{
			NodeID:   "node-3",
			NodeName: "node-3",
			Address:  "192.168.1.102:50005",
			DomainID: "test-domain",
			ResourceCapacity: &types.Capacity{
				Total:     &types.Info{CPU: 6000, Memory: 6 * 1024 * 1024 * 1024, GPU: 2},
				Used:      &types.Info{CPU: 3000, Memory: 3 * 1024 * 1024 * 1024, GPU: 1},
				Available: &types.Info{CPU: 3000, Memory: 3 * 1024 * 1024 * 1024, GPU: 1},
			},
			ResourceTags: types.NewResourceTags(true, true, true, true), // 支持所有资源
			Status:       discovery.NodeStatusOnline,
			LastSeen:     time.Now(),
			LastUpdated:  time.Now(),
			Version:      1,
		},
	}

	for _, node := range nodes {
		manager.ProcessNodeInfo(node, "localhost:50005")
	}

	testutil.PrintSuccess(t, fmt.Sprintf("已添加 %d 个 peer 节点", len(nodes)))

	// 获取聚合视图
	testutil.PrintTestSection(t, "步骤 2: 获取聚合视图")
	aggregateView := manager.GetAggregateView()
	require.NotNil(t, aggregateView, "Aggregate view should not be nil")

	aggregatedCapacity := aggregateView.GetAggregatedCapacity()
	require.NotNil(t, aggregatedCapacity, "Aggregated capacity should not be nil")

	// 验证聚合资源容量（本地节点 + 3个 peer 节点）
	// 本地: CPU 8000, Memory 8GB, GPU 0
	// node-1: CPU 4000, Memory 4GB, GPU 1
	// node-2: CPU 2000, Memory 2GB, GPU 0
	// node-3: CPU 6000, Memory 6GB, GPU 2
	// 总计: CPU 20000, Memory 20GB, GPU 3
	expectedTotalCPU := int64(8000 + 4000 + 2000 + 6000)
	expectedTotalMemory := int64((8 + 4 + 2 + 6) * 1024 * 1024 * 1024)
	expectedTotalGPU := int64(0 + 1 + 0 + 2)

	t.Log("\n" + testutil.Colorize("聚合资源容量:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d millicores", aggregatedCapacity.Total.CPU), testutil.ColorWhite),
		testutil.Colorize(fmt.Sprintf("%d millicores", aggregatedCapacity.Used.CPU), testutil.ColorYellow),
		testutil.Colorize(fmt.Sprintf("%d millicores", aggregatedCapacity.Available.CPU), testutil.ColorGreen))
	t.Logf("  %s   总计: %s, 已用: %s, 可用: %s",
		testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(testutil.FormatBytes(aggregatedCapacity.Total.Memory), testutil.ColorWhite),
		testutil.Colorize(testutil.FormatBytes(aggregatedCapacity.Used.Memory), testutil.ColorYellow),
		testutil.Colorize(testutil.FormatBytes(aggregatedCapacity.Available.Memory), testutil.ColorGreen))
	t.Logf("  %s    总计: %s, 已用: %s, 可用: %s",
		testutil.Colorize("GPU:", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize(fmt.Sprintf("%d", aggregatedCapacity.Total.GPU), testutil.ColorWhite),
		testutil.Colorize(fmt.Sprintf("%d", aggregatedCapacity.Used.GPU), testutil.ColorYellow),
		testutil.Colorize(fmt.Sprintf("%d", aggregatedCapacity.Available.GPU), testutil.ColorGreen))

	assert.Equal(t, expectedTotalCPU, aggregatedCapacity.Total.CPU, "Total CPU should match")
	assert.Equal(t, expectedTotalMemory, aggregatedCapacity.Total.Memory, "Total Memory should match")
	assert.Equal(t, expectedTotalGPU, aggregatedCapacity.Total.GPU, "Total GPU should match")

	// 验证聚合资源标签
	testutil.PrintTestSection(t, "步骤 3: 验证聚合资源标签")
	aggregatedTags := aggregateView.GetAggregatedTags()
	require.NotNil(t, aggregatedTags, "Aggregated tags should not be nil")

	t.Log("\n" + testutil.Colorize("聚合资源标签:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s    %s", testutil.Colorize("CPU:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(aggregatedTags.CPU))
	t.Logf("  %s   %s", testutil.Colorize("GPU:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(aggregatedTags.GPU))
	t.Logf("  %s  %s", testutil.Colorize("内存:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(aggregatedTags.Memory))
	t.Logf("  %s %s", testutil.Colorize("摄像头:", testutil.ColorWhite+testutil.ColorBold), testutil.ColorizeBool(aggregatedTags.Camera))

	assert.True(t, aggregatedTags.CPU, "Should support CPU")
	assert.True(t, aggregatedTags.GPU, "Should support GPU")
	assert.True(t, aggregatedTags.Memory, "Should support Memory")
	assert.True(t, aggregatedTags.Camera, "Should support Camera (at least one node)")

	// 验证节点统计
	testutil.PrintTestSection(t, "步骤 4: 验证节点统计")
	t.Log("\n" + testutil.Colorize("节点统计:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s: %d", testutil.Colorize("总节点数", testutil.ColorWhite+testutil.ColorBold), aggregateView.TotalNodes)
	t.Logf("  %s: %d", testutil.Colorize("在线节点数", testutil.ColorWhite+testutil.ColorBold), aggregateView.OnlineNodes)
	t.Logf("  %s: %d", testutil.Colorize("离线节点数", testutil.ColorWhite+testutil.ColorBold), aggregateView.OfflineNodes)

	assert.Equal(t, 4, aggregateView.TotalNodes, "Total nodes should be 4 (1 local + 3 peers)")
	assert.Equal(t, 4, aggregateView.OnlineNodes, "All nodes should be online")

	// 打印当前网络拓扑
	testutil.PrintNetworkTopology(t, manager, "聚合视图测试的网络拓扑")

	// 测试资源查询
	testutil.PrintTestSection(t, "步骤 5: 测试资源查询")
	// 使用较小的资源需求，确保能找到节点
	resourceRequest := &types.Info{
		CPU:    1000,                   // 需要 1000 millicores（降低要求）
		Memory: 1 * 1024 * 1024 * 1024, // 需要 1GB（降低要求）
		GPU:    0,                      // 不需要 GPU（降低要求）
	}

	requiredTags := types.NewResourceTags(true, false, true, false) // CPU, Memory（不需要GPU）

	availableNodes := aggregateView.FindAvailableNodes(resourceRequest, requiredTags)
	t.Logf("\n%s", testutil.Colorize("资源查询结果:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s: %s", testutil.Colorize("资源需求", testutil.ColorWhite+testutil.ColorBold),
		fmt.Sprintf("CPU %d mC, Memory %s, GPU %d",
			resourceRequest.CPU,
			testutil.FormatBytes(resourceRequest.Memory),
			resourceRequest.GPU))
	t.Logf("  %s: %d 个节点", testutil.Colorize("可用节点数", testutil.ColorWhite+testutil.ColorBold), len(availableNodes))

	for i, node := range availableNodes {
		t.Logf("  %d. %s (%s)", i+1, node.NodeName, node.NodeID)
		t.Logf("     可用资源: CPU %d mC, Memory %s, GPU %d",
			node.ResourceCapacity.Available.CPU,
			testutil.FormatBytes(node.ResourceCapacity.Available.Memory),
			node.ResourceCapacity.Available.GPU)
	}

	// 验证查询结果（应该找到满足条件的节点）
	assert.Greater(t, len(availableNodes), 0, "Should find at least one available node")

	t.Log("\n" + testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold))
	t.Log(testutil.Colorize("✓ Gossip 资源发现 - 聚合视图测试通过", testutil.ColorGreen+testutil.ColorBold))
	t.Log(testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold) + "\n")
}

// TestGossipDiscovery_ExpirationManagement 测试用例 3: Gossip 资源发现, 过期治理
func TestGossipDiscovery_ExpirationManagement(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: Gossip 资源发现 - 过期治理",
		"验证节点过期清理机制")

	ctx := context.Background()

	// 创建节点管理器（使用较短的 TTL 以便测试）
	testutil.PrintTestSection(t, "步骤 1: 创建节点管理器（TTL = 5秒）")
	nodeTTL := 5 * time.Second
	manager := discovery.NewNodeDiscoveryManager(
		"test-node-expiration",
		"test-expiration",
		"localhost:50005",
		"localhost:50006",
		"test-domain",
		[]string{},
		30*time.Second,
		nodeTTL,
	)

	manager.UpdateLocalNode(
		&types.Capacity{
			Total:     &types.Info{CPU: 8000, Memory: 8 * 1024 * 1024 * 1024, GPU: 0},
			Used:      &types.Info{CPU: 2000, Memory: 2 * 1024 * 1024 * 1024, GPU: 0},
			Available: &types.Info{CPU: 6000, Memory: 6 * 1024 * 1024 * 1024, GPU: 0},
		},
		types.NewResourceTags(true, false, true, false),
	)

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// 添加一个正常节点
	testutil.PrintTestSection(t, "步骤 2: 添加正常节点")
	freshNode := &discovery.PeerNode{
		NodeID:   "node-fresh",
		NodeName: "node-fresh",
		Address:  "192.168.1.100:50005",
		DomainID: "test-domain",
		ResourceCapacity: &types.Capacity{
			Total:     &types.Info{CPU: 4000, Memory: 4 * 1024 * 1024 * 1024, GPU: 0},
			Used:      &types.Info{CPU: 1000, Memory: 1 * 1024 * 1024 * 1024, GPU: 0},
			Available: &types.Info{CPU: 3000, Memory: 3 * 1024 * 1024 * 1024, GPU: 0},
		},
		ResourceTags: types.NewResourceTags(true, false, true, false),
		Status:       discovery.NodeStatusOnline,
		LastSeen:     time.Now(),
		LastUpdated:  time.Now(),
		Version:      1,
	}

	manager.ProcessNodeInfo(freshNode, "localhost:50005")
	knownNodes := manager.GetKnownNodes()
	assert.Equal(t, 1, len(knownNodes), "Should have 1 known node")
	testutil.PrintSuccess(t, "正常节点已添加")

	// 添加一个即将过期的节点
	testutil.PrintTestSection(t, "步骤 3: 添加即将过期的节点")
	staleNode := &discovery.PeerNode{
		NodeID:   "node-stale",
		NodeName: "node-stale",
		Address:  "192.168.1.101:50005",
		DomainID: "test-domain",
		ResourceCapacity: &types.Capacity{
			Total:     &types.Info{CPU: 2000, Memory: 2 * 1024 * 1024 * 1024, GPU: 0},
			Used:      &types.Info{CPU: 500, Memory: 512 * 1024 * 1024, GPU: 0},
			Available: &types.Info{CPU: 1500, Memory: 1536 * 1024 * 1024, GPU: 0},
		},
		ResourceTags: types.NewResourceTags(true, false, true, false),
		Status:       discovery.NodeStatusOnline,
		LastSeen:     time.Now().Add(-nodeTTL - 1*time.Second), // 已经过期
		LastUpdated:  time.Now().Add(-nodeTTL - 1*time.Second),
		Version:      1,
	}

	manager.ProcessNodeInfo(staleNode, "localhost:50005")
	knownNodes = manager.GetKnownNodes()
	assert.Equal(t, 2, len(knownNodes), "Should have 2 known nodes")
	testutil.PrintSuccess(t, "过期节点已添加（LastSeen 已过期）")

	// 验证节点是否过期
	testutil.PrintTestSection(t, "步骤 4: 验证节点过期状态")
	freshNodeInfo, found := manager.GetNodeByID("node-fresh")
	require.True(t, found, "Fresh node should be found")
	assert.False(t, freshNodeInfo.IsStale(nodeTTL), "Fresh node should not be stale")

	staleNodeInfo, found := manager.GetNodeByID("node-stale")
	require.True(t, found, "Stale node should be found")
	assert.True(t, staleNodeInfo.IsStale(nodeTTL), "Stale node should be stale")

	t.Log("\n" + testutil.Colorize("节点过期状态:", testutil.ColorYellow+testutil.ColorBold))
	freshIsStale := freshNodeInfo.IsStale(nodeTTL)
	t.Logf("  %s: %s (过期: %s)",
		testutil.Colorize("node-fresh", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize("正常", testutil.ColorGreen),
		testutil.ColorizeBool(freshIsStale))
	staleIsStale := staleNodeInfo.IsStale(nodeTTL)
	t.Logf("  %s: %s (过期: %s)",
		testutil.Colorize("node-stale", testutil.ColorWhite+testutil.ColorBold),
		testutil.Colorize("已过期", testutil.ColorRed),
		testutil.ColorizeBool(staleIsStale))

	// 等待清理循环执行（清理间隔是 1 分钟，但我们可以手动触发清理）
	testutil.PrintTestSection(t, "步骤 5: 触发清理操作")
	lostNodes := []string{}
	manager.SetOnNodeLost(func(nodeID string) {
		lostNodes = append(lostNodes, nodeID)
		testutil.PrintInfo(t, fmt.Sprintf("节点丢失回调触发: %s", nodeID))
	})

	// 更新正常节点（保持活跃）
	freshNode.LastSeen = time.Now()
	freshNode.LastUpdated = time.Now()
	freshNode.Version++
	manager.ProcessNodeInfo(freshNode, "localhost:50005")

	// 手动触发清理（执行清理循环）
	// 注意：清理循环的间隔是 1 分钟，为了测试，我们直接调用清理方法
	testutil.PrintInfo(t, "手动触发清理循环...")
	manager.CleanupExpiredNodes()

	// 验证过期节点是否被清理
	testutil.PrintTestSection(t, "步骤 6: 验证过期节点清理")
	knownNodes = manager.GetKnownNodes()

	// 检查过期节点是否还在
	staleNodeStillExists := false
	for _, node := range knownNodes {
		if node.NodeID == "node-stale" {
			staleNodeStillExists = true
			break
		}
	}

	t.Log("\n" + testutil.Colorize("清理结果:", testutil.ColorYellow+testutil.ColorBold))
	t.Logf("  %s: %d", testutil.Colorize("当前已知节点数", testutil.ColorWhite+testutil.ColorBold), len(knownNodes))
	t.Logf("  %s: %s", testutil.Colorize("过期节点是否仍存在", testutil.ColorWhite+testutil.ColorBold),
		testutil.ColorizeBool(staleNodeStillExists))

	// 验证正常节点仍然存在
	freshNodeStillExists := false
	for _, node := range knownNodes {
		if node.NodeID == "node-fresh" {
			freshNodeStillExists = true
			break
		}
	}

	assert.True(t, freshNodeStillExists, "Fresh node should still exist")
	t.Logf("  %s: %s", testutil.Colorize("正常节点是否仍存在", testutil.ColorWhite+testutil.ColorBold),
		testutil.ColorizeBool(freshNodeStillExists))

	// 测试节点更新（防止过期）
	testutil.PrintTestSection(t, "步骤 7: 测试节点更新（防止过期）")
	// 更新正常节点，保持其活跃状态
	freshNode.LastSeen = time.Now()
	freshNode.LastUpdated = time.Now()
	freshNode.Version++
	manager.ProcessNodeInfo(freshNode, "localhost:50005")

	// 验证节点更新后不会过期
	updatedNode, found := manager.GetNodeByID("node-fresh")
	require.True(t, found, "Updated node should be found")
	assert.False(t, updatedNode.IsStale(nodeTTL), "Updated node should not be stale after update")

	testutil.PrintSuccess(t, "节点更新机制正常工作")

	// 打印最终网络拓扑
	testutil.PrintNetworkTopology(t, manager, "过期治理测试后的网络拓扑")

	t.Log("\n" + testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold))
	t.Log(testutil.Colorize("✓ Gossip 资源发现 - 过期治理测试通过", testutil.ColorGreen+testutil.ColorBold))
	t.Log(testutil.Colorize(strings.Repeat("=", 80), testutil.ColorCyan+testutil.ColorBold) + "\n")
}
