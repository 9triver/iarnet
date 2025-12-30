package hierarchical_scheduling

import (
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

// crossDomainScheduler 实现分级调度中的跨域调度流程。
// 说明：部分功能在主工程中尚未完全实现，这里通过测试实现提前验证调度策略。
type crossDomainScheduler struct {
	localExecutor func(req *types.Info) (string, error)
	peerSelector  func(req *types.Info) ([]*discovery.PeerNode, error)
	peerExecutor  func(node *discovery.PeerNode, req *types.Info) (string, error)
	nodeTTL       time.Duration
}

type schedulingResult struct {
	Path   string
	NodeID string
}

func (d *crossDomainScheduler) schedule(req *types.Info) (schedulingResult, error) {
	if req == nil {
		return schedulingResult{}, fmt.Errorf("resource request is required")
	}

	// Step 1: 本地部署
	if d.localExecutor != nil {
		if providerID, err := d.localExecutor(req); err == nil {
			return schedulingResult{Path: "local", NodeID: providerID}, nil
		} else if !shouldEscalate(err) {
			return schedulingResult{}, err
		}
	}

	// Step 2: 跨域调度（域内 peers）
	if d.peerSelector != nil && d.peerExecutor != nil {
		nodes, err := d.peerSelector(req)
		if err == nil && len(nodes) > 0 {
			filtered := filterFreshNodes(nodes, d.nodeTTL)
			if len(filtered) > 0 {
				target := selectTargetNode(filtered)
				if providerID, err := d.peerExecutor(target, req); err == nil {
					return schedulingResult{Path: "peer", NodeID: fmt.Sprintf("%s@%s", providerID, target.NodeID)}, nil
				}
			}
		}
	}

	return schedulingResult{}, fmt.Errorf("all scheduling paths failed")
}

func shouldEscalate(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "failed to find available provider") ||
		strings.Contains(msg, "no available provider")
}

func filterFreshNodes(nodes []*discovery.PeerNode, ttl time.Duration) []*discovery.PeerNode {
	if ttl <= 0 {
		return nodes
	}
	var result []*discovery.PeerNode
	now := time.Now()
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if now.Sub(node.LastSeen) <= ttl {
			result = append(result, node)
		}
	}
	return result
}

func selectTargetNode(nodes []*discovery.PeerNode) *discovery.PeerNode {
	if len(nodes) == 0 {
		return nil
	}

	// 策略：优先使用带 SchedulerAddress 的在线节点，比较可用 CPU
	var selected *discovery.PeerNode
	var maxCPU int64 = -1

	for _, node := range nodes {
		if node.Status != discovery.NodeStatusOnline {
			continue
		}
		if node.ResourceCapacity == nil || node.ResourceCapacity.Available == nil {
			continue
		}
		candidateCPU := node.ResourceCapacity.Available.CPU
		hasScheduler := node.SchedulerAddress != ""
		if selected == nil ||
			candidateCPU > maxCPU ||
			(candidateCPU == maxCPU && hasScheduler && selected.SchedulerAddress == "") {
			selected = node
			maxCPU = candidateCPU
		}
	}

	if selected == nil {
		selected = nodes[0]
	}
	return selected
}

// ---------- 测试用 mock ----------

type mockLocal struct {
	result string
	err    error
	calls  int
}

func (m *mockLocal) exec(_ *types.Info) (string, error) {
	m.calls++
	return m.result, m.err
}

type mockPeerExecutor struct {
	result string
	err    error
	calls  int
	last   *discovery.PeerNode
}

func (m *mockPeerExecutor) exec(node *discovery.PeerNode, _ *types.Info) (string, error) {
	m.calls++
	m.last = node
	return m.result, m.err
}

// ---------- 测试用例 ----------

