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
	ID           DAGNodeID         // 节点 ID
	FunctionName string            // 函数名称（对应 Function 的 name）
	Params       map[string]string // lambda_id -> parameter_name 映射
	Current      int32             // 当前参数计数
	DataNode     DAGNodeID         // 输出数据节点 ID
	PreDataNodes []DAGNodeID       // 前置数据节点 IDs（依赖的数据节点）
	FunctionType string            // 函数类型："remote" 或 "local"
	Status       DAGNodeStatus     // 节点状态
	RuntimeID    types.RuntimeID   // 关联的 Function Runtime ID
}

// Start 启动控制节点
func (c *ControlNode) Start() {
	c.Status = DAGNodeStatusRunning
}

// Done 标记控制节点为已完成
func (c *ControlNode) Done() {
	c.Status = DAGNodeStatusDone
}

// Ready 标记控制节点为就绪
func (c *ControlNode) Ready() {
	c.Status = DAGNodeStatusReady
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

// Run 运行数据节点
func (d *DataNode) Start() {
	d.Status = DAGNodeStatusRunning
}

// DAG DAG 图结构
type DAG struct {
	SessionID    types.SessionID            // 执行会话 ID
	ControlNodes map[DAGNodeID]*ControlNode // 控制节点映射
	DataNodes    map[DAGNodeID]*DataNode    // 数据节点映射
	Edges        []*Edge                    // 已完成的边

	// data -> control 输入边的挂起列表（lambda 参数名）
	pendingInputEdgesByData    map[DAGNodeID][]DAGNodeID // 等待数据节点出现的边（value 为控制节点 ID）
	pendingInputEdgesByControl map[DAGNodeID][]DAGNodeID // 等待控制节点出现的边（value 为数据节点 ID）

	// control -> data 输出边的挂起列表
	pendingOutputEdgesByData    map[DAGNodeID][]DAGNodeID // 等待数据节点出现的边（value 为控制节点 ID）
	pendingOutputEdgesByControl map[DAGNodeID][]DAGNodeID // 等待控制节点出现的边（value 为数据节点 ID）
}

// Edge DAG 边关系
type Edge struct {
	From  DAGNodeID
	To    DAGNodeID
	Label string
}

// NewDAG 创建新的 DAG
func NewDAG(sessionID types.SessionID) *DAG {
	return &DAG{
		SessionID:                   sessionID,
		ControlNodes:                make(map[DAGNodeID]*ControlNode),
		DataNodes:                   make(map[DAGNodeID]*DataNode),
		Edges:                       make([]*Edge, 0),
		pendingInputEdgesByData:     make(map[DAGNodeID][]DAGNodeID),
		pendingInputEdgesByControl:  make(map[DAGNodeID][]DAGNodeID),
		pendingOutputEdgesByData:    make(map[DAGNodeID][]DAGNodeID),
		pendingOutputEdgesByControl: make(map[DAGNodeID][]DAGNodeID),
	}
}

// AddControlNode 添加控制节点
func (d *DAG) AddControlNode(node *ControlNode) {
	d.ControlNodes[node.ID] = node
	for _, dataID := range node.PreDataNodes {
		d.ensureDataControlEdge(dataID, node.ID)
	}
	if pendingDatas, ok := d.pendingInputEdgesByControl[node.ID]; ok {
		delete(d.pendingInputEdgesByControl, node.ID)
		for _, dataID := range pendingDatas {
			d.ensureDataControlEdge(dataID, node.ID)
		}
	}

	if node.DataNode != "" {
		d.ensureControlDataEdge(node.ID, node.DataNode)
	}
	if pendingOutputs, ok := d.pendingOutputEdgesByControl[node.ID]; ok {
		delete(d.pendingOutputEdgesByControl, node.ID)
		for _, dataID := range pendingOutputs {
			d.ensureControlDataEdge(node.ID, dataID)
		}
	}
}

// AddDataNode 添加数据节点
func (d *DAG) AddDataNode(node *DataNode) {
	d.DataNodes[node.ID] = node
	for _, controlID := range node.SufControlNodes {
		d.ensureDataControlEdge(node.ID, controlID)
	}
	if pendingControls, ok := d.pendingInputEdgesByData[node.ID]; ok {
		delete(d.pendingInputEdgesByData, node.ID)
		for _, controlID := range pendingControls {
			d.ensureDataControlEdge(node.ID, controlID)
		}
	}

	if node.PreControlNode != nil {
		d.ensureControlDataEdge(*node.PreControlNode, node.ID)
	}
	if pendingOutputs, ok := d.pendingOutputEdgesByData[node.ID]; ok {
		delete(d.pendingOutputEdgesByData, node.ID)
		for _, controlID := range pendingOutputs {
			d.ensureControlDataEdge(controlID, node.ID)
		}
	}
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

// GetEdge 获取边
func (d *DAG) GetEdges() []*Edge {
	return d.Edges
}

// ensureDataControlEdge 确保数据节点到控制节点的边被构建
func (d *DAG) ensureDataControlEdge(dataID, controlID DAGNodeID) {
	dataNode, dataOK := d.DataNodes[dataID]
	controlNode, controlOK := d.ControlNodes[controlID]

	if dataOK && controlOK {
		label := controlNode.Params[dataNode.Lambda]
		if label == "" {
			label = dataNode.Lambda
		}
		if label == "" {
			label = "input"
		}
		d.Edges = append(d.Edges, &Edge{
			From:  dataID,
			To:    controlID,
			Label: label,
		})
		return
	}

	if !dataOK {
		d.pendingInputEdgesByData[dataID] = appendUnique(d.pendingInputEdgesByData[dataID], controlID)
	}
	if !controlOK {
		d.pendingInputEdgesByControl[controlID] = appendUnique(d.pendingInputEdgesByControl[controlID], dataID)
	}
}

// ensureControlDataEdge 确保控制节点到数据节点的输出边被构建
func (d *DAG) ensureControlDataEdge(controlID, dataID DAGNodeID) {
	_, dataOK := d.DataNodes[dataID]
	_, controlOK := d.ControlNodes[controlID]

	if dataOK && controlOK {
		d.Edges = append(d.Edges, &Edge{
			From:  controlID,
			To:    dataID,
			Label: "",
		})
		return
	}

	if !dataOK {
		d.pendingOutputEdgesByData[dataID] = appendUnique(d.pendingOutputEdgesByData[dataID], controlID)
	}
	if !controlOK {
		d.pendingOutputEdgesByControl[controlID] = appendUnique(d.pendingOutputEdgesByControl[controlID], dataID)
	}
}

func appendUnique(list []DAGNodeID, value DAGNodeID) []DAGNodeID {
	for _, v := range list {
		if v == value {
			return list
		}
	}
	return append(list, value)
}
