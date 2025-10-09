package application

// GetComponent 获取指定ID的组件
func (dag *ApplicationDAG) GetComponent(id string) *Component {
	return dag.Components[id]
}

// DAG Node Messages
type ControlNode struct {
	Id           string            `protobuf:"bytes,1,opt,name=Id,proto3" json:"Id,omitempty"`
	Done         bool              `protobuf:"varint,2,opt,name=Done,proto3" json:"Done,omitempty"`
	FunctionName string            `protobuf:"bytes,3,opt,name=FunctionName,proto3" json:"FunctionName,omitempty"`
	Params       map[string]string `protobuf:"bytes,4,rep,name=Params,proto3" json:"Params,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"` // lambda_id -> parameter_name mapping
	Current      int32             `protobuf:"varint,5,opt,name=Current,proto3" json:"Current,omitempty"`
	DataNode     string            `protobuf:"bytes,6,opt,name=DataNode,proto3" json:"DataNode,omitempty"`         // id of the output data node
	PreDataNodes []string          `protobuf:"bytes,7,rep,name=PreDataNodes,proto3" json:"PreDataNodes,omitempty"` // ids of input data nodes
	FunctionType string            `protobuf:"bytes,8,opt,name=FunctionType,proto3" json:"FunctionType,omitempty"` // "remote" or "local"
}

func (x *ControlNode) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ControlNode) GetDone() bool {
	if x != nil {
		return x.Done
	}
	return false
}

func (x *ControlNode) GetFunctionName() string {
	if x != nil {
		return x.FunctionName
	}
	return ""
}

func (x *ControlNode) GetParams() map[string]string {
	if x != nil {
		return x.Params
	}
	return nil
}

func (x *ControlNode) GetCurrent() int32 {
	if x != nil {
		return x.Current
	}
	return 0
}

func (x *ControlNode) GetDataNode() string {
	if x != nil {
		return x.DataNode
	}
	return ""
}

func (x *ControlNode) GetPreDataNodes() []string {
	if x != nil {
		return x.PreDataNodes
	}
	return nil
}

func (x *ControlNode) GetFunctionType() string {
	if x != nil {
		return x.FunctionType
	}
	return ""
}

type DataNode struct {
	Id         string   `json:"id,omitempty"`
	Done       bool     `json:"done,omitempty"`
	Lambda     string   `json:"lambda,omitempty"` // lambda id
	Ready      bool     `json:"ready,omitempty"`
	ParentNode *string  `json:"parentNode,omitempty"` // id of parent data node (nullable)
	ChildNode  []string `json:"childNode,omitempty"`  // ids of child data nodes
}

func (x *DataNode) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *DataNode) GetDone() bool {
	if x != nil {
		return x.Done
	}
	return false
}

func (x *DataNode) GetLambda() string {
	if x != nil {
		return x.Lambda
	}
	return ""
}

func (x *DataNode) GetReady() bool {
	if x != nil {
		return x.Ready
	}
	return false
}

func (x *DataNode) GetParentNode() string {
	if x != nil && x.ParentNode != nil {
		return *x.ParentNode
	}
	return ""
}

func (x *DataNode) GetChildNode() []string {
	if x != nil {
		return x.ChildNode
	}
	return nil
}

type DAGNode struct {
	Type string `protobuf:"bytes,1,opt,name=Type,proto3" json:"Type,omitempty"` // "ControlNode" or "DataNode"
	// Types that are valid to be assigned to Node:
	//
	//	*DAGNode_ControlNode
	//	*DAGNode_DataNode
	Node isDAGNode_Node `protobuf_oneof:"Node"`
}

func (x *DAGNode) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *DAGNode) GetNode() isDAGNode_Node {
	if x != nil {
		return x.Node
	}
	return nil
}

func (x *DAGNode) GetControlNode() *ControlNode {
	if x != nil {
		if x, ok := x.Node.(*DAGNode_ControlNode); ok {
			return x.ControlNode
		}
	}
	return nil
}

func (x *DAGNode) GetDataNode() *DataNode {
	if x != nil {
		if x, ok := x.Node.(*DAGNode_DataNode); ok {
			return x.DataNode
		}
	}
	return nil
}

type isDAGNode_Node interface {
	isDAGNode_Node()
}

type DAGNode_ControlNode struct {
	ControlNode *ControlNode `protobuf:"bytes,2,opt,name=ControlNode,proto3,oneof"`
}

type DAGNode_DataNode struct {
	DataNode *DataNode `protobuf:"bytes,3,opt,name=DataNode,proto3,oneof"`
}

func (*DAGNode_ControlNode) isDAGNode_Node() {}

func (*DAGNode_DataNode) isDAGNode_Node() {}

type DAGEdge struct {
	FromNodeID string `json:"from_node_id,omitempty"`
	ToNodeID   string `json:"to_node_id,omitempty"`
	Info       string `json:"info,omitempty"`
}

type DAG struct {
	Nodes []*DAGNode `json:"nodes,omitempty"`
	Edges []*DAGEdge `json:"edges,omitempty"`
}

func (x *DAG) GetNodes() []*DAGNode {
	if x != nil {
		return x.Nodes
	}
	return nil
}
