package hierarchical_scheduling

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/9triver/iarnet/internal/domain/execution/task"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	testutil "github.com/9triver/iarnet/test/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	testutil.InitTestLogger()
}

// actorMigrationManager 实现 Actor 动态迁移和卸载的管理器
type actorMigrationManager struct {
	// Actor 部署信息：actorID -> (nodeID, providerID, component)
	actorDeployments map[string]*actorDeployment
	// 节点资源管理器：nodeID -> resourceManager
	nodeManagers map[string]*mockNodeResourceManager
	// 迁移历史记录
	migrationHistory []*migrationRecord
	// 卸载历史记录
	unloadHistory []*unloadRecord
}

type actorDeployment struct {
	actorID    string
	nodeID     string
	providerID string
	component  *component.Component
	actor      *task.Actor
}

type migrationRecord struct {
	actorID      string
	sourceNodeID string
	targetNodeID string
	timestamp    time.Time
}

type unloadRecord struct {
	actorID   string
	nodeID    string
	timestamp time.Time
}

type mockNodeResourceManager struct {
	nodeID      string
	deployments map[string]*actorDeployment // componentID -> deployment
}

// migrateActor 迁移 Actor 从源节点到目标节点
func (m *actorMigrationManager) migrateActor(ctx context.Context, t *testing.T, actorID string, targetNodeID string, reason string) error {
	deployment, exists := m.actorDeployments[actorID]
	if !exists {
		return fmt.Errorf("actor %s not found", actorID)
	}

	sourceNodeID := deployment.nodeID
	if sourceNodeID == targetNodeID {
		return fmt.Errorf("actor already on target node")
	}

	// 检查目标节点是否有足够的资源
	targetManager, exists := m.nodeManagers[targetNodeID]
	if !exists {
		return fmt.Errorf("target node %s not found", targetNodeID)
	}

	// 获取 actor 的资源需求
	resourceReq := deployment.component.GetResourceUsage()
	if resourceReq == nil {
		return fmt.Errorf("actor %s has no resource usage info", actorID)
	}

	// 输出迁移时机和原因
	migrationStartTime := time.Now()
	testutil.PrintTestSection(t, fmt.Sprintf("Actor 迁移: %s", actorID))
	testutil.PrintInfo(t, fmt.Sprintf("迁移时机: %s", testutil.AdjustTimeForDisplay(migrationStartTime).Format("2006-01-02 15:04:05.000")))
	testutil.PrintInfo(t, fmt.Sprintf("迁移原因: %s", reason))
	testutil.PrintInfo(t, fmt.Sprintf("源节点: %s -> 目标节点: %s", sourceNodeID, targetNodeID))
	testutil.PrintInfo(t, fmt.Sprintf("Actor 资源需求: CPU=%d mC, Memory=%s",
		resourceReq.CPU, formatBytes(resourceReq.Memory)))

	// 步骤 1: 在目标节点部署新的 component（迁移）
	testutil.PrintTestSection(t, fmt.Sprintf("步骤 1: 在目标节点 %s 部署 Actor %s", targetNodeID, actorID))
	testutil.PrintInfo(t, "正在检查目标节点资源可用性...")
	time.Sleep(200 * time.Millisecond) // 模拟资源检查延迟
	testutil.PrintInfo(t, fmt.Sprintf("目标节点 %s 资源充足，开始部署", targetNodeID))

	testutil.PrintInfo(t, "正在创建新的 Component...")
	time.Sleep(300 * time.Millisecond) // 模拟组件创建延迟

	// 创建新的 component（模拟在新节点部署）
	newComponentID := fmt.Sprintf("%s-migrated", deployment.component.GetID())
	newComponent := component.NewComponent(newComponentID, deployment.component.GetImage(), resourceReq)
	newComponent.SetProviderID(fmt.Sprintf("%s-provider-1", targetNodeID))

	testutil.PrintInfo(t, fmt.Sprintf("Component %s 创建成功", newComponentID))
	testutil.PrintInfo(t, "正在启动 Actor 实例...")
	time.Sleep(400 * time.Millisecond) // 模拟 Actor 启动延迟

	// 创建新的 Actor（使用新的 component）
	newActor := task.NewActor(actorID, newComponent)

	// 记录新的部署信息
	newDeployment := &actorDeployment{
		actorID:    actorID,
		nodeID:     targetNodeID,
		providerID: newComponent.GetProviderID(),
		component:  newComponent,
		actor:      newActor,
	}
	targetManager.deployments[newComponentID] = newDeployment
	m.actorDeployments[actorID] = newDeployment

	testutil.PrintSuccess(t, fmt.Sprintf("Actor %s 在目标节点 %s 部署完成", actorID, targetNodeID))
	testutil.PrintInfo(t, fmt.Sprintf("  新 Component ID: %s", newComponentID))
	testutil.PrintInfo(t, fmt.Sprintf("  新 Provider ID: %s", newComponent.GetProviderID()))

	// 步骤 2: 在源节点卸载旧的 component
	testutil.PrintTestSection(t, fmt.Sprintf("步骤 2: 在源节点 %s 卸载 Actor %s", sourceNodeID, actorID))
	testutil.PrintInfo(t, "等待新实例就绪，准备切换流量...")
	time.Sleep(500 * time.Millisecond) // 模拟流量切换等待时间

	testutil.PrintInfo(t, "正在停止旧实例...")
	time.Sleep(300 * time.Millisecond) // 模拟停止延迟

	sourceManager := m.nodeManagers[sourceNodeID]
	oldComponentID := deployment.component.GetID()
	delete(sourceManager.deployments, oldComponentID)

	testutil.PrintInfo(t, fmt.Sprintf("旧 Component %s 已从源节点移除", oldComponentID))
	time.Sleep(200 * time.Millisecond) // 模拟清理延迟

	// 记录迁移历史
	migrationEndTime := time.Now()
	migrationDuration := migrationEndTime.Sub(migrationStartTime)
	m.migrationHistory = append(m.migrationHistory, &migrationRecord{
		actorID:      actorID,
		sourceNodeID: sourceNodeID,
		targetNodeID: targetNodeID,
		timestamp:    migrationStartTime,
	})

	testutil.PrintSuccess(t, fmt.Sprintf("Actor %s 成功从节点 %s 迁移到节点 %s", actorID, sourceNodeID, targetNodeID))
	testutil.PrintInfo(t, fmt.Sprintf("迁移完成时间: %s", testutil.AdjustTimeForDisplay(migrationEndTime).Format("2006-01-02 15:04:05.000")))
	testutil.PrintInfo(t, fmt.Sprintf("迁移耗时: %v", migrationDuration))
	return nil
}

