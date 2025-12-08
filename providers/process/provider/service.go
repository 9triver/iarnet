package provider

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	commonpb "github.com/9triver/iarnet/internal/proto/common"
	ctrlpb "github.com/9triver/iarnet/internal/proto/ignis/controller"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	componentpb "github.com/9triver/iarnet/internal/proto/resource/component"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const providerType = "process"

// Service Process provider 服务实现
type Service struct {
	providerpb.UnimplementedServiceServer
	mu           sync.RWMutex
	ignisConn    *grpc.ClientConn
	ignisClient  ctrlpb.ServiceClient
	manager      *Manager
	resourceTags *providerpb.ResourceTags

	// 资源容量管理（从配置文件读取）
	totalCapacity *resourcepb.Info // 配置的总容量
	allocated     *resourcepb.Info // 当前已分配的容量（内存中动态维护）

	// 组件会话管理：componentID -> session
	componentSessions map[string]*ComponentSession
	sessionsMu        sync.RWMutex

	// iarnet zmq 消息发送器（由外部注入）
	zmqSender func(componentID string, data []byte)
}

// ComponentSession 管理一个组件在 ignis 中的 gRPC stream 会话
type ComponentSession struct {
	componentID string
	stream      ctrlpb.Service_SessionClient
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	zmqSender   func(componentID string, data []byte)
	ignisClient ctrlpb.ServiceClient
}

// NewService 创建新的 Process provider 服务
func NewService(ignisAddress string, resourceTags []string, totalCapacity *resourcepb.Info, zmqSender func(componentID string, data []byte)) (*Service, error) {
	// 连接到 Ignis
	conn, err := grpc.NewClient(ignisAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ignis: %w", err)
	}

	client := ctrlpb.NewServiceClient(conn)

	// 创建健康检查管理器
	manager := NewManager(
		90*time.Second,
		10*time.Second,
		func() {
			logrus.Debug("Provider ID cleared due to health check timeout")
		},
	)

	service := &Service{
		ignisConn:   conn,
		ignisClient: client,
		manager:     manager,
		resourceTags: &providerpb.ResourceTags{
			Cpu:    contains(resourceTags, "cpu"),
			Memory: contains(resourceTags, "memory"),
			Gpu:    contains(resourceTags, "gpu"),
			Camera: contains(resourceTags, "camera"),
		},
		totalCapacity: totalCapacity,
		allocated: &resourcepb.Info{
			Cpu:    0,
			Memory: 0,
			Gpu:    0,
		},
		componentSessions: make(map[string]*ComponentSession),
		zmqSender:         zmqSender,
	}

	// 启动健康检测超时监控
	manager.Start()

	return service, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Close 关闭服务
func (s *Service) Close() error {
	if s.manager != nil {
		s.manager.Stop()
	}

	// 关闭所有组件会话
	s.sessionsMu.Lock()
	for _, session := range s.componentSessions {
		session.cancel()
	}
	s.sessionsMu.Unlock()

	// 等待所有会话关闭
	s.sessionsMu.RLock()
	for _, session := range s.componentSessions {
		session.wg.Wait()
	}
	s.sessionsMu.RUnlock()

	if s.ignisConn != nil {
		return s.ignisConn.Close()
	}
	return nil
}

// Connect 处理 provider 连接请求
func (s *Service) Connect(ctx context.Context, req *providerpb.ConnectRequest) (*providerpb.ConnectResponse, error) {
	if req == nil {
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   "request is nil",
		}, nil
	}

	if req.ProviderId == "" {
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   "provider ID is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager.GetProviderID() != "" && s.manager.GetProviderID() != req.ProviderId {
		logrus.Errorf("provider already connected: %s", s.manager.GetProviderID())
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   fmt.Sprintf("provider already connected: %s", s.manager.GetProviderID()),
		}, nil
	}

	// 通过 manager 设置 provider ID
	if s.manager != nil {
		s.manager.SetProviderID(req.ProviderId)
	}
	logrus.Infof("Provider ID assigned: %s", s.manager.GetProviderID())

	return &providerpb.ConnectResponse{
		Success: true,
		ProviderType: &providerpb.ProviderType{
			Name: providerType,
		},
	}, nil
}