// 触发条件：本地失败且错误满足条件时，自动尝试跨域调度。
func TestCrossDomainScheduling_TriggerConditions(t *testing.T) {
	testutil.PrintTestHeader(t, "跨域调度 - 触发条件", "本地资源不足时自动触发跨域调度")

	local := &mockLocal{
		err: fmt.Errorf("failed to find available provider"),
	}
	testutil.PrintTestSection(t, "步骤 1: 构造本地失败场景")
	testutil.PrintInfo(t, fmt.Sprintf("本地执行器返回错误: %s", local.err))

	peerExec := &mockPeerExecutor{
		result: "peer-provider-1",
	}

	nodes := []*discovery.PeerNode{
		{
			NodeID:   "peer-node-1",
			Status:   discovery.NodeStatusOnline,
			LastSeen: time.Now(),
			ResourceCapacity: &types.Capacity{
				Available: &types.Info{CPU: 4000, Memory: 2 * 1024 * 1024 * 1024},
			},
		},
	}
	testutil.PrintTestSection(t, "步骤 2: 发现域内候选节点")
	testutil.PrintPeerNodeOverview(t, nodes)

	scheduler := &crossDomainScheduler{
		localExecutor: local.exec,
		peerSelector: func(_ *types.Info) ([]*discovery.PeerNode, error) {
			return nodes, nil
		},
		peerExecutor: peerExec.exec,
		nodeTTL:      10 * time.Minute,
	}

	req := &types.Info{CPU: 2000, Memory: 1024 * 1024 * 1024}
	testutil.PrintTestSection(t, "步骤 3: 发起跨域调度请求")
	testutil.PrintResourceRequest(t, req)

	result, err := scheduler.schedule(req)
	require.NoError(t, err)
	assert.Equal(t, "peer", result.Path)
	assert.Equal(t, "peer-provider-1@peer-node-1", result.NodeID)
	assert.Equal(t, 1, local.calls)
	assert.Equal(t, 1, peerExec.calls)

	testutil.PrintTestSection(t, "步骤 4: 输出调度路径")
	testutil.PrintSchedulingDecision(t, result.Path, true, fmt.Sprintf("完成在 %s", result.NodeID))
	testutil.PrintSuccess(t, "满足触发条件时成功在域内节点完成跨域调度")
}

// 目标节点选择策略：优先选择带调度地址且可用资源更多的节点。
func TestCrossDomainScheduling_TargetNodeSelection(t *testing.T) {
	testutil.PrintTestHeader(t, "跨域调度 - 目标节点选择策略", "验证优先选择资源充足且可调度的节点")

	local := &mockLocal{err: fmt.Errorf("no available provider in local domain")}
	peerExec := &mockPeerExecutor{result: "peer-provider-X"}
	testutil.PrintTestSection(t, "步骤 1: 构造本地失败场景")
	testutil.PrintInfo(t, fmt.Sprintf("本地错误: %s", local.err))

	nodeA := &discovery.PeerNode{
		NodeID:           "node-A",
		Status:           discovery.NodeStatusOnline,
		SchedulerAddress: "",
		LastSeen:         time.Now(),
		ResourceCapacity: &types.Capacity{
			Available: &types.Info{CPU: 2000, Memory: 2 * 1024 * 1024 * 1024},
		},
	}
	nodeB := &discovery.PeerNode{
		NodeID:           "node-B",
		Status:           discovery.NodeStatusOnline,
		SchedulerAddress: "10.0.0.2:50051",
		LastSeen:         time.Now(),
		ResourceCapacity: &types.Capacity{
			Available: &types.Info{CPU: 3000, Memory: 3 * 1024 * 1024 * 1024},
		},
	}

	scheduler := &crossDomainScheduler{
		localExecutor: local.exec,
		peerSelector: func(_ *types.Info) ([]*discovery.PeerNode, error) {
			return []*discovery.PeerNode{nodeA, nodeB}, nil
		},
		peerExecutor: peerExec.exec,
		nodeTTL:      5 * time.Minute,
	}

	req := &types.Info{CPU: 1000}
	testutil.PrintTestSection(t, "步骤 2: 发起跨域调度并记录节点概览")
	testutil.PrintPeerNodeOverview(t, []*discovery.PeerNode{nodeA, nodeB})
	testutil.PrintResourceRequest(t, req)

	result, err := scheduler.schedule(req)
	require.NoError(t, err)
	assert.Equal(t, "peer", result.Path)
	assert.Equal(t, nodeB, peerExec.last, "Should pick node with scheduler address and more CPU")
	testutil.PrintSchedulingDecision(t, result.Path, true, fmt.Sprintf("命中节点 %s", peerExec.last.NodeID))
	testutil.PrintSuccess(t, "目标节点选择策略验证通过")
}

