package hierarchical_scheduling

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/discovery"
	"github.com/9triver/iarnet/internal/domain/resource/scheduler"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	testutil "github.com/9triver/iarnet/test/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLocalResourceManager 实现 scheduler.Service 所需的本地资源管理接口
type fakeLocalResourceManager struct {
	deployCalls int
	component   *component.Component
	err         error
}

func (f *fakeLocalResourceManager) DeployComponent(
	ctx context.Context,
	runtimeEnv types.RuntimeEnv,
	resourceRequest *types.Info,
) (*component.Component, error) {
	f.deployCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.component, nil
}

func (f *fakeLocalResourceManager) DeployComponentOnProvider(
	ctx context.Context,
	runtimeEnv types.RuntimeEnv,
	resourceRequest *types.Info,
	providerID string,
) (*component.Component, error) {
	f.deployCalls++
	if f.err != nil {
		return nil, f.err
	}
	if f.component != nil {
		f.component.SetProviderID(providerID)
	}
	return f.component, nil
}

func (f *fakeLocalResourceManager) ScheduleLocalProvider(
	ctx context.Context,
	resourceRequest *types.Info,
) (*scheduler.LocalScheduleResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &scheduler.LocalScheduleResult{
		NodeID:     f.GetNodeID(),
		NodeName:   f.GetNodeName(),
		ProviderID: "local-provider-1",
		Available: &types.Info{
			CPU:    4000,
			Memory: 8 * 1024 * 1024 * 1024,
			GPU:    1,
		},
	}, nil
}

func (f *fakeLocalResourceManager) ListAllProviders(
	ctx context.Context,
	includeResources bool,
) ([]*scheduler.ProviderInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	// 返回一个空的 Provider 列表（测试中不需要）
	return []*scheduler.ProviderInfo{}, nil
}

func (f *fakeLocalResourceManager) GetNodeID() string {
	return "local-node-001"
}

func (f *fakeLocalResourceManager) GetNodeName() string {
	return "local-node"
}

// fakeDiscoveryService 实现远程节点发现服务
type fakeDiscoveryService struct {
	remoteNodes []*discovery.PeerNode
	queryCalls  int
}

func newFakeDiscoveryService(remoteNodes []*discovery.PeerNode) *fakeDiscoveryService {
	return &fakeDiscoveryService{remoteNodes: remoteNodes}
}

func (f *fakeDiscoveryService) Start(ctx context.Context) error { return nil }
func (f *fakeDiscoveryService) Stop()                           {}
func (f *fakeDiscoveryService) PerformGossip(ctx context.Context) error {
	return nil
}

func (f *fakeDiscoveryService) QueryResources(
	ctx context.Context,
	resourceRequest *types.Info,
	requiredTags *types.ResourceTags,
) ([]*discovery.PeerNode, error) {
	f.queryCalls++
	return f.remoteNodes, nil
}

func (f *fakeDiscoveryService) GetKnownNodes() []*discovery.PeerNode {
	return f.remoteNodes
}

func (f *fakeDiscoveryService) GetLocalNode() *discovery.PeerNode {
	return &discovery.PeerNode{
		NodeID:   "local-node-001",
		NodeName: "local-node",
	}
}

func (f *fakeDiscoveryService) UpdateLocalNode(resourceCapacity *types.Capacity, resourceTags interface{}) {
}

func buildRemoteNodes() []*discovery.PeerNode {
	return []*discovery.PeerNode{
		{
			NodeID:   "remote-node-001",
			NodeName: "edge-remote-a",
			Address:  "10.10.0.21:18090",
			Status:   discovery.NodeStatusOnline,
			ResourceCapacity: &types.Capacity{
				Total: &types.Info{
					CPU:    4000,
					Memory: 4 * 1024 * 1024 * 1024,
					GPU:    1,
				},
				Available: &types.Info{
					CPU:    3500,
					Memory: 3 * 1024 * 1024 * 1024,
					GPU:    1,
				},
			},
			ResourceTags: types.NewResourceTags(true, true, true, false),
			LastSeen:     time.Now(),
			LastUpdated:  time.Now(),
		},
		{
			NodeID:   "remote-node-002",
			NodeName: "edge-remote-b",
			Address:  "10.10.0.22:18090",
			Status:   discovery.NodeStatusOnline,
			ResourceCapacity: &types.Capacity{
				Total: &types.Info{
					CPU:    2000,
					Memory: 2 * 1024 * 1024 * 1024,
					GPU:    0,
				},
				Available: &types.Info{
					CPU:    1800,
					Memory: 1536 * 1024 * 1024,
					GPU:    0,
				},
			},
			ResourceTags: types.NewResourceTags(true, false, true, false),
			LastSeen:     time.Now(),
			LastUpdated:  time.Now(),
		},
	}
}

