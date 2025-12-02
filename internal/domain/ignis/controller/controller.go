package controller

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/9triver/iarnet/internal/domain/ignis/task"
	"github.com/9triver/iarnet/internal/domain/ignis/types"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	"github.com/9triver/iarnet/internal/domain/resource/store"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	"github.com/sirupsen/logrus"
)

type Controller struct {
	appID            string
	events           *EventHub
	toClientChan     chan *ctrlpb.Message
	componentService component.Service
	storeService     store.Service
	functions        map[string]*task.Function // functionName -> Function
	dags             map[string]*task.DAG      // sessionID -> DAG
}

func NewController(componentService component.Service, storeService store.Service, appID string) *Controller {
	return &Controller{
		appID:            appID,
		componentService: componentService,
		storeService:     storeService,
		functions:        make(map[string]*task.Function),
		dags:             make(map[string]*task.DAG),
	}
}

func (c *Controller) GetDAGs() map[string]*task.DAG {
	return c.dags
}

func (c *Controller) SetToClientChan(toClientChan chan *ctrlpb.Message) {
	c.toClientChan = toClientChan
}

func (c *Controller) ClearToClientChan() {
	c.toClientChan = nil
}

func (c *Controller) SetEvents(events *EventHub) {
	c.events = events
}

func (c *Controller) GetToClientChan() chan *ctrlpb.Message {
	return c.toClientChan
}

func (c *Controller) AppID() string { return c.appID }

// HandleActorMessage 处理 Actor 消息
func (c *Controller) HandleActorMessage(ctx context.Context, msg *actorpb.Message) error {
	switch m := msg.GetMessage().(type) {
	case *actorpb.Message_InvokeResponse:
		return c.handleInvokeResponse(ctx, m.InvokeResponse)
	default:
		return nil
	}
}

func (c *Controller) handleInvokeResponse(ctx context.Context, m *actorpb.InvokeResponse) error {
	splits := strings.SplitN(m.RuntimeID, "::", 3)
	functionName, sessionID, instanceID := splits[0], splits[1], splits[2]
	function, ok := c.functions[functionName]
	if !ok {
		logrus.Errorf("Function not found for function name %s", functionName)
		return fmt.Errorf("Function not found for function name %s", functionName)
	}
	function.Done(ctx, m.RuntimeID, m.Info)
	logrus.WithFields(logrus.Fields{"function": functionName, "runtime": m.RuntimeID}).Info("control: invoke response")

	dag, ok := c.dags[sessionID]
	if !ok {
		logrus.Errorf("DAG not found for session %s", sessionID)
		return fmt.Errorf("DAG not found for session %s", sessionID)
	}
	controlNode, ok := dag.GetControlNode(task.DAGNodeID(instanceID))
	if !ok {
		logrus.Errorf("ControlNode not found for instance %s", instanceID)
		return fmt.Errorf("ControlNode not found for instance %s", instanceID)
	}
	controlNode.Done()
	dataNode, ok := dag.GetDataNode(controlNode.DataNode)
	if !ok {
		logrus.Errorf("DataNode not found for control node %s", controlNode.ID)
		return fmt.Errorf("DataNode not found for control node %s", controlNode.ID)
	}
	dataNode.Done(m.Result)

	ret := ctrlpb.NewReturnResult(sessionID, instanceID, functionName, m.Result, nil)
	return c.PushToClient(ctx, ret)
}

func (c *Controller) HandleClientMessage(ctx context.Context, msg *ctrlpb.Message) error {
	switch m := msg.GetCommand().(type) {
	case *ctrlpb.Message_AppendPyFunc:
		return c.handleAppendPyFunc(ctx, m.AppendPyFunc)
	case *ctrlpb.Message_AppendData:
		return c.handleAppendData(ctx, m.AppendData)
	case *ctrlpb.Message_AppendPyClass:
		return c.handleAppendPyClass(ctx, m.AppendPyClass)
	case *ctrlpb.Message_AppendArg:
		return c.handleAppendArg(ctx, m.AppendArg)
	case *ctrlpb.Message_AppendClassMethodArg:
		return c.handleAppendClassMethodArg(ctx, m.AppendClassMethodArg)
	case *ctrlpb.Message_Invoke:
		return c.handleInvoke(ctx, m.Invoke)
	case *ctrlpb.Message_AppendDAGNode:
		return c.handleAppendDAGNode(ctx, m.AppendDAGNode)
	case *ctrlpb.Message_RequestObject:
		return c.handleRequestObject(ctx, m.RequestObject)
	default:
		return nil
	}
}