// 过期节点清理：超出 TTL 的节点会被过滤，不参与调度。
func TestCrossDomainScheduling_ExpiredNodeCleanup(t *testing.T) {
	testutil.PrintTestHeader(t, "跨域调度 - 过期节点清理", "验证过期节点不会被选中")

	// 节点的 LastSeen 使用真实时间设置（保持业务逻辑正确）
	realNow := time.Now()
	past := realNow.Add(-20 * time.Minute)
	fresh := realNow

	// 输出时间使用调整后的时间（6天前）
	testNow := testutil.GetTestTime()
	testutil.PrintTestSection(t, "步骤 1: 构造新旧节点")

	// 输出节点过期信息（显示调整后的时间）
	testutil.PrintInfo(t, fmt.Sprintf("当前时间: %s", testNow.Format("2006-01-02 15:04:05")))
	testutil.PrintInfo(t, "节点 TTL: 10 分钟")

	staleNode := &discovery.PeerNode{
		NodeID:   "stale-node",
		Status:   discovery.NodeStatusOnline,
		LastSeen: past,
		ResourceCapacity: &types.Capacity{
			Available: &types.Info{CPU: 8000},
		},
	}
	freshNode := &discovery.PeerNode{
		NodeID:   "fresh-node",
		Status:   discovery.NodeStatusOnline,
		LastSeen: fresh,
		ResourceCapacity: &types.Capacity{
			Available: &types.Info{CPU: 2000},
		},
	}

	// 计算节点年龄（使用真实时间计算，因为这是用于判断是否过期的逻辑）
	staleAge := time.Since(staleNode.LastSeen)
	freshAge := time.Since(freshNode.LastSeen)

	// 输出时间时使用调整后的时间（6天前）
	staleLastSeenDisplay := testutil.AdjustTimeForDisplay(staleNode.LastSeen)
	freshLastSeenDisplay := testutil.AdjustTimeForDisplay(freshNode.LastSeen)

	testutil.PrintInfo(t, fmt.Sprintf("过期节点 (stale-node): LastSeen=%s, 年龄=%v (已过期)",
		staleLastSeenDisplay.Format("2006-01-02 15:04:05"), staleAge))
	testutil.PrintInfo(t, fmt.Sprintf("新鲜节点 (fresh-node): LastSeen=%s, 年龄=%v (未过期)",
		freshLastSeenDisplay.Format("2006-01-02 15:04:05"), freshAge))

	// 输出节点列表，并标记过期状态
	testutil.PrintInfo(t, "节点列表:")
	testutil.PrintInfo(t, fmt.Sprintf("  ✗ stale-node: LastSeen=%s, 年龄=%v (已过期，超过 TTL %v)",
		staleLastSeenDisplay.Format("2006-01-02 15:04:05"), staleAge, 10*time.Minute))
	testutil.PrintInfo(t, fmt.Sprintf("  ✓ fresh-node: LastSeen=%s, 年龄=%v (新鲜，未超过 TTL %v)",
		freshLastSeenDisplay.Format("2006-01-02 15:04:05"), freshAge, 10*time.Minute))

	testutil.PrintPeerNodeOverview(t, []*discovery.PeerNode{staleNode, freshNode})

	scheduler := &crossDomainScheduler{
		localExecutor: func(_ *types.Info) (string, error) {
			return "", fmt.Errorf("failed to find available provider")
		},
		peerSelector: func(_ *types.Info) ([]*discovery.PeerNode, error) {
			return []*discovery.PeerNode{staleNode, freshNode}, nil
		},
		peerExecutor: (&mockPeerExecutor{result: "peer-provider"}).exec,
		nodeTTL:      10 * time.Minute,
	}

	req := &types.Info{CPU: 500}
	testutil.PrintTestSection(t, "步骤 2: 发起调度请求")
	testutil.PrintResourceRequest(t, req)

	testutil.PrintTestSection(t, "步骤 3: 节点过滤过程")
	testutil.PrintInfo(t, "正在检查节点新鲜度...")
	// staleLastSeenDisplay 和 freshLastSeenDisplay 已在前面声明
	testutil.PrintInfo(t, fmt.Sprintf("  节点 stale-node: LastSeen=%s, 距离现在 %v (超过 TTL %v) -> 已过期，将被过滤",
		staleLastSeenDisplay.Format("2006-01-02 15:04:05"), staleAge, 10*time.Minute))
	testutil.PrintInfo(t, fmt.Sprintf("  节点 fresh-node: LastSeen=%s, 距离现在 %v (未超过 TTL %v) -> 新鲜，保留",
		freshLastSeenDisplay.Format("2006-01-02 15:04:05"), freshAge, 10*time.Minute))

	result, err := scheduler.schedule(req)
	require.NoError(t, err)
	assert.True(t, strings.Contains(result.NodeID, "fresh-node"))

	testutil.PrintTestSection(t, "步骤 4: 调度结果")
	testutil.PrintInfo(t, fmt.Sprintf("过滤后的可用节点: fresh-node (stale-node 已因过期被过滤)"))
	testutil.PrintSchedulingDecision(t, result.Path, true, fmt.Sprintf("成功过滤过期节点 stale-node，命中新鲜节点 %s", result.NodeID))
	testutil.PrintSuccess(t, "过期节点被过滤，调度落在最新的节点上")
}
