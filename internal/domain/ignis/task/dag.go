package task

import (
	"github.com/9triver/iarnet/internal/domain/ignis/types"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
)

// DAGNodeID DAG 节点 ID
type DAGNodeID = string

// DAGNodeStatus DAG 节点状态
type DAGNodeStatus string

const (
	DAGNodeStatusPending DAGNodeStatus = "pending" // 等待执行
	DAGNodeStatusReady   DAGNodeStatus = "ready"   // 就绪，可以执行
	DAGNodeStatusRunning DAGNodeStatus = "running" // 执行中
	DAGNodeStatusDone    DAGNodeStatus = "done"    // 已完成
	DAGNodeStatusFailed  DAGNodeStatus = "failed"  // 执行失败
)

// DAGNodeType DAG 节点类型
type DAGNodeType string

const (
	DAGNodeTypeControl DAGNodeType = "control" // 控制节点（函数执行）
	DAGNodeTypeData    DAGNodeType = "data"    // 数据节点
)

// ControlNode DAG 控制节点（函数执行节点）
type ControlNode struct {
	ID           DAGNodeID           // 节点 ID
	FunctionName string              // 函数名称（对应 Function 的 name）
	Params       map[string]string   // lambda_id -> parameter_name 映射
	Current      int32               // 当前参数计数
	DataNode     DAGNodeID           // 输出数据节点 ID
	PreDataNodes []DAGNodeID         // 前置数据节点 IDs（依赖的数据节点）
	FunctionType string              // 函数类型："remote" 或 "local"
	Status       DAGNodeStatus       // 节点状态
	RuntimeID    types.RuntimeID     // 关联的 Function Runtime ID
	Result       *commonpb.ObjectRef // 执行结果（对象引用）
}

// Done 标记控制节点为已完成
func (c *ControlNode) Done(result *commonpb.ObjectRef) {
	c.Status = DAGNodeStatusDone
	c.Result = result
}

// DataNode DAG 数据节点
type DataNode struct {
	ID              DAGNodeID           // 节点 ID
	Lambda          string              // lambda id
	SufControlNodes []DAGNodeID         // 后续控制节点 IDs（依赖此数据节点的控制节点）
	PreControlNode  *DAGNodeID          // 前置控制节点 ID（产生此数据节点的控制节点，可为空）
	ParentNode      *DAGNodeID          // 父数据节点 ID（可为空）
	ChildNodes      []DAGNodeID         // 子数据节点 IDs
	Status          DAGNodeStatus       // 节点状态
	ObjectRef       *commonpb.ObjectRef // 数据对象的引用（当数据就绪时）
}

// Done 标记数据节点为已完成
func (d *DataNode) Done(objectRef *commonpb.ObjectRef) {
	d.Status = DAGNodeStatusDone
	d.ObjectRef = objectRef
}

// DAGNode DAG 节点（ControlNode 或 DataNode 的联合类型）
type DAGNode struct {
	Type        DAGNodeType
	ControlNode *ControlNode
	DataNode    *DataNode
}

// DAG DAG 图结构
type DAG struct {
	SessionID    types.SessionID            // 执行会话 ID
	ControlNodes map[DAGNodeID]*ControlNode // 控制节点映射
	DataNodes    map[DAGNodeID]*DataNode    // 数据节点映射
}

// NewDAG 创建新的 DAG
func NewDAG(sessionID types.SessionID) *DAG {
	return &DAG{
		SessionID:    sessionID,
		ControlNodes: make(map[DAGNodeID]*ControlNode),
		DataNodes:    make(map[DAGNodeID]*DataNode),
	}
}

// AddControlNode 添加控制节点
func (d *DAG) AddControlNode(node *ControlNode) {
	d.ControlNodes[node.ID] = node
}

// AddDataNode 添加数据节点
func (d *DAG) AddDataNode(node *DataNode) {
	d.DataNodes[node.ID] = node
}

// GetControlNode 获取控制节点
func (d *DAG) GetControlNode(id DAGNodeID) (*ControlNode, bool) {
	node, ok := d.ControlNodes[id]
	return node, ok
}

// GetDataNode 获取数据节点
func (d *DAG) GetDataNode(id DAGNodeID) (*DataNode, bool) {
	node, ok := d.DataNodes[id]
	return node, ok
}

// GetDataNodeByLambda 通过 lambda ID 查找数据节点
func (d *DAG) GetDataNodeByLambda(lambdaID string) (*DataNode, bool) {
	for _, node := range d.DataNodes {
		if node.Lambda == lambdaID {
			return node, true
		}
	}
	return nil, false
}