// checkAuth 检查鉴权
func (s *Service) checkAuth(requestProviderID string, allowUnconnected bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.manager.GetProviderID() == "" {
		if allowUnconnected {
			return nil
		}
		return fmt.Errorf("provider not connected, please call Connect first")
	}

	if requestProviderID == "" {
		return fmt.Errorf("provider_id is required for authenticated requests")
	}

	if requestProviderID != s.manager.GetProviderID() {
		return fmt.Errorf("unauthorized: provider_id mismatch, expected %s, got %s", s.manager.GetProviderID(), requestProviderID)
	}

	return nil
}

// HealthCheck 健康检测
func (s *Service) HealthCheck(ctx context.Context, req *providerpb.HealthCheckRequest) (*providerpb.HealthCheckResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// 更新最后收到健康检测的时间
	if s.manager != nil {
		s.manager.UpdateHealthCheck()
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured")
	}

	available := &resourcepb.Info{
		Cpu:    total.Cpu - allocated.Cpu,
		Memory: total.Memory - allocated.Memory,
		Gpu:    total.Gpu - allocated.Gpu,
	}

	capacity := &resourcepb.Capacity{
		Total:     total,
		Used:      allocated,
		Available: available,
	}

	return &providerpb.HealthCheckResponse{
		Capacity:     capacity,
		ResourceTags: s.resourceTags,
	}, nil
}

// GetCapacity 获取资源容量
func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured")
	}

	available := &resourcepb.Info{
		Cpu:    total.Cpu - allocated.Cpu,
		Memory: total.Memory - allocated.Memory,
		Gpu:    total.Gpu - allocated.Gpu,
	}

	capacity := &resourcepb.Capacity{
		Total:     total,
		Used:      allocated,
		Available: available,
	}

	return &providerpb.GetCapacityResponse{
		Capacity: capacity,
	}, nil
}

// GetAvailable 获取可用资源
func (s *Service) GetAvailable(ctx context.Context, req *providerpb.GetAvailableRequest) (*providerpb.GetAvailableResponse, error) {
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	s.mu.RUnlock()

	if total == nil {
		return nil, fmt.Errorf("resource capacity not configured")
	}

	return &providerpb.GetAvailableResponse{
		Available: &resourcepb.Info{
			Cpu:    total.Cpu - allocated.Cpu,
			Memory: total.Memory - allocated.Memory,
			Gpu:    total.Gpu - allocated.Gpu,
		},
	}, nil
}

// Disconnect 断开连接
func (s *Service) Disconnect(ctx context.Context, req *providerpb.DisconnectRequest) (*providerpb.DisconnectResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager != nil {
		s.manager.ClearProviderID()
	}
	logrus.Infof("Provider disconnected: %s", req.ProviderId)

	return &providerpb.DisconnectResponse{}, nil
}

// GetRealTimeUsage 获取实时资源使用情况
func (s *Service) GetRealTimeUsage(ctx context.Context, req *providerpb.GetRealTimeUsageRequest) (*providerpb.GetRealTimeUsageResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Process provider 暂时返回已分配的资源作为使用量
	s.mu.RLock()
	allocated := s.allocated
	s.mu.RUnlock()

	return &providerpb.GetRealTimeUsageResponse{
		Usage: &resourcepb.Info{
			Cpu:    allocated.Cpu,
			Memory: allocated.Memory,
			Gpu:    allocated.Gpu,
		},
	}, nil
}

