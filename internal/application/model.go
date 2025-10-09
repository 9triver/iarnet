package application

// GetComponent 获取指定ID的组件
func (dag *ApplicationDAG) GetComponent(id string) *Component {
	return dag.Components[id]
}

// DAG Node Messages
type ControlNode struct {
	Id           string            `json:"id,omitempty"`
	Done         bool              `json:"done,omitempty"`
	FunctionName string            `json:"functionName,omitempty"`
	Params       map[string]string `json:"params,omitempty"` // lambda_id -> parameter_name mapping
	Current      int32             `json:"current,omitempty"`
}

func (x *ControlNode) isDAGNode_Node() {}

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

type DataNode struct {
	Id         string   `json:"id,omitempty"`
	Done       bool     `json:"done,omitempty"`
	Lambda     string   `json:"lambda,omitempty"` // lambda id
	Ready      bool     `json:"ready,omitempty"`
	ParentNode *string  `json:"parentNode,omitempty"` // id of parent data node (nullable)
	ChildNode  []string `json:"childNode,omitempty"`  // ids of child data nodes
}

func (x *DataNode) isDAGNode_Node() {}

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
	Type string `json:"type,omitempty"` // "ControlNode" or "DataNode"
	// Types that are valid to be assigned to Node:
	//
	//	*ControlNode
	//	*DataNode
	Node isDAGNode_Node `json:"node,omitempty"`
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
		if x, ok := x.Node.(*ControlNode); ok {
			return x
		}
	}
	return nil
}

func (x *DAGNode) GetDataNode() *DataNode {
	if x != nil {
		if x, ok := x.Node.(*DataNode); ok {
			return x
		}
	}
	return nil
}

type isDAGNode_Node interface {
	isDAGNode_Node()
}

type DAGEdge struct {
	FromNodeID string `json:"fromNodeId,omitempty"`
	ToNodeID   string `json:"toNodeId,omitempty"`
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
