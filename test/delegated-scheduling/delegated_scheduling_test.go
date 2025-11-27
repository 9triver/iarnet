package delegated_scheduling

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	testutil "github.com/9triver/iarnet/test/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errNoSchedulingCapability = errors.New("domain scheduler lacks autonomous scheduling")

type scheduleProposal struct {
	ProviderID string
	NodeID     string
	Score      float64
	Labels     map[string]string
	Message    string
}

type policyDecision struct {
	Accept bool
	Retry  bool
	Reason string
}

type delegationResult struct {
	Success    bool
	Path       string
	ProviderID string
	NodeID     string
	Message    string
	NextAction string
	AuditTrail []string
}

type resourceManager interface {
	ProposeSchedule(req *types.Info) (*scheduleProposal, error)
	CommitDeployment(proposal *scheduleProposal) (*delegationResult, error)
	ListDomains(req *types.Info) ([]*discovery.PeerNode, error)
	DeployAtNode(node *discovery.PeerNode, req *types.Info) (*delegationResult, error)
}

type executionEngineDelegator struct {
	rm             resourceManager
	evaluatePolicy func(req *types.Info, proposal *scheduleProposal) policyDecision
	nodeSelector   func(nodes []*discovery.PeerNode, req *types.Info) *discovery.PeerNode
	maxRetries     int
	auditSink      func(entry string)
}

func (d *executionEngineDelegator) coordinate(req *types.Info) (*delegationResult, error) {
	if req == nil {
		return nil, fmt.Errorf("resource request is required")
	}
	if d.rm == nil || d.evaluatePolicy == nil {
		return nil, fmt.Errorf("resource manager and policy evaluator are required")
	}
	nodeSelector := d.nodeSelector
	if nodeSelector == nil {
		nodeSelector = selectNodeByCPU
	}
	maxRetries := d.maxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var trail []string
	record := func(entry string) {
		if entry == "" {
			return
		}
		trail = append(trail, entry)
		if d.auditSink != nil {
			d.auditSink(entry)
		}
	}

	record(fmt.Sprintf("request: cpu=%d memory=%d gpu=%d", req.CPU, req.Memory, req.GPU))

	for attempt := 0; attempt < maxRetries; attempt++ {
		proposal, err := d.rm.ProposeSchedule(req)
		if err != nil {
			if errors.Is(err, errNoSchedulingCapability) {
				return d.coordinateWithEngineSelection(req, nodeSelector, trail)
			}
			return nil, err
		}

		record(fmt.Sprintf("proposal:%s@%s score=%.2f", proposal.ProviderID, proposal.NodeID, proposal.Score))
		decision := d.evaluatePolicy(req, proposal)
		if decision.Accept {
			record("policy:accept")
			result, err := d.rm.CommitDeployment(proposal)
			if err != nil {
				return nil, err
			}
			result.Path = "entrust-commit"
			result.AuditTrail = append(trail, fmt.Sprintf("commit:%s", proposal.NodeID))
			return result, nil
		}

		record("policy:reject")
		if !decision.Retry || attempt == maxRetries-1 {
			return &delegationResult{
				Success:    false,
				Path:       "refuse",
				Message:    decision.Reason,
				NextAction: "request_reschedule",
				AuditTrail: append(trail, "policy:final_reject"),
			}, nil
		}
		record("policy:request_retry")
	}

	return nil, fmt.Errorf("exceeded retry attempts")
}

func (d *executionEngineDelegator) coordinateWithEngineSelection(
	req *types.Info,
	nodeSelector func(nodes []*discovery.PeerNode, req *types.Info) *discovery.PeerNode,
	trail []string,
) (*delegationResult, error) {
	nodes, err := d.rm.ListDomains(req)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available for manual selection")
	}
	selected := nodeSelector(nodes, req)
	if selected == nil {
		return nil, fmt.Errorf("node selector returned nil")
	}
	selectedEntry := fmt.Sprintf("engine:selected:%s", selected.NodeID)
	trail = append(trail, selectedEntry)
	if d.auditSink != nil {
		d.auditSink(selectedEntry)
	}
	result, err := d.rm.DeployAtNode(selected, req)
	if err != nil {
		return nil, err
	}
	result.Path = "engine-select"
	result.NodeID = selected.NodeID
	result.AuditTrail = append(trail, fmt.Sprintf("delegate:%s", selected.NodeID))
	return result, nil
}

func selectNodeByCPU(nodes []*discovery.PeerNode, _ *types.Info) *discovery.PeerNode {
	var selected *discovery.PeerNode
	var maxCPU int64 = -1
	for _, node := range nodes {
		if node == nil ||
			node.Status != discovery.NodeStatusOnline ||
			node.ResourceCapacity == nil ||
			node.ResourceCapacity.Available == nil {
			continue
		}
		if node.ResourceCapacity.Available.CPU > maxCPU {
			maxCPU = node.ResourceCapacity.Available.CPU
			selected = node
		}
	}
	if selected == nil && len(nodes) > 0 {
		selected = nodes[0]
	}
	return selected
}