// Deploy 部署组件到 Ignis
func (s *Service) Deploy(ctx context.Context, req *providerpb.DeployRequest) (*providerpb.DeployResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": req.InstanceId,
		"image":       req.Image,
		"resources":   req.ResourceRequest,
	}).Info("process provider deploy component")

	// 创建组件会话，建立到 Ignis 的 gRPC stream
	session, err := s.createComponentSession(ctx, req.InstanceId)
	if err != nil {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("failed to create component session: %v", err),
		}, nil
	}

	// 更新已分配的资源容量
	s.mu.Lock()
	s.allocated.Cpu += req.ResourceRequest.Cpu
	s.allocated.Memory += req.ResourceRequest.Memory
	s.allocated.Gpu += req.ResourceRequest.Gpu
	s.mu.Unlock()

	logrus.Infof("Component %s deployed successfully, allocated resources: CPU=%d, Memory=%d, GPU=%d",
		req.InstanceId, req.ResourceRequest.Cpu, req.ResourceRequest.Memory, req.ResourceRequest.Gpu)

	// 启动会话的消息处理循环
	session.Start()

	return &providerpb.DeployResponse{
		Error: "",
	}, nil
}

// createComponentSession 为组件创建到 Ignis 的 gRPC stream 会话
func (s *Service) createComponentSession(ctx context.Context, componentID string) (*ComponentSession, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// 检查是否已存在会话
	if existing, ok := s.componentSessions[componentID]; ok {
		return existing, nil
	}

	// 创建新的 gRPC stream
	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := s.ignisClient.Session(streamCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create ignis session: %w", err)
	}

	session := &ComponentSession{
		componentID: componentID,
		stream:      stream,
		ctx:         streamCtx,
		cancel:      cancel,
		zmqSender:   s.zmqSender,
		ignisClient: s.ignisClient,
	}

	s.componentSessions[componentID] = session

	return session, nil
}

// Start 启动组件会话的消息处理循环
func (cs *ComponentSession) Start() {
	cs.wg.Add(2)

	// 启动从 Ignis 接收消息的 goroutine
	go func() {
		defer cs.wg.Done()
		cs.receiveFromIgnis()
	}()

	// 发送 READY 消息到 Ignis
	go func() {
		defer cs.wg.Done()
		readyMsg := &ctrlpb.Message{
			Type:  ctrlpb.CommandType_FR_READY,
			AppID: cs.componentID,
			Command: &ctrlpb.Message_Ready{
				Ready: &commonpb.Ready{},
			},
		}
		if err := cs.stream.Send(readyMsg); err != nil {
			logrus.Errorf("Failed to send READY to ignis for component %s: %v", cs.componentID, err)
		}
	}()
}

// receiveFromIgnis 从 Ignis 接收消息并转换为 ZMQ 消息
func (cs *ComponentSession) receiveFromIgnis() {
	for {
		select {
		case <-cs.ctx.Done():
			return
		default:
			msg, err := cs.stream.Recv()
			if err == io.EOF {
				logrus.Infof("Ignis stream closed for component %s", cs.componentID)
				return
			}
			if err != nil {
				logrus.Errorf("Failed to receive from ignis for component %s: %v", cs.componentID, err)
				return
			}

			// 将 Ignis 消息转换为 iarnet component 消息
			componentMsg, err := cs.convertIgnisToIarnet(msg)
			if err != nil {
				logrus.Errorf("Failed to convert ignis message for component %s: %v", cs.componentID, err)
				continue
			}

			// 通过 ZMQ 发送到组件
			data, err := proto.Marshal(componentMsg)
			if err != nil {
				logrus.Errorf("Failed to marshal component message for component %s: %v", cs.componentID, err)
				continue
			}

			if cs.zmqSender != nil {
				cs.zmqSender(cs.componentID, data)
			}
		}
	}
}