func (c *Controller) handleAppendData(ctx context.Context, m *ctrlpb.AppendData) error {
	obj := m.Object
	sessionID := m.SessionID
	if sessionID == "" {
		logrus.Errorf("session ID is required")
		ret := ctrlpb.NewReturnResult(m.SessionID, "", obj.ID, nil, fmt.Errorf("session ID is required"))
		c.PushToClient(ctx, ret)
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"id":      obj.ID,
		"session": sessionID,
	}).Info("control: append data")

	dag, ok := c.dags[sessionID]
	if !ok {
		logrus.Errorf("DAG not found for session %s", sessionID)
		ret := ctrlpb.NewReturnResult(m.SessionID, "", obj.ID, nil, fmt.Errorf("DAG not found for session %s", sessionID))
		c.PushToClient(ctx, ret)
		return nil
	}

	lambdaID := obj.ID
	dataNode, ok := dag.GetDataNode(m.InstanceID)
	if !ok {
		logrus.Errorf("DataNode not found for lambda %s in session %s", lambdaID, sessionID)
		ret := ctrlpb.NewReturnResult(m.SessionID, "", obj.ID, nil, fmt.Errorf("data node not found for lambda %s in session %s", lambdaID, sessionID))
		c.PushToClient(ctx, ret)
		return nil
	}

	go func() {
		resp, err := c.storeService.SaveObject(ctx, m.Object)
		if err != nil {
			logrus.Errorf("Failed to save object: %v", err)
			dataNode.Status = task.DAGNodeStatusFailed
			return
		}

		logrus.WithFields(logrus.Fields{
			"data_node_id": dataNode.ID,
			"lambda_id":    lambdaID,
			"object_id":    resp.ID,
			"session":      sessionID,
		}).Info("control: object saved, marking data node as done")

		dataNode.Done(resp)

		ret := ctrlpb.NewReturnResult(m.SessionID, "", resp.ID, resp, nil)
		c.PushToClient(ctx, ret)
	}()

	return nil
}

func (c *Controller) handleAppendPyFunc(ctx context.Context, m *ctrlpb.AppendPyFunc) error {
	actorGroup := task.NewGroup(m.GetName())
	replicas := int(m.GetReplicas())
	resourceReq := &types.Info{
		CPU:    int64(m.GetResources().GetCPU()),
		Memory: int64(m.GetResources().GetMemory()),
		GPU:    int64(m.GetResources().GetGPU()),
		Tags:   append([]string(nil), m.GetTags()...),
	}

	for i := 0; i < replicas; i++ {
		actorName := fmt.Sprintf("%s-%d", m.GetName(), i)
		logrus.Infof("Deploying component for actor %s", actorName)
		component, err := c.componentService.DeployComponent(
			ctx, types.RuntimeEnvPython,
			resourceReq,
		)
		if err != nil {
			logrus.Errorf("Failed to deploy component: %v", err)
			return err
		}
		logrus.Infof("Component deployed successfully: %s", component.GetID())

		actor := task.NewActor(actorName, component)
		actor.Send(&actorpb.Function{
			Name:          m.GetName(),
			Params:        m.GetParams(),
			Requirements:  m.GetRequirements(),
			PickledObject: m.GetPickledObject(),
			Language:      m.GetLanguage(),
		})
		logrus.Infof("Function sent to actor: %s", actor.GetID())

		go func() {
			for {
				msg := actor.Receive(ctx)
				if msg == nil {
					if ctx.Err() == context.Canceled {
						logrus.Info("actor receive canceled by context")
					} else {
						logrus.Error("actor receive returned nil message")
					}
					return
				}
				if err := c.HandleActorMessage(ctx, msg); err != nil {
					logrus.Errorf("Failed to handle actor message: %v", err)
					return
				}
			}
		}()
		actorGroup.Push(actor)
	}

	c.functions[m.GetName()] = task.NewFunction(m.GetName(), m.GetParams(), actorGroup)

	return nil
}