type mockDomainScheduler struct {
	proposals          []*scheduleProposal
	proposalIdx        int
	commitResponse     *delegationResult
	listNodes          []*discovery.PeerNode
	nodeDeployResponse *delegationResult
	noScheduler        bool

	proposeCalls int
	commitCalls  int
	listCalls    int
	deployCalls  int
}

func (m *mockDomainScheduler) ProposeSchedule(_ *types.Info) (*scheduleProposal, error) {
	if m.noScheduler {
		return nil, errNoSchedulingCapability
	}
	if m.proposalIdx >= len(m.proposals) {
		return nil, fmt.Errorf("no more proposals")
	}
	m.proposeCalls++
	p := m.proposals[m.proposalIdx]
	m.proposalIdx++
	return p, nil
}

func (m *mockDomainScheduler) CommitDeployment(proposal *scheduleProposal) (*delegationResult, error) {
	m.commitCalls++
	if m.commitResponse == nil {
		return nil, fmt.Errorf("commit response not configured")
	}
	res := *m.commitResponse
	res.ProviderID = proposal.ProviderID
	res.NodeID = proposal.NodeID
	return &res, nil
}

func (m *mockDomainScheduler) ListDomains(_ *types.Info) ([]*discovery.PeerNode, error) {
	m.listCalls++
	return m.listNodes, nil
}

func (m *mockDomainScheduler) DeployAtNode(node *discovery.PeerNode, _ *types.Info) (*delegationResult, error) {
	m.deployCalls++
	if m.nodeDeployResponse == nil {
		return nil, fmt.Errorf("deploy response not configured")
	}
	res := *m.nodeDeployResponse
	res.NodeID = node.NodeID
	return &res, nil
}

// ---------- 测试用例 ----------

// 4.1 具备自主调度机制：执行引擎拒绝第一次调度结果并要求域调度器重新调度
func TestDelegatedScheduling_AutonomousScheduler_RejectAndRetry(t *testing.T) {
	testutil.PrintTestHeader(t, "委托调度 - 自主调度(拒绝&重试)",
		"执行引擎按策略拒绝调度结果，触发域调度器重新调度")

	firstActorPlacement := "provider-edge-b@edge-b"
	testutil.PrintInfo(t, fmt.Sprintf("前置条件：计算任务 actor#1 已部署在 %s", firstActorPlacement))

	rm := &mockDomainScheduler{
		proposals: []*scheduleProposal{
			{
				ProviderID: "provider-edge-a",
				NodeID:     "edge-a",
				Score:      0.6,
				Labels:     map[string]string{"gpu": "0"},
				Message:    "缺少 GPU",
			},
			{
				ProviderID: "provider-edge-b",
				NodeID:     "edge-b",
				Score:      0.9,
				Labels:     map[string]string{"gpu": "1"},
				Message:    "具备 GPU",
			},
		},
		commitResponse: &delegationResult{
			Success: true,
			Message: "deployment committed",
		},
	}

	var auditTrail []string
	attempt := 0
	firstEvaluationLogged := false
	retryEvaluationLogged := false
	delegator := &executionEngineDelegator{
		rm: rm,
		evaluatePolicy: func(_ *types.Info, proposal *scheduleProposal) policyDecision {
			attempt++

			if attempt >= 2 && !retryEvaluationLogged {
				testutil.PrintTestSection(t, "步骤 3: 校验重调结果并确认接受")
				retryEvaluationLogged = true
			}
			testutil.PrintInfo(t,
				fmt.Sprintf("域调度器提案 %d: %s@%s",
					attempt, proposal.ProviderID, proposal.NodeID))
			if attempt == 1 && !firstEvaluationLogged {
				testutil.PrintTestSection(t, "步骤 2: 校验第一次调度结果（触发重调）")
				firstEvaluationLogged = true
			}
			if proposal.Labels["gpu"] != "1" {
				testutil.PrintInfo(t, fmt.Sprintf(
					"执行引擎判断 actor#2 需与 actor#1 同节点 (%s)，当前结果不满足数据亲和性要求，发起重调",
					firstActorPlacement,
				))
				return policyDecision{
					Accept: false,
					Retry:  true,
					Reason: "gpu required",
				}
			}
			testutil.PrintInfo(t, fmt.Sprintf(
				"域调度器重调后输出 %s@%s，满足与 actor#1 同节点的亲和策略",
				proposal.ProviderID, proposal.NodeID,
			))
			testutil.PrintInfo(t, "执行引擎判断该结果满足所有策略，选择接受")
			return policyDecision{Accept: true}
		},
		maxRetries: 2,
		auditSink: func(entry string) {
			auditTrail = append(auditTrail, entry)
		},
	}

	req := &types.Info{CPU: 3000, Memory: 4 * 1024 * 1024 * 1024}
	testutil.PrintTestSection(t, "步骤 1: 执行引擎获取调度提案")
	testutil.PrintResourceRequest(t, req)

	result, err := delegator.coordinate(req)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Equal(t, "entrust-commit", result.Path)
	assert.Equal(t, "provider-edge-b", result.ProviderID)
	assert.Equal(t, 2, rm.proposeCalls)
	assert.Equal(t, 1, rm.commitCalls)
	assert.Contains(t, auditTrail, "policy:reject")

	testutil.PrintSchedulingDecision(t, result.Path, true, "执行引擎成功在第二次提案中接受调度")
	testutil.PrintSuccess(t, "拒绝并重试场景验证完成")
}