// TestCrossDomainScheduling_LocalPriority_SufficientResources
// 当本地资源充足时，调度应优先在本地完成部署
func TestCrossDomainScheduling_LocalPriority_SufficientResources(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: 本地优先调度 - 本地资源充足", "验证资源充足时优先选择本地节点")

	remoteNodes := buildRemoteNodes()
	discoverySvc := newFakeDiscoveryService(remoteNodes)

	testutil.PrintTestSection(t, "步骤 1: 构造域内/域外节点视图")
	testutil.PrintPeerNodeOverview(t, remoteNodes)

	localComponent := component.NewComponent(
		"comp-local",
		"example-image:latest",
		&types.Info{CPU: 1000, Memory: 512 * 1024 * 1024},
	)
	localComponent.SetProviderID("local-provider-1")

	localManager := &fakeLocalResourceManager{
		component: localComponent,
	}
	testutil.PrintInfo(t, "本地节点: local-node (local-node-001)")

	svc := scheduler.NewService(localManager, discoverySvc)

	req := &scheduler.DeployRequest{
		RuntimeEnv: types.RuntimeEnv("docker"),
		ResourceRequest: &types.Info{
			CPU:    500,
			Memory: 256 * 1024 * 1024,
			GPU:    0,
		},
	}

	testutil.PrintTestSection(t, "步骤 2: 提交调度请求")
	testutil.PrintResourceRequest(t, req.ResourceRequest)

	resp, err := svc.DeployComponent(context.Background(), req)
	require.NoError(t, err, "DeployComponent should not return error")
	require.NotNil(t, resp, "Response should not be nil")
	assert.True(t, resp.Success, "Deployment should succeed locally")
	require.NotNil(t, resp.Component, "Component should be returned")
	assert.Equal(t, "local-provider-1", resp.ProviderID, "Provider ID should be local provider")
	assert.Equal(t, 1, localManager.deployCalls, "Local manager should be invoked exactly once")
	assert.Equal(t, 0, discoverySvc.queryCalls, "本地资源充足时不应触发远程调度查询")

	testutil.PrintTestSection(t, "步骤 3: 验证调度结果")
	testutil.PrintSuccess(t, "本地资源充足时成功在本地完成部署")
	t.Logf("  节点: %s (%s)", resp.NodeName, resp.NodeID)
	t.Logf("  Provider: %s", resp.ProviderID)
	testutil.PrintSchedulingDecision(t, "local", true,
		fmt.Sprintf("命中本地节点 %s (%s)", resp.NodeName, resp.NodeID))
	testutil.PrintSuccess(t, "本地资源充足时优先本地部署测试通过")
}

// TestCrossDomainScheduling_LocalPriority_InsufficientResources
// 当本地资源不足时，需要将错误信息反馈给上层（由后续流程决定是否跨域）
func TestCrossDomainScheduling_LocalPriority_InsufficientResources(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: 本地优先调度 - 本地资源不足", "验证资源不足时的反馈机制")

	localErr := fmt.Errorf("failed to find available provider: cpu=4000")
	localManager := &fakeLocalResourceManager{
		err: localErr,
	}
	remoteNodes := buildRemoteNodes()
	discoverySvc := newFakeDiscoveryService(remoteNodes)

	testutil.PrintTestSection(t, "步骤 1: 构造本地资源耗尽场景")
	testutil.PrintPeerNodeOverview(t, remoteNodes)
	testutil.PrintInfo(t, fmt.Sprintf("预期本地调度失败错误: %s", localErr.Error()))

	svc := scheduler.NewService(localManager, discoverySvc)

	req := &scheduler.DeployRequest{
		RuntimeEnv: types.RuntimeEnv("docker"),
		ResourceRequest: &types.Info{
			CPU:    4000,
			Memory: 8 * 1024 * 1024 * 1024,
		},
	}

	testutil.PrintTestSection(t, "步骤 2: 提交调度请求")
	testutil.PrintResourceRequest(t, req.ResourceRequest)

	resp, err := svc.DeployComponent(context.Background(), req)
	require.NoError(t, err, "DeployComponent should not return transport error")
	require.NotNil(t, resp, "Response should not be nil")
	assert.False(t, resp.Success, "Deployment should fail when local resources are insufficient")
	assert.Equal(t, localErr.Error(), resp.Error, "Error message should reflect local resource shortage")
	assert.Equal(t, 1, localManager.deployCalls, "Local manager should still be invoked once")
	assert.Equal(t, 0, discoverySvc.queryCalls, "跨域流程由上层触发，本地失败阶段不应直接远程部署")

	testutil.PrintTestSection(t, "步骤 3: 验证反馈信息")
	testutil.PrintSuccess(t, "本地资源不足时能够正确反馈错误信息")
	t.Logf("  错误信息: %s", resp.Error)
	testutil.PrintSchedulingDecision(t, "local", false, resp.Error)
	testutil.PrintSuccess(t, "本地资源不足反馈机制测试通过")
}