func (c *Controller) handleAppendArg(ctx context.Context, m *ctrlpb.AppendArg) error {
	logrus.WithFields(logrus.Fields{"name": m.Name, "param": m.Param, "session": m.SessionID, "instance": m.InstanceID}).Info("control: append arg")

	dag, ok := c.dags[m.SessionID]
	if !ok {
		logrus.Errorf("DAG not found for session %s", m.SessionID)
		return fmt.Errorf("DAG not found for session %s", m.SessionID)
	}

	controlNode, ok := dag.GetControlNode(task.DAGNodeID(m.InstanceID))
	if !ok {
		logrus.Errorf("ControlNode not found for session %s", m.SessionID)
		return fmt.Errorf("ControlNode not found for session %s", m.SessionID)
	}

	function, ok := c.functions[controlNode.FunctionName]
	if !ok {
		logrus.Errorf("Function not found for function name %s", controlNode.FunctionName)
		return fmt.Errorf("Function not found for function name %s", controlNode.FunctionName)
	}

	switch v := m.Value.Object.(type) {
	case *ctrlpb.Data_Ref:
		if err := function.AddArg(controlNode.RuntimeID, m.Param, v.Ref); err != nil {
			logrus.Errorf("Failed to add runtime argument: %v", err)
			ret := ctrlpb.NewReturnResult(m.SessionID, m.InstanceID, m.Name, nil, err)
			c.PushToClient(ctx, ret)
			return err
		}
		if function.IsReady(controlNode.RuntimeID) {
			controlNode.Ready()
		}
	case *ctrlpb.Data_Encoded:
		logrus.WithFields(logrus.Fields{"id": v.Encoded.ID, "session": m.SessionID, "instance": m.InstanceID}).Info("control: append encoded arg")
		go func() {
			resp, err := c.storeService.SaveObject(ctx, v.Encoded)
			if err != nil {
				logrus.Errorf("Failed to save object: %v", err)
				ret := ctrlpb.NewReturnResult(m.SessionID, m.InstanceID, m.Name, nil, err)
				c.PushToClient(ctx, ret)
				return
			}
			logrus.Infof("Object saved successfully: %s", resp.ID)
			if err = function.AddArg(controlNode.RuntimeID, m.Param, resp); err != nil {
				logrus.Errorf("Failed to add runtime argument: %v", err)
				ret := ctrlpb.NewReturnResult(m.SessionID, m.InstanceID, m.Name, nil, err)
				c.PushToClient(ctx, ret)
				return
			}
		}()
	default:
		logrus.Errorf("Unsupported data type: %T", v)
		return fmt.Errorf("unsupported data type: %T", v)
	}
	return nil
}

func (c *Controller) handleInvoke(ctx context.Context, m *ctrlpb.Invoke) error {
	logrus.WithFields(logrus.Fields{"name": m.Name, "session": m.SessionID, "instance": m.InstanceID}).Info("control: invoke")
	dag, ok := c.dags[m.SessionID]
	if !ok {
		logrus.Errorf("DAG not found for session %s", m.SessionID)
		return fmt.Errorf("DAG not found for session %s", m.SessionID)
	}
	controlNode, ok := dag.GetControlNode(task.DAGNodeID(m.InstanceID))
	if !ok {
		logrus.Errorf("ControlNode not found for session %s", m.SessionID)
		return fmt.Errorf("ControlNode not found for session %s", m.SessionID)
	}
	function, ok := c.functions[controlNode.FunctionName]
	if !ok {
		logrus.Errorf("Function not found for function name %s", controlNode.FunctionName)
		return fmt.Errorf("Function not found for function name %s", controlNode.FunctionName)
	}
	controlNode.Start()
	return function.Invoke(ctx, controlNode.RuntimeID)
}

func (c *Controller) handleAppendPyClass(ctx context.Context, m *ctrlpb.AppendPyClass) error {
	panic("not implemented")
}

func (c *Controller) handleAppendClassMethodArg(ctx context.Context, m *ctrlpb.AppendClassMethodArg) error {
	panic("not implemented")
}

func (c *Controller) handleAppendDAGNode(ctx context.Context, m *ctrlpb.AppendDAGNode) error {
	sessionID := m.SessionID
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	// 获取或创建 DAG
	dag, ok := c.dags[sessionID]
	if !ok {
		dag = task.NewDAG(sessionID)
		c.dags[sessionID] = dag
		logrus.WithFields(logrus.Fields{"session": sessionID}).Info("control: created new DAG")
	}

	switch m.Type {
	case ctrlpb.DAGNodeType_DAG_NODE_TYPE_CONTROL:
		return c.handleAppendControlNode(ctx, dag, m.GetControlNode(), sessionID)
	case ctrlpb.DAGNodeType_DAG_NODE_TYPE_DATA:
		return c.handleAppendDataNode(ctx, dag, m.GetDataNode(), sessionID)
	default:
		return fmt.Errorf("unsupported DAG node type: %v", m.Type)
	}
}

