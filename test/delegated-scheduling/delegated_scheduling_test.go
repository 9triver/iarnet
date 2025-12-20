package delegated_scheduling

import (
	"context"
	"fmt"
	"testing"

	"github.com/9triver/iarnet/internal/domain/resource/policy"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	testutil "github.com/9triver/iarnet/test/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDelegatedScheduling_SuccessfulDeployment 测试场景1：一次成功部署
// 展示：本地节点委托远程节点进行调度，远程节点返回调度结果，本地节点策略评估通过，确认部署成功
func TestDelegatedScheduling_SuccessfulDeployment(t *testing.T) {
	testutil.PrintTestHeader(t, "委托调度 - 一次成功部署",
		"展示两阶段提交流程：ProposeRemoteSchedule -> 策略评估 -> CommitRemoteSchedule")

	// Mock scheduler service
	mockScheduler := &mockSchedulerService{
		proposeResult: &scheduler.LocalScheduleResult{
			NodeID:     "remote-node-1",
			NodeName:   "远程节点1",
			ProviderID: "remote-provider-1",
			Available: &types.Info{
				CPU:    5000,                    // 满足 2000 * 1.2 = 2400 的安全裕度要求
				Memory: 10 * 1024 * 1024 * 1024, // 满足 4GB * 1.2 = 4.8GB 的要求
				GPU:    1,
			},
		},
		commitSuccess: true,
	}

	// 创建策略链（资源安全裕度策略）
	policyChain := policy.NewChain()
	safetyPolicy := &mockResourceSafetyMarginPolicy{
		cpuRatio:    1.2,
		memoryRatio: 1.2,
		gpuRatio:    1.0,
	}
	policyChain.AddPolicy(safetyPolicy)

	req := &types.Info{
		CPU:    2000,
		Memory: 4 * 1024 * 1024 * 1024,
		GPU:    1,
	}

	testutil.PrintTestSection(t, "步骤 1: 本地节点调用远程节点的 ProposeRemoteSchedule")
	testutil.PrintResourceRequest(t, req)
	testutil.PrintInfo(t, "本地节点通过 RPC 调用远程节点的 ProposeLocalSchedule，获取调度结果但不触发部署")

	// 第一阶段：调用远程节点获取调度结果
	remoteResult, err := mockScheduler.ProposeRemoteSchedule(
		context.Background(),
		"remote-node-1",
		"localhost:50051",
		req,
	)
	require.NoError(t, err)
	require.NotNil(t, remoteResult)

	testutil.PrintInfo(t, fmt.Sprintf("远程调度结果: NodeID=%s, ProviderID=%s, 可用CPU=%d, 可用内存=%d",
		remoteResult.NodeID, remoteResult.ProviderID,
		remoteResult.Available.CPU, remoteResult.Available.Memory))

	testutil.PrintTestSection(t, "步骤 2: 本地节点使用策略链评估远程调度结果")
	testutil.PrintInfo(t, "使用资源安全裕度策略评估调度结果...")

	// 构建策略评估上下文
	policyCtx := &policy.Context{
		NodeID:        remoteResult.NodeID,
		NodeName:      remoteResult.NodeName,
		ProviderID:    remoteResult.ProviderID,
		Available:     remoteResult.Available,
		Request:       req,
		LocalNodeID:   "local-node",
		LocalDomainID: "domain-1",
	}

	// 执行策略评估
	policyResult := policyChain.Evaluate(policyCtx)
	assert.Equal(t, policy.DecisionAccept, policyResult.Decision)
	testutil.PrintInfo(t, fmt.Sprintf("策略评估通过: %s", policyResult.Reason))

	testutil.PrintTestSection(t, "步骤 3: 本地节点调用远程节点的 CommitRemoteSchedule 确认部署")
	commitReq := &scheduler.CommitLocalScheduleRequest{
		RuntimeEnv:            types.RuntimeEnvPython,
		ResourceRequest:       req,
		ProviderID:            remoteResult.ProviderID,
		UpstreamZMQAddress:    "tcp://local:5555",
		UpstreamStoreAddress:  "tcp://local:6666",
		UpstreamLoggerAddress: "tcp://local:7777",
	}

	commitResp, err := mockScheduler.CommitRemoteSchedule(
		context.Background(),
		"remote-node-1",
		"localhost:50051",
		commitReq,
	)
	require.NoError(t, err)
	assert.True(t, commitResp.Success)
	assert.Equal(t, "remote-node-1", commitResp.NodeID)
	assert.Equal(t, "remote-provider-1", commitResp.ProviderID)

	testutil.PrintInfo(t, fmt.Sprintf("部署成功: NodeID=%s, ProviderID=%s",
		commitResp.NodeID, commitResp.ProviderID))
	testutil.PrintSuccess(t, "一次成功部署场景验证完成：两阶段提交流程成功执行")
}

// TestDelegatedScheduling_RejectAndRetry 测试场景2：拒绝并重新调度
// 展示：第一次调度结果被策略拒绝，重新调用 ProposeRemoteSchedule 获取新的调度结果，最终成功部署
func TestDelegatedScheduling_RejectAndRetry(t *testing.T) {
	testutil.PrintTestHeader(t, "委托调度 - 拒绝并重新调度",
		"展示策略拒绝第一次调度结果后，重新调度并最终成功部署的流程")

	// Mock scheduler service - 提供两个不同的调度结果
	proposalIndex := 0
	proposeResults := []*scheduler.LocalScheduleResult{
		// 第一次：资源不足，不满足安全裕度
		{
			NodeID:     "remote-node-1",
			NodeName:   "远程节点1",
			ProviderID: "remote-provider-1",
			Available: &types.Info{
				CPU:    2300,                   // 不满足 2000 * 1.2 = 2400 的要求
				Memory: 4 * 1024 * 1024 * 1024, // 不满足 4GB * 1.2 = 4.8GB 的要求（刚好不够）
				GPU:    1,
			},
		},
		// 第二次：资源充足，满足安全裕度
		{
			NodeID:     "remote-node-1",
			NodeName:   "远程节点1",
			ProviderID: "remote-provider-2",
			Available: &types.Info{
				CPU:    5000,                    // 满足要求
				Memory: 10 * 1024 * 1024 * 1024, // 满足要求
				GPU:    1,
			},
		},
	}

	mockScheduler := &mockSchedulerService{
		proposeResults: proposeResults,
		proposeFunc: func() *scheduler.LocalScheduleResult {
			if proposalIndex < len(proposeResults) {
				result := proposeResults[proposalIndex]
				proposalIndex++
				return result
			}
			return nil
		},
		commitSuccess: true,
	}

	// 创建策略链（资源安全裕度策略）
	policyChain := policy.NewChain()
	safetyPolicy := &mockResourceSafetyMarginPolicy{
		cpuRatio:    1.2,
		memoryRatio: 1.2,
		gpuRatio:    1.0,
	}
	policyChain.AddPolicy(safetyPolicy)

	req := &types.Info{
		CPU:    2000,
		Memory: 4 * 1024 * 1024 * 1024,
		GPU:    1,
	}

	maxRetries := 3
	var selectedResult *scheduler.LocalScheduleResult
	var commitResp *scheduler.DeployResponse

	testutil.PrintTestSection(t, "步骤 1: 第一次调用 ProposeRemoteSchedule")
	testutil.PrintResourceRequest(t, req)

	for attempt := 0; attempt < maxRetries; attempt++ {
		testutil.PrintInfo(t, fmt.Sprintf("尝试 %d/%d: 调用远程节点的 ProposeRemoteSchedule", attempt+1, maxRetries))

		// 调用远程节点获取调度结果
		result, err := mockScheduler.ProposeRemoteSchedule(
			context.Background(),
			"remote-node-1",
			"localhost:50051",
			req,
		)
		require.NoError(t, err)
		require.NotNil(t, result)

		testutil.PrintInfo(t, fmt.Sprintf("调度结果: ProviderID=%s, 可用CPU=%d, 可用内存=%d",
			result.ProviderID, result.Available.CPU, result.Available.Memory))

		testutil.PrintTestSection(t, fmt.Sprintf("步骤 %d: 策略评估", attempt+2))
		// 构建策略评估上下文
		policyCtx := &policy.Context{
			NodeID:        result.NodeID,
			NodeName:      result.NodeName,
			ProviderID:    result.ProviderID,
			Available:     result.Available,
			Request:       req,
			LocalNodeID:   "local-node",
			LocalDomainID: "domain-1",
		}

		// 执行策略评估
		policyResult := policyChain.Evaluate(policyCtx)

		if policyResult.Decision == policy.DecisionReject {
			testutil.PrintInfo(t, fmt.Sprintf("策略拒绝: [%s] %s", policyResult.Policy, policyResult.Reason))
			if attempt < maxRetries-1 {
				testutil.PrintInfo(t, "调度结果被策略拒绝，将重新调度...")
				continue
			} else {
				t.Fatalf("达到最大重试次数，仍然被策略拒绝")
			}
		}

		// 策略通过
		testutil.PrintInfo(t, fmt.Sprintf("策略评估通过: %s", policyResult.Reason))
		selectedResult = result
		break
	}

	require.NotNil(t, selectedResult, "应该找到一个满足策略的调度结果")

	testutil.PrintTestSection(t, "步骤 3: 确认部署")
	testutil.PrintInfo(t, fmt.Sprintf("使用 ProviderID=%s 进行部署", selectedResult.ProviderID))

	commitReq := &scheduler.CommitLocalScheduleRequest{
		RuntimeEnv:            types.RuntimeEnvPython,
		ResourceRequest:       req,
		ProviderID:            selectedResult.ProviderID,
		UpstreamZMQAddress:    "tcp://local:5555",
		UpstreamStoreAddress:  "tcp://local:6666",
		UpstreamLoggerAddress: "tcp://local:7777",
	}

	var err error
	commitResp, err = mockScheduler.CommitRemoteSchedule(
		context.Background(),
		"remote-node-1",
		"localhost:50051",
		commitReq,
	)
	require.NoError(t, err)
	assert.True(t, commitResp.Success)

	testutil.PrintInfo(t, fmt.Sprintf("部署成功: NodeID=%s, ProviderID=%s",
		commitResp.NodeID, commitResp.ProviderID))
	testutil.PrintSuccess(t, "拒绝并重新调度场景验证完成：第一次被拒绝，第二次成功部署")
}

// TestDelegatedScheduling_ListProvidersAndLocalDecision 测试场景3：直接获取列表并本地决策
// 展示：当远程节点无自主调度能力时，本地节点获取 Provider 列表，在本地进行调度决策，然后确认部署
func TestDelegatedScheduling_ListProvidersAndLocalDecision(t *testing.T) {
	testutil.PrintTestHeader(t, "委托调度 - 直接获取列表并本地决策",
		"展示无自主调度能力场景：获取 Provider 列表 -> 本地决策 -> 确认部署")

	// Mock scheduler service - 提供 Provider 列表
	mockScheduler := &mockSchedulerService{
		providerList: &scheduler.ProviderListResponse{
			Success:  true,
			NodeID:   "remote-node-1",
			NodeName: "远程节点1",
			Providers: []*scheduler.ProviderInfo{
				{
					ProviderID:   "remote-provider-1",
					ProviderName: "Provider 1",
					ProviderType: "docker",
					Status:       "connected",
					Available: &types.Info{
						CPU:    3000,
						Memory: 6 * 1024 * 1024 * 1024,
						GPU:    0,
					},
					TotalCapacity: &types.Info{
						CPU:    4000,
						Memory: 8 * 1024 * 1024 * 1024,
						GPU:    0,
					},
					Used: &types.Info{
						CPU:    1000,
						Memory: 2 * 1024 * 1024 * 1024,
						GPU:    0,
					},
					ResourceTags: types.NewResourceTags(true, false, true, false),
				},
				{
					ProviderID:   "remote-provider-2",
					ProviderName: "Provider 2",
					ProviderType: "docker",
					Status:       "connected",
					Available: &types.Info{
						CPU:    5000, // 资源更充足
						Memory: 10 * 1024 * 1024 * 1024,
						GPU:    1, // 有 GPU
					},
					TotalCapacity: &types.Info{
						CPU:    6000,
						Memory: 12 * 1024 * 1024 * 1024,
						GPU:    1,
					},
					Used: &types.Info{
						CPU:    1000,
						Memory: 2 * 1024 * 1024 * 1024,
						GPU:    0,
					},
					ResourceTags: types.NewResourceTags(true, true, true, false),
				},
			},
		},
		commitSuccess: true,
	}

	req := &types.Info{
		CPU:    2000,
		Memory: 4 * 1024 * 1024 * 1024,
		GPU:    1, // 需要 GPU
	}

	testutil.PrintTestSection(t, "步骤 1: 获取远程节点的 Provider 列表")
	testutil.PrintResourceRequest(t, req)
	testutil.PrintInfo(t, "远程节点无自主调度能力，调用 ListRemoteProviders 获取所有 Provider 列表")

	providerList, err := mockScheduler.ListRemoteProviders(
		context.Background(),
		"remote-node-1",
		"localhost:50051",
		true, // includeResources
	)
	require.NoError(t, err)
	require.NotNil(t, providerList)
	assert.True(t, providerList.Success)

	testutil.PrintInfo(t, fmt.Sprintf("获取到 %d 个 Provider", len(providerList.Providers)))
	for i, p := range providerList.Providers {
		testutil.PrintInfo(t, fmt.Sprintf("  Provider %d: ID=%s, 可用CPU=%d, 可用内存=%d, GPU=%d, 状态=%s",
			i+1, p.ProviderID, p.Available.CPU, p.Available.Memory, p.Available.GPU, p.Status))
	}

	testutil.PrintTestSection(t, "步骤 2: 本地节点进行调度决策")
	testutil.PrintInfo(t, "根据资源需求在本地选择最合适的 Provider...")

	// 本地调度决策：选择满足资源需求且资源最充足的 Provider
	var selectedProvider *scheduler.ProviderInfo
	for _, p := range providerList.Providers {
		if p.Status != "connected" {
			continue
		}
		// 检查是否满足资源需求
		if p.Available.CPU >= req.CPU &&
			p.Available.Memory >= req.Memory &&
			p.Available.GPU >= req.GPU {
			// 选择资源最充足的 Provider（按可用 CPU 排序）
			if selectedProvider == nil || p.Available.CPU > selectedProvider.Available.CPU {
				selectedProvider = p
			}
		}
	}

	require.NotNil(t, selectedProvider, "应该找到一个满足资源需求的 Provider")
	testutil.PrintInfo(t, fmt.Sprintf("选中 Provider: ID=%s, 可用CPU=%d, 可用内存=%d, GPU=%d",
		selectedProvider.ProviderID,
		selectedProvider.Available.CPU,
		selectedProvider.Available.Memory,
		selectedProvider.Available.GPU))

	testutil.PrintTestSection(t, "步骤 3: 确认部署")
	testutil.PrintInfo(t, fmt.Sprintf("使用选中的 ProviderID=%s 进行部署", selectedProvider.ProviderID))

	commitReq := &scheduler.CommitLocalScheduleRequest{
		RuntimeEnv:            types.RuntimeEnvPython,
		ResourceRequest:       req,
		ProviderID:            selectedProvider.ProviderID,
		UpstreamZMQAddress:    "tcp://local:5555",
		UpstreamStoreAddress:  "tcp://local:6666",
		UpstreamLoggerAddress: "tcp://local:7777",
	}

	commitResp, err := mockScheduler.CommitRemoteSchedule(
		context.Background(),
		"remote-node-1",
		"localhost:50051",
		commitReq,
	)
	require.NoError(t, err)
	assert.True(t, commitResp.Success)
	assert.Equal(t, selectedProvider.ProviderID, commitResp.ProviderID)

	testutil.PrintInfo(t, fmt.Sprintf("部署成功: NodeID=%s, ProviderID=%s",
		commitResp.NodeID, commitResp.ProviderID))
	testutil.PrintSuccess(t, "直接获取列表并本地决策场景验证完成：成功获取列表、本地决策并部署")
}

// Mock implementations

type mockSchedulerService struct {
	proposeResult  *scheduler.LocalScheduleResult
	proposeResults []*scheduler.LocalScheduleResult
	proposeFunc    func() *scheduler.LocalScheduleResult
	commitSuccess  bool
	commitError    error
	providerList   *scheduler.ProviderListResponse
}

func (m *mockSchedulerService) ProposeLocalSchedule(ctx context.Context, req *types.Info) (*scheduler.LocalScheduleResult, error) {
	if m.proposeFunc != nil {
		result := m.proposeFunc()
		if result == nil {
			return nil, fmt.Errorf("no more proposals")
		}
		return result, nil
	}
	if m.proposeResult == nil {
		return nil, fmt.Errorf("no proposal available")
	}
	return m.proposeResult, nil
}

func (m *mockSchedulerService) CommitLocalSchedule(ctx context.Context, req *scheduler.CommitLocalScheduleRequest) (*scheduler.DeployResponse, error) {
	if m.commitError != nil {
		return &scheduler.DeployResponse{
			Success: false,
			Error:   m.commitError.Error(),
		}, nil
	}

	if !m.commitSuccess {
		return &scheduler.DeployResponse{
			Success: false,
			Error:   "commit failed",
		}, nil
	}

	return &scheduler.DeployResponse{
		Success:    true,
		Component:  nil,
		NodeID:     "remote-node-1",
		NodeName:   "远程节点1",
		ProviderID: req.ProviderID,
	}, nil
}

func (m *mockSchedulerService) ProposeRemoteSchedule(ctx context.Context, targetNodeID string, targetAddress string, req *types.Info) (*scheduler.LocalScheduleResult, error) {
	return m.ProposeLocalSchedule(ctx, req)
}

func (m *mockSchedulerService) CommitRemoteSchedule(ctx context.Context, targetNodeID string, targetAddress string, req *scheduler.CommitLocalScheduleRequest) (*scheduler.DeployResponse, error) {
	return m.CommitLocalSchedule(ctx, req)
}

func (m *mockSchedulerService) ListRemoteProviders(ctx context.Context, targetNodeID string, targetAddress string, includeResources bool) (*scheduler.ProviderListResponse, error) {
	if m.providerList == nil {
		return &scheduler.ProviderListResponse{
			Success: false,
			Error:   "provider list not configured",
		}, nil
	}
	return m.providerList, nil
}

// Mock policies

type mockResourceSafetyMarginPolicy struct {
	cpuRatio    float64
	memoryRatio float64
	gpuRatio    float64
}

func (p *mockResourceSafetyMarginPolicy) Name() string {
	return "resource_safety_margin"
}

func (p *mockResourceSafetyMarginPolicy) Evaluate(ctx *policy.Context) policy.Result {
	if ctx.Available == nil || ctx.Request == nil {
		return policy.Result{
			Decision: policy.DecisionReject,
			Policy:   p.Name(),
			Reason:   "available or request is nil",
		}
	}

	// Check CPU
	if ctx.Available.CPU < int64(float64(ctx.Request.CPU)*p.cpuRatio) {
		return policy.Result{
			Decision: policy.DecisionReject,
			Policy:   p.Name(),
			Reason: fmt.Sprintf("CPU safety margin not met: available=%d, required=%d",
				ctx.Available.CPU, int64(float64(ctx.Request.CPU)*p.cpuRatio)),
		}
	}

	// Check Memory
	if ctx.Available.Memory < int64(float64(ctx.Request.Memory)*p.memoryRatio) {
		return policy.Result{
			Decision: policy.DecisionReject,
			Policy:   p.Name(),
			Reason: fmt.Sprintf("Memory safety margin not met: available=%d, required=%d",
				ctx.Available.Memory, int64(float64(ctx.Request.Memory)*p.memoryRatio)),
		}
	}

	// Check GPU
	if ctx.Available.GPU < int64(float64(ctx.Request.GPU)*p.gpuRatio) {
		return policy.Result{
			Decision: policy.DecisionReject,
			Policy:   p.Name(),
			Reason: fmt.Sprintf("GPU safety margin not met: available=%d, required=%d",
				ctx.Available.GPU, int64(float64(ctx.Request.GPU)*p.gpuRatio)),
		}
	}

	return policy.Result{
		Decision: policy.DecisionAccept,
		Policy:   p.Name(),
		Reason:   "all safety margins met",
	}
}