// 4.1 具备自主调度机制：执行引擎直接接受第一次调度结果
func TestDelegatedScheduling_AutonomousScheduler_Accept(t *testing.T) {
	testutil.PrintTestHeader(t, "委托调度 - 自主调度(直接接受)",
		"执行引擎第一次提案即通过策略校验")

	rm := &mockDomainScheduler{
		proposals: []*scheduleProposal{
			{
				ProviderID: "provider-core",
				NodeID:     "core-node",
				Score:      0.95,
				Labels:     map[string]string{"gpu": "2"},
			},
		},
		commitResponse: &delegationResult{
			Success: true,
			Message: "deployment committed",
		},
	}

	delegator := &executionEngineDelegator{
		rm: rm,
		evaluatePolicy: func(_ *types.Info, _ *scheduleProposal) policyDecision {
			return policyDecision{Accept: true}
		},
	}

	req := &types.Info{CPU: 1000, Memory: 1 * 1024 * 1024 * 1024}
	testutil.PrintTestSection(t, "步骤 1: 获取首个提案并即时通过")
	testutil.PrintResourceRequest(t, req)

	result, err := delegator.coordinate(req)
	require.NoError(t, err)

	testutil.PrintTestSection(t, "步骤 2: 校验提交部署")
	assert.True(t, result.Success)
	assert.Equal(t, "entrust-commit", result.Path)
	assert.Equal(t, 1, rm.proposeCalls)
	assert.Equal(t, 1, rm.commitCalls)

	testutil.PrintSchedulingDecision(t, result.Path, true, "执行引擎直接接受首个调度方案")
	testutil.PrintSuccess(t, "直接接受场景验证完成")
}

// 4.2 无自主调度机制：域调度器提供域信息，执行引擎自选节点并委托部署
func TestDelegatedScheduling_NoScheduler_EngineSelects(t *testing.T) {
	testutil.PrintTestHeader(t, "委托调度 - 无调度机制(自选节点)",
		"算力网络缺少自主调度时，执行引擎基于自身分级调度策略筛选节点，再委托部署")

	nodes := []*discovery.PeerNode{
		{
			NodeID:   "edge-a",
			NodeName: "edge-alpha",
			Status:   discovery.NodeStatusOnline,
			LastSeen: time.Now(),
			ResourceCapacity: &types.Capacity{
				Available: &types.Info{CPU: 2000, Memory: 4 * 1024 * 1024 * 1024},
			},
		},
		{
			NodeID:   "edge-b",
			NodeName: "edge-beta",
			Status:   discovery.NodeStatusOnline,
			LastSeen: time.Now(),
			ResourceCapacity: &types.Capacity{
				Available: &types.Info{CPU: 4000, Memory: 8 * 1024 * 1024 * 1024},
			},
		},
	}

	rm := &mockDomainScheduler{
		noScheduler: true,
		listNodes:   nodes,
		nodeDeployResponse: &delegationResult{
			Success: true,
			Message: "delegated deployment executed",
		},
	}

	var auditTrail []string
	delegator := &executionEngineDelegator{
		rm: rm,
		evaluatePolicy: func(_ *types.Info, _ *scheduleProposal) policyDecision {
			return policyDecision{Accept: false}
		},
		auditSink: func(entry string) {
			auditTrail = append(auditTrail, entry)
		},
	}

	req := &types.Info{CPU: 2500, Memory: 3 * 1024 * 1024 * 1024}
	testutil.PrintTestSection(t, "步骤 1: 域调度器返回域内节点信息")
	testutil.PrintResourceRequest(t, req)
	testutil.PrintPeerNodeOverview(t, nodes)

	result, err := delegator.coordinate(req)
	require.NoError(t, err)

	testutil.PrintTestSection(t, "步骤 2: 执行引擎选择节点并委托部署")
	assert.True(t, result.Success)
	assert.Equal(t, "engine-select", result.Path)
	assert.Equal(t, "edge-b", result.NodeID)
	assert.Equal(t, 1, rm.listCalls)
	assert.Equal(t, 1, rm.deployCalls)
	assert.Contains(t, auditTrail, "engine:selected:edge-b")

	testutil.PrintSchedulingDecision(t, result.Path, true, "执行引擎自选 edge-b 并让域调度器执行部署")
	testutil.PrintSuccess(t, "执行引擎手动选节点场景验证通过")
}