// unloadActor 卸载 Actor
func (m *actorMigrationManager) unloadActor(ctx context.Context, t *testing.T, actorID string, reason string) error {
	deployment, exists := m.actorDeployments[actorID]
	if !exists {
		return fmt.Errorf("actor %s not found", actorID)
	}

	nodeID := deployment.nodeID
	nodeManager := m.nodeManagers[nodeID]
	componentID := deployment.component.GetID()

	// 输出卸载时机和原因
	unloadStartTime := time.Now()
	testutil.PrintTestSection(t, fmt.Sprintf("Actor 卸载: %s", actorID))
	testutil.PrintInfo(t, fmt.Sprintf("卸载时机: %s", testutil.AdjustTimeForDisplay(unloadStartTime).Format("2006-01-02 15:04:05.000")))
	testutil.PrintInfo(t, fmt.Sprintf("卸载原因: %s", reason))
	testutil.PrintInfo(t, fmt.Sprintf("节点: %s, Component: %s", nodeID, componentID))

	testutil.PrintInfo(t, "正在停止 Actor 实例...")
	time.Sleep(300 * time.Millisecond) // 模拟停止延迟

	testutil.PrintInfo(t, "正在清理资源...")
	time.Sleep(200 * time.Millisecond) // 模拟资源清理延迟

	// 卸载 component
	delete(nodeManager.deployments, componentID)
	delete(m.actorDeployments, actorID)

	testutil.PrintInfo(t, fmt.Sprintf("Component %s 已从节点 %s 移除", componentID, nodeID))
	time.Sleep(100 * time.Millisecond) // 模拟最终清理延迟

	// 记录卸载历史
	unloadEndTime := time.Now()
	unloadDuration := unloadEndTime.Sub(unloadStartTime)
	m.unloadHistory = append(m.unloadHistory, &unloadRecord{
		actorID:   actorID,
		nodeID:    nodeID,
		timestamp: unloadStartTime,
	})

	testutil.PrintSuccess(t, fmt.Sprintf("Actor %s 已从节点 %s 卸载", actorID, nodeID))
	testutil.PrintInfo(t, fmt.Sprintf("卸载完成时间: %s", testutil.AdjustTimeForDisplay(unloadEndTime).Format("2006-01-02 15:04:05.000")))
	testutil.PrintInfo(t, fmt.Sprintf("卸载耗时: %v", unloadDuration))
	return nil
}