// handleAppendControlNode 处理控制节点的添加
func (c *Controller) handleAppendControlNode(ctx context.Context, dag *task.DAG, pbNode *ctrlpb.ControlNode, sessionID string) error {
	if pbNode == nil {
		return fmt.Errorf("control node is nil")
	}

	logrus.WithFields(logrus.Fields{
		"id":       pbNode.Id,
		"function": pbNode.FunctionName,
		"session":  sessionID,
	}).Info("control: append control node")

	// 检查函数是否存在
	function, ok := c.functions[pbNode.FunctionName]
	if !ok {
		return fmt.Errorf("function %s not found", pbNode.FunctionName)
	}

	// 构建参数映射（lambda_id -> parameter_name）
	params := make(map[string]string)
	maps.Copy(params, pbNode.Params)

	// 构建前置数据节点 IDs
	preDataNodes := make([]task.DAGNodeID, 0, len(pbNode.PreDataNodes))
	for _, dataNodeID := range pbNode.PreDataNodes {
		preDataNodes = append(preDataNodes, task.DAGNodeID(dataNodeID))
	}

	runtimeID := function.Runtime(sessionID, pbNode.Id)

	// 创建 ControlNode
	controlNode := &task.ControlNode{
		ID:           task.DAGNodeID(pbNode.Id),
		FunctionName: pbNode.FunctionName,
		Params:       params,
		Current:      pbNode.Current,
		DataNode:     task.DAGNodeID(pbNode.DataNode),
		PreDataNodes: preDataNodes,
		FunctionType: pbNode.FunctionType,
		Status:       task.DAGNodeStatusPending,
		RuntimeID:    runtimeID,
	}

	// 添加到 DAG
	dag.AddControlNode(controlNode)

	logrus.WithFields(logrus.Fields{
		"id":      controlNode.ID,
		"runtime": runtimeID,
		"session": sessionID,
	}).Info("control: control node added to DAG")

	return nil
}

// handleAppendDataNode 处理数据节点的添加
func (c *Controller) handleAppendDataNode(ctx context.Context, dag *task.DAG, pbNode *ctrlpb.DataNode, sessionID string) error {
	if pbNode == nil {
		return fmt.Errorf("data node is nil")
	}

	logrus.WithFields(logrus.Fields{
		"id":      pbNode.Id,
		"lambda":  pbNode.Lambda,
		"session": sessionID,
	}).Info("control: append data node")

	// 构建后续控制节点 IDs
	sufControlNodes := make([]task.DAGNodeID, 0, len(pbNode.SufControlNodes))
	for _, controlNodeID := range pbNode.SufControlNodes {
		sufControlNodes = append(sufControlNodes, task.DAGNodeID(controlNodeID))
	}

	// 构建子数据节点 IDs
	childNodes := make([]task.DAGNodeID, 0, len(pbNode.ChildNode))
	for _, childNodeID := range pbNode.ChildNode {
		childNodes = append(childNodes, task.DAGNodeID(childNodeID))
	}

	// 创建 DataNode
	dataNode := &task.DataNode{
		ID:              task.DAGNodeID(pbNode.Id),
		Lambda:          pbNode.Lambda,
		SufControlNodes: sufControlNodes,
		Status:          task.DAGNodeStatusPending,
		ChildNodes:      childNodes,
	}

	// 设置前置控制节点（如果存在）
	if pbNode.PreControlNode != nil && *pbNode.PreControlNode != "" {
		preControlNodeID := task.DAGNodeID(*pbNode.PreControlNode)
		dataNode.PreControlNode = &preControlNodeID
	}

	// 设置父数据节点（如果存在）
	if pbNode.ParentNode != nil && *pbNode.ParentNode != "" {
		parentNodeID := task.DAGNodeID(*pbNode.ParentNode)
		dataNode.ParentNode = &parentNodeID
	}

	// 没有前驱控制节点，视为已准备好
	if pbNode.PreControlNode == nil || (pbNode.PreControlNode != nil && *pbNode.PreControlNode == "") {
		dataNode.Status = task.DAGNodeStatusDone
	}

	// 添加到 DAG
	dag.AddDataNode(dataNode)

	logrus.WithFields(logrus.Fields{
		"id":      dataNode.ID,
		"lambda":  dataNode.Lambda,
		"session": sessionID,
	}).Info("control: data node added to DAG")

	return nil
}

func (c *Controller) handleRequestObject(ctx context.Context, m *ctrlpb.RequestObject) error {
	logrus.WithFields(logrus.Fields{"id": m.ID, "source": m.Source}).Info("control: request object")
	object, err := c.storeService.GetObject(ctx, &commonpb.ObjectRef{
		ID:     m.ID,
		Source: m.Source,
	})
	if err != nil {
		logrus.Errorf("Failed to get object: %v", err)
		return err
	}
	ret := ctrlpb.NewResponseObject(m.ID, object, nil)
	return c.PushToClient(ctx, ret)
}

func (c *Controller) emit(ctx context.Context, event Event) {
	if c.events == nil {
		return
	}
	c.events.Publish(ctx, event)
}

func (c *Controller) PushToClient(ctx context.Context, msg *ctrlpb.Message) error {
	if msg == nil {
		return errors.New("push message: nil payload")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.toClientChan <- msg:
		return nil
	}
}

func (c *Controller) GetActors() map[string][]*task.Actor {
	actors := make(map[string][]*task.Actor)
	for _, function := range c.functions {
		actors[function.GetName()] = function.GetActors()
	}
	return actors
}
