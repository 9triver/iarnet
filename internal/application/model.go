package application

// GetComponent 获取指定ID的组件
func (dag *ApplicationDAG) GetComponent(id string) *Component {
	return dag.Components[id]
}