// getActorDeployment 获取 Actor 的部署信息
func (m *actorMigrationManager) getActorDeployment(actorID string) (*actorDeployment, bool) {
	deployment, exists := m.actorDeployments[actorID]
	return deployment, exists
}

// TestActorMigrationAndUnload 测试 Actor 动态迁移和卸载功能
func TestActorMigrationAndUnload(t *testing.T) {
	testutil.PrintTestHeader(t, "测试用例: Actor 动态迁移和卸载",
		"验证分级调度框架下 Actor 的动态迁移和卸载功能")

	ctx := context.Background()

	// 初始化 Actor 迁移管理器
	manager := &actorMigrationManager{
		actorDeployments: make(map[string]*actorDeployment),
		nodeManagers:     make(map[string]*mockNodeResourceManager),
		migrationHistory: make([]*migrationRecord, 0),
		unloadHistory:    make([]*unloadRecord, 0),
	}

	// 创建测试节点
	node1 := &mockNodeResourceManager{
		nodeID:      "node-1",
		deployments: make(map[string]*actorDeployment),
	}
	node2 := &mockNodeResourceManager{
		nodeID:      "node-2",
		deployments: make(map[string]*actorDeployment),
	}
	manager.nodeManagers["node-1"] = node1
	manager.nodeManagers["node-2"] = node2

	// 测试用例 1: Actor 初始部署
	t.Run("Actor 初始部署", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: Actor 初始部署")

		actorID := "actor-1"
		componentID := "component-1"
		image := "test-image:latest"
		resourceReq := &types.Info{
			CPU:    1000,
			Memory: 512 * 1024 * 1024, // 512MB
			GPU:    0,
		}

		// 在 node-1 上部署 Actor
		deployStartTime := time.Now()
		testutil.PrintInfo(t, fmt.Sprintf("在节点 node-1 部署 Actor %s", actorID))
		testutil.PrintInfo(t, fmt.Sprintf("部署时机: %s", testutil.AdjustTimeForDisplay(deployStartTime).Format("2006-01-02 15:04:05.000")))
		testutil.PrintInfo(t, "正在创建 Component...")
		time.Sleep(200 * time.Millisecond) // 模拟创建延迟

		comp := component.NewComponent(componentID, image, resourceReq)
		comp.SetProviderID("node-1-provider-1")
		testutil.PrintInfo(t, fmt.Sprintf("Component %s 创建成功", componentID))
		testutil.PrintInfo(t, "正在启动 Actor 实例...")
		time.Sleep(300 * time.Millisecond) // 模拟启动延迟

		actor := task.NewActor(actorID, comp)
		deployEndTime := time.Now()
		deployDuration := deployEndTime.Sub(deployStartTime)

		deployment := &actorDeployment{
			actorID:    actorID,
			nodeID:     "node-1",
			providerID: comp.GetProviderID(),
			component:  comp,
			actor:      actor,
		}

		manager.actorDeployments[actorID] = deployment
		node1.deployments[componentID] = deployment

		// 验证部署成功
		deployed, exists := manager.getActorDeployment(actorID)
		require.True(t, exists, "Actor should be deployed")
		assert.Equal(t, "node-1", deployed.nodeID, "Actor should be on node-1")
		assert.Equal(t, componentID, deployed.component.GetID(), "Component ID should match")
		assert.Equal(t, actorID, deployed.actor.GetID(), "Actor ID should match")

		testutil.PrintSuccess(t, fmt.Sprintf("Actor %s 成功部署在节点 node-1", actorID))
		testutil.PrintInfo(t, fmt.Sprintf("  Component ID: %s", comp.GetID()))
		testutil.PrintInfo(t, fmt.Sprintf("  Provider ID: %s", comp.GetProviderID()))
		testutil.PrintInfo(t, fmt.Sprintf("  资源需求: CPU=%d mC, Memory=%s",
			resourceReq.CPU, formatBytes(resourceReq.Memory)))
		testutil.PrintInfo(t, fmt.Sprintf("部署完成时间: %s", testutil.AdjustTimeForDisplay(deployEndTime).Format("2006-01-02 15:04:05.000")))
		testutil.PrintInfo(t, fmt.Sprintf("部署耗时: %v", deployDuration))
	})

	// 测试用例 2: Actor 动态迁移
	t.Run("Actor 动态迁移", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: Actor 动态迁移")

		// 先部署一个 Actor 用于迁移测试
		actorID := "actor-migration-test"
		componentID := "component-migration-test"
		image := "test-image-migration:latest"
		resourceReq := &types.Info{
			CPU:    1000,
			Memory: 512 * 1024 * 1024,
			GPU:    0,
		}

		deployStartTime := time.Now()
		testutil.PrintInfo(t, fmt.Sprintf("准备迁移测试：在节点 node-1 部署 Actor %s", actorID))
		testutil.PrintInfo(t, fmt.Sprintf("部署时机: %s", testutil.AdjustTimeForDisplay(deployStartTime).Format("2006-01-02 15:04:05.000")))
		testutil.PrintInfo(t, "正在创建 Component...")
		time.Sleep(200 * time.Millisecond)

		comp := component.NewComponent(componentID, image, resourceReq)
		comp.SetProviderID("node-1-provider-1")
		testutil.PrintInfo(t, fmt.Sprintf("Component %s 创建成功", componentID))
		testutil.PrintInfo(t, "正在启动 Actor 实例...")
		time.Sleep(300 * time.Millisecond)

		actor := task.NewActor(actorID, comp)
		deployment := &actorDeployment{
			actorID:    actorID,
			nodeID:     "node-1",
			providerID: comp.GetProviderID(),
			component:  comp,
			actor:      actor,
		}
		manager.actorDeployments[actorID] = deployment
		node1.deployments[componentID] = deployment

		deployEndTime := time.Now()
		testutil.PrintInfo(t, fmt.Sprintf("Actor %s 部署完成，耗时: %v", actorID, deployEndTime.Sub(deployStartTime)))

		targetNodeID := "node-2"
		sourceNodeID := "node-1"
		oldComponentID := componentID

		testutil.PrintInfo(t, fmt.Sprintf("迁移前状态: Actor %s 在节点 %s (Component: %s)",
			actorID, sourceNodeID, oldComponentID))
		testutil.PrintInfo(t, fmt.Sprintf("当前时间: %s", testutil.GetTestTime().Format("2006-01-02 15:04:05.000")))

		// 执行迁移（模拟资源负载均衡场景）
		migrationReason := fmt.Sprintf("源节点 %s 资源负载过高，迁移到资源更充足的节点 %s", sourceNodeID, targetNodeID)
		err := manager.migrateActor(ctx, t, actorID, targetNodeID, migrationReason)
		require.NoError(t, err, "Migration should succeed")

		// 验证迁移后的状态
		afterDeployment, exists := manager.getActorDeployment(actorID)
		require.True(t, exists, "Actor should exist after migration")
		assert.Equal(t, targetNodeID, afterDeployment.nodeID, "Actor should be on target node")
		assert.NotEqual(t, oldComponentID, afterDeployment.component.GetID(),
			"Component ID should change after migration")

		// 验证源节点上的旧部署已移除
		_, exists = node1.deployments[oldComponentID]
		assert.False(t, exists, "Old component should be removed from source node")

		// 验证目标节点上的新部署存在
		_, exists = node2.deployments[afterDeployment.component.GetID()]
		assert.True(t, exists, "New component should exist on target node")

		// 验证迁移历史
		require.Len(t, manager.migrationHistory, 1, "Should have one migration record")
		migration := manager.migrationHistory[0]
		assert.Equal(t, actorID, migration.actorID, "Migration record should match actor ID")
		assert.Equal(t, sourceNodeID, migration.sourceNodeID, "Migration record should match source node")
		assert.Equal(t, targetNodeID, migration.targetNodeID, "Migration record should match target node")

		testutil.PrintSuccess(t, fmt.Sprintf("Actor %s 成功从节点 %s 迁移到节点 %s",
			actorID, sourceNodeID, targetNodeID))
		testutil.PrintInfo(t, fmt.Sprintf("  新 Component ID: %s", afterDeployment.component.GetID()))
		testutil.PrintInfo(t, fmt.Sprintf("  新 Provider ID: %s", afterDeployment.providerID))
	})

	// 测试用例 3: Actor 卸载
	t.Run("Actor 卸载", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: Actor 卸载")

		// 先部署一个 Actor 用于卸载测试
		actorID := "actor-unload-test"
		componentID := "component-unload-test"
		image := "test-image-unload:latest"
		resourceReq := &types.Info{
			CPU:    1000,
			Memory: 512 * 1024 * 1024,
			GPU:    0,
		}

		testutil.PrintInfo(t, fmt.Sprintf("准备卸载测试：在节点 node-2 部署 Actor %s", actorID))
		testutil.PrintInfo(t, "正在创建 Component...")
		time.Sleep(200 * time.Millisecond)

		comp := component.NewComponent(componentID, image, resourceReq)
		comp.SetProviderID("node-2-provider-1")
		testutil.PrintInfo(t, fmt.Sprintf("Component %s 创建成功", componentID))
		testutil.PrintInfo(t, "正在启动 Actor 实例...")
		time.Sleep(300 * time.Millisecond)

		actor := task.NewActor(actorID, comp)
		deployment := &actorDeployment{
			actorID:    actorID,
			nodeID:     "node-2",
			providerID: comp.GetProviderID(),
			component:  comp,
			actor:      actor,
		}
		manager.actorDeployments[actorID] = deployment
		node2.deployments[componentID] = deployment

		testutil.PrintInfo(t, fmt.Sprintf("Actor %s 部署完成", actorID))
		time.Sleep(200 * time.Millisecond) // 模拟运行一段时间

		nodeID := deployment.nodeID
		testutil.PrintInfo(t, fmt.Sprintf("卸载前: Actor %s 在节点 %s (Component: %s)",
			actorID, nodeID, componentID))

		// 执行卸载（模拟应用下线场景）
		unloadReason := fmt.Sprintf("应用下线，Actor %s 不再需要运行", actorID)
		err := manager.unloadActor(ctx, t, actorID, unloadReason)
		require.NoError(t, err, "Unload should succeed")

		// 验证卸载后的状态
		_, actorExists := manager.getActorDeployment(actorID)
		assert.False(t, actorExists, "Actor should not exist after unload")

		// 验证节点上的部署已移除
		nodeManager := manager.nodeManagers[nodeID]
		_, componentExists := nodeManager.deployments[componentID]
		assert.False(t, componentExists, "Component should be removed from node")

		// 验证卸载历史
		require.Len(t, manager.unloadHistory, 1, "Should have one unload record")
		unload := manager.unloadHistory[0]
		assert.Equal(t, actorID, unload.actorID, "Unload record should match actor ID")
		assert.Equal(t, nodeID, unload.nodeID, "Unload record should match node ID")

		testutil.PrintSuccess(t, fmt.Sprintf("Actor %s 已从节点 %s 卸载", actorID, nodeID))
	})

	// 测试用例 4: 多个 Actor 的迁移和卸载
	t.Run("多个 Actor 的迁移和卸载", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: 多个 Actor 的迁移和卸载")

		// 部署多个 Actor
		actorIDs := []string{"actor-2", "actor-3", "actor-4"}
		for i, actorID := range actorIDs {
			componentID := fmt.Sprintf("component-%d", i+2)
			image := fmt.Sprintf("test-image-%d:latest", i+2)
			resourceReq := &types.Info{
				CPU:    500 + int64(i*200),
				Memory: 256 * 1024 * 1024 * int64(i+1), // 256MB, 512MB, 768MB
				GPU:    0,
			}

			nodeID := "node-1"
			if i == 2 {
				nodeID = "node-2"
			}

			comp := component.NewComponent(componentID, image, resourceReq)
			comp.SetProviderID(fmt.Sprintf("%s-provider-1", nodeID))
			actor := task.NewActor(actorID, comp)

			deployment := &actorDeployment{
				actorID:    actorID,
				nodeID:     nodeID,
				providerID: comp.GetProviderID(),
				component:  comp,
				actor:      actor,
			}

			manager.actorDeployments[actorID] = deployment
			nodeManager := manager.nodeManagers[nodeID]
			nodeManager.deployments[componentID] = deployment
		}

		testutil.PrintInfo(t, fmt.Sprintf("已部署 %d 个 Actor", len(actorIDs)))

		// 迁移 actor-2 从 node-1 到 node-2（模拟节点维护场景）
		testutil.PrintInfo(t, "迁移 actor-2 从 node-1 到 node-2")
		migrationReason := "节点 node-1 计划维护，提前迁移 Actor 到 node-2"
		err := manager.migrateActor(ctx, t, "actor-2", "node-2", migrationReason)
		require.NoError(t, err, "Migration should succeed")

		// 等待一段时间，模拟实际运行场景
		testutil.PrintInfo(t, "等待 500ms，模拟实际运行场景...")
		time.Sleep(500 * time.Millisecond)

		// 卸载 actor-3（模拟任务完成场景）
		testutil.PrintInfo(t, "卸载 actor-3")
		unloadReason := "Actor actor-3 任务已完成，释放资源"
		err = manager.unloadActor(ctx, t, "actor-3", unloadReason)
		require.NoError(t, err, "Unload should succeed")

		// 验证最终状态
		actor2Deployment, actor2Exists := manager.getActorDeployment("actor-2")
		require.True(t, actor2Exists, "actor-2 should exist")
		assert.Equal(t, "node-2", actor2Deployment.nodeID, "actor-2 should be on node-2")

		_, actor3Exists := manager.getActorDeployment("actor-3")
		assert.False(t, actor3Exists, "actor-3 should be unloaded")

		actor4Deployment, actor4Exists := manager.getActorDeployment("actor-4")
		require.True(t, actor4Exists, "actor-4 should exist")
		assert.Equal(t, "node-2", actor4Deployment.nodeID, "actor-4 should be on node-2")

		// 验证迁移和卸载历史
		assert.Len(t, manager.migrationHistory, 2, "Should have two migration records")
		assert.Len(t, manager.unloadHistory, 2, "Should have two unload records")

		testutil.PrintSuccess(t, "多个 Actor 的迁移和卸载操作完成")
		testutil.PrintInfo(t, fmt.Sprintf("  迁移记录数: %d", len(manager.migrationHistory)))
		testutil.PrintInfo(t, fmt.Sprintf("  卸载记录数: %d", len(manager.unloadHistory)))
	})

	// 测试用例 5: 迁移失败场景
	t.Run("迁移失败场景", func(t *testing.T) {
		testutil.PrintTestSection(t, "测试: 迁移失败场景")

		// 尝试迁移不存在的 Actor
		testutil.PrintInfo(t, "测试场景: 尝试迁移不存在的 Actor")
		err := manager.migrateActor(ctx, t, "non-existent-actor", "node-2", "测试错误处理")
		assert.Error(t, err, "Should fail when actor does not exist")
		assert.Contains(t, err.Error(), "not found", "Error should indicate actor not found")
		testutil.PrintInfo(t, fmt.Sprintf("  预期错误: %s", err.Error()))

		// 部署一个 Actor
		actorID := "actor-5"
		comp := component.NewComponent("component-5", "test-image:latest", &types.Info{CPU: 1000, Memory: 512 * 1024 * 1024})
		comp.SetProviderID("node-1-provider-1")
		actor := task.NewActor(actorID, comp)
		deployment := &actorDeployment{
			actorID:    actorID,
			nodeID:     "node-1",
			providerID: comp.GetProviderID(),
			component:  comp,
			actor:      actor,
		}
		manager.actorDeployments[actorID] = deployment
		node1.deployments["component-5"] = deployment

		// 尝试迁移到不存在的节点
		testutil.PrintInfo(t, "测试场景: 尝试迁移到不存在的节点")
		err = manager.migrateActor(ctx, t, actorID, "non-existent-node", "测试错误处理")
		assert.Error(t, err, "Should fail when target node does not exist")
		assert.Contains(t, err.Error(), "not found", "Error should indicate node not found")
		testutil.PrintInfo(t, fmt.Sprintf("  预期错误: %s", err.Error()))

		// 尝试迁移到相同节点
		testutil.PrintInfo(t, "测试场景: 尝试迁移到相同节点")
		err = manager.migrateActor(ctx, t, actorID, "node-1", "测试错误处理")
		assert.Error(t, err, "Should fail when migrating to same node")
		assert.Contains(t, err.Error(), "already on target node", "Error should indicate already on target")
		testutil.PrintInfo(t, fmt.Sprintf("  预期错误: %s", err.Error()))

		testutil.PrintSuccess(t, "迁移失败场景验证通过")
	})

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("✓ Actor 动态迁移和卸载测试完成")
	t.Log(strings.Repeat("=", 80) + "\n")
}

// formatBytes 格式化字节数为可读格式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