// convertIgnisToIarnet 将 Ignis 消息转换为 iarnet component 消息
func (cs *ComponentSession) convertIgnisToIarnet(ignisMsg *ctrlpb.Message) (*componentpb.Message, error) {
	componentMsg := &componentpb.Message{}

	switch cmd := ignisMsg.GetCommand().(type) {
	case *ctrlpb.Message_Ready:
		componentMsg.Type = componentpb.MessageType_READY
		componentMsg.Message = &componentpb.Message_Ready{
			Ready: &commonpb.Ready{},
		}
	case *ctrlpb.Message_AppendPyFunc:
		// 将 AppendPyFunc 转换为 Payload
		anyMsg, err := anypb.New(cmd.AppendPyFunc)
		if err != nil {
			return nil, fmt.Errorf("failed to create anypb: %w", err)
		}
		componentMsg.Type = componentpb.MessageType_PAYLOAD
		componentMsg.Message = &componentpb.Message_Payload{
			Payload: anyMsg,
		}
	case *ctrlpb.Message_Invoke:
		// 将 Invoke 转换为 Payload
		anyMsg, err := anypb.New(cmd.Invoke)
		if err != nil {
			return nil, fmt.Errorf("failed to create anypb: %w", err)
		}
		componentMsg.Type = componentpb.MessageType_PAYLOAD
		componentMsg.Message = &componentpb.Message_Payload{
			Payload: anyMsg,
		}
	default:
		// 其他消息类型也转换为 Payload
		if ignisMsg.GetCommand() != nil {
			anyMsg, err := anypb.New(ignisMsg)
			if err != nil {
				return nil, fmt.Errorf("failed to create anypb: %w", err)
			}
			componentMsg.Type = componentpb.MessageType_PAYLOAD
			componentMsg.Message = &componentpb.Message_Payload{
				Payload: anyMsg,
			}
		}
	}

	return componentMsg, nil
}

// SendToIgnis 将 iarnet ZMQ 消息发送到 Ignis
func (cs *ComponentSession) SendToIgnis(componentMsg *componentpb.Message) error {
	// 将 iarnet component 消息转换为 Ignis 消息
	ignisMsg, err := cs.convertIarnetToIgnis(componentMsg)
	if err != nil {
		return fmt.Errorf("failed to convert iarnet message: %w", err)
	}

	return cs.stream.Send(ignisMsg)
}

// convertIarnetToIgnis 将 iarnet component 消息转换为 Ignis 消息
func (cs *ComponentSession) convertIarnetToIgnis(componentMsg *componentpb.Message) (*ctrlpb.Message, error) {
	ignisMsg := &ctrlpb.Message{
		AppID: cs.componentID,
	}

	switch msg := componentMsg.GetMessage().(type) {
	case *componentpb.Message_Ready:
		ignisMsg.Type = ctrlpb.CommandType_FR_READY
		ignisMsg.Command = &ctrlpb.Message_Ready{
			Ready: &commonpb.Ready{},
		}
	case *componentpb.Message_Payload:
		// 尝试解析 Payload 中的消息
		var innerMsg proto.Message
		if err := msg.Payload.UnmarshalTo(innerMsg); err != nil {
			// 如果解析失败，直接发送 Invoke
			ignisMsg.Type = ctrlpb.CommandType_FR_INVOKE
			ignisMsg.Command = &ctrlpb.Message_Invoke{
				Invoke: &ctrlpb.Invoke{},
			}
		} else {
			// 根据消息类型设置对应的命令
			// 这里需要根据实际的消息类型进行转换
			// 暂时默认使用 Invoke
			ignisMsg.Type = ctrlpb.CommandType_FR_INVOKE
			ignisMsg.Command = &ctrlpb.Message_Invoke{
				Invoke: &ctrlpb.Invoke{},
			}
		}
	default:
		ignisMsg.Type = ctrlpb.CommandType_FR_INVOKE
		ignisMsg.Command = &ctrlpb.Message_Invoke{
			Invoke: &ctrlpb.Invoke{},
		}
	}

	return ignisMsg, nil
}

// Stop 停止组件会话
func (cs *ComponentSession) Stop() {
	cs.cancel()
	cs.wg.Wait()
	if cs.stream != nil {
		cs.stream.CloseSend()
	}
}
