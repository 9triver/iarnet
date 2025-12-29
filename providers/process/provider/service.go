package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	commonpb "github.com/9triver/iarnet/internal/proto/common"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	componentpb "github.com/9triver/iarnet/internal/proto/resource/component"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	ignisprotopb "github.com/9triver/ignis/proto"
	ignisctrlpb "github.com/9triver/ignis/proto/controller"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"gopkg.in/zeromq/goczmq.v4"
)

const providerType = "process"

// Service Process provider 服务实现
type Service struct {
	providerpb.UnimplementedServiceServer
	mu           sync.RWMutex
	ignisConn    *grpc.ClientConn
	ignisClient  ignisctrlpb.ServiceClient // 使用 ignis-go 的 ServiceClient
	manager      *Manager
	resourceTags *providerpb.ResourceTags

	// 资源容量管理（从配置文件读取）
	totalCapacity *resourcepb.Info // 配置的总容量
	allocated     *resourcepb.Info // 当前已分配的容量（内存中动态维护）

	// 支持的语言列表（从配置文件读取）
	supportedLanguages []commonpb.Language

	// DNS 配置：主机名到 IP 地址的映射
	dnsHosts map[string]string

	// 共享的 Ignis stream session（所有 component 共用）
	ignisStream       grpc.BidiStreamingClient[ignisctrlpb.Message, ignisctrlpb.Message]
	ignisStreamCtx    context.Context
	ignisStreamCancel context.CancelFunc
	ignisStreamMu     sync.RWMutex // 保护 ignisStream 的锁
	ignisStreamOnce   sync.Once    // 确保只初始化一次

	// 消息发送队列：确保消息串行发送到 Ignis
	sendQueue chan *queuedMessage
	sendWg    sync.WaitGroup

	// 组件会话管理：componentID -> session
	componentSessions map[string]*ComponentSession
	sessionsMu        sync.RWMutex

	// 路由映射：sessionID::instanceID -> componentID
	// 用于将 Ignis 返回的消息路由到正确的 component
	routingMap map[string]string // sessionID::instanceID -> componentID
	routingMu  sync.RWMutex      // 保护 routingMap 的锁

	// 全局对象映射：objectID -> ignisStoreID
	// 记录 ignis 中保存的对象 ID 和对应的 source ID（ignis store ID）
	// 这是全局的，因为同一个对象可能被多个 component 使用
	ignisObjects   map[string]string // objectID -> ignisStoreID
	ignisObjectsMu sync.RWMutex      // 保护 ignisObjects 的锁
}

// queuedMessage 队列中的消息，包含 componentID 和要发送的消息
type queuedMessage struct {
	componentID string
	msg         *ignisctrlpb.Message
}

// ComponentSession 管理一个组件的会话
// 注意：不再包含独立的 stream，所有消息通过 Service 的共享 stream 发送
type ComponentSession struct {
	componentID              string
	service                  *Service // 引用 Service，用于发送消息
	ctx                      context.Context
	cancel                   context.CancelFunc
	wg                       sync.WaitGroup
	zmqSocket                *goczmq.Sock      // 每个 component 独立的 ZMQ socket（用于发送和接收）
	storeAddr                string            // iarnet store 地址
	storeID                  string            // iarnet store ID（用于在响应中设置 source 字段）
	functionSent             bool              // 标记是否已发送 append function 消息
	functionMu               sync.Mutex        // 保护 functionSent 的锁
	deployedFunctionName     string            // 已部署的函数名称（每个 component 只有一个函数）
	deployedFunctionLanguage commonpb.Language // 已部署函数的语言类型
	deployedFunctionMu       sync.RWMutex      // 保护 deployedFunction 字段的锁
}

// stringToLanguage 将字符串转换为 common.Language
func stringToLanguage(s string) commonpb.Language {
	switch s {
	case "python":
		return commonpb.Language_LANG_PYTHON
	case "go":
		return commonpb.Language_LANG_GO
	case "unikernel":
		// Unikernel 目前使用 Python 语言类型
		return commonpb.Language_LANG_PYTHON
	default:
		return commonpb.Language_LANG_UNKNOWN
	}
}

// NewService 创建新的 Process provider 服务
func NewService(ignisAddress string, resourceTags []string, totalCapacity *resourcepb.Info, supportedLanguages []string, dnsHosts map[string]string) (*Service, error) {
	// 连接到 Ignis
	conn, err := grpc.NewClient(ignisAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ignis: %w", err)
	}

	client := ignisctrlpb.NewServiceClient(conn)

	// 创建健康检查管理器
	manager := NewManager(
		90*time.Second,
		10*time.Second,
		func() {
			logrus.Debug("Provider ID cleared due to health check timeout")
		},
	)

	// 转换支持的语言列表
	languages := make([]commonpb.Language, 0, len(supportedLanguages))
	for _, langStr := range supportedLanguages {
		lang := stringToLanguage(langStr)
		if lang != commonpb.Language_LANG_UNKNOWN {
			languages = append(languages, lang)
		} else {
			logrus.Warnf("Unknown language in config: %s, skipping", langStr)
		}
	}
	// 如果没有配置任何语言，默认支持 Go
	if len(languages) == 0 {
		languages = []commonpb.Language{commonpb.Language_LANG_GO}
		logrus.Info("No supported languages configured, defaulting to Go")
	}

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
		totalCapacity:      totalCapacity,
		supportedLanguages: languages,
		dnsHosts:           dnsHosts,
		routingMap:         make(map[string]string),
		allocated: &resourcepb.Info{
			Cpu:    0,
			Memory: 0,
			Gpu:    0,
		},
		componentSessions: make(map[string]*ComponentSession),
		sendQueue:         make(chan *queuedMessage, 1000), // 缓冲队列，避免阻塞
		ignisObjects:      make(map[string]string),         // 全局对象映射
	}

	// 启动健康检测超时监控
	manager.Start()

	// 初始化共享的 Ignis stream（延迟初始化，在第一次使用时创建）
	// 启动消息发送 goroutine
	service.sendWg.Add(1)
	go service.sendMessageLoop()

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

// resolveAddress 解析地址，如果命中 DNS 配置，则替换主机名
func (s *Service) resolveAddress(addr string) string {
	if len(s.dnsHosts) == 0 {
		return addr
	}

	// 解析地址（可能是 host:port 格式）
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// 如果没有端口，直接检查主机名
		if ip, ok := s.dnsHosts[addr]; ok {
			logrus.Debugf("DNS mapping: %s -> %s", addr, ip)
			return ip
		}
		return addr
	}

	// 检查主机名是否在 DNS 配置中
	if ip, ok := s.dnsHosts[host]; ok {
		logrus.Debugf("DNS mapping: %s -> %s", host, ip)
		return net.JoinHostPort(ip, port)
	}

	return addr
}

// initIgnisStream 初始化共享的 Ignis stream（只初始化一次）
func (s *Service) initIgnisStream() error {
	var initErr error
	s.ignisStreamOnce.Do(func() {
		streamCtx, cancel := context.WithCancel(context.Background())
		stream, err := s.ignisClient.Session(streamCtx)
		if err != nil {
			cancel()
			initErr = fmt.Errorf("failed to create ignis session: %w", err)
			return
		}

		// 立即发送 READY 消息，通知 Ignis 连接已就绪
		readyMsg := ignisctrlpb.NewReady()
		if err := stream.Send(readyMsg); err != nil {
			cancel()
			initErr = fmt.Errorf("failed to send READY message to ignis: %w", err)
			return
		}
		logrus.Infof("Initialized shared Ignis stream and sent READY message")

		s.ignisStreamMu.Lock()
		s.ignisStream = stream
		s.ignisStreamCtx = streamCtx
		s.ignisStreamCancel = cancel
		s.ignisStreamMu.Unlock()

		// 启动从 Ignis 接收消息的 goroutine
		go s.receiveFromIgnis()
	})
	return initErr
}

// sendMessageToIgnis 将消息放入队列，由 sendMessageLoop 串行发送
func (s *Service) sendMessageToIgnis(componentID string, msg *ignisctrlpb.Message) error {
	// 记录消息类型用于调试
	msgType := "unknown"
	if msg != nil && msg.Type != 0 {
		msgType = msg.Type.String()
	}
	logrus.Debugf("Queuing message to Ignis for component %s: type=%s", componentID, msgType)

	select {
	case s.sendQueue <- &queuedMessage{
		componentID: componentID,
		msg:         msg,
	}:
		logrus.Debugf("Message queued successfully for component %s: type=%s", componentID, msgType)
		return nil
	default:
		logrus.Errorf("Send queue is full for component %s, dropping message: type=%s", componentID, msgType)
		return fmt.Errorf("send queue is full for component %s", componentID)
	}
}

// sendMessageLoop 消息发送循环：从队列中取出消息并串行发送到 Ignis
func (s *Service) sendMessageLoop() {
	defer s.sendWg.Done()
	logrus.Infof("Started message send loop for Ignis")

	for queuedMsg := range s.sendQueue {
		if queuedMsg == nil {
			logrus.Debugf("Received nil message in send queue, skipping")
			continue
		}

		// 记录消息类型
		msgType := "unknown"
		if queuedMsg.msg != nil && queuedMsg.msg.Type != 0 {
			msgType = queuedMsg.msg.Type.String()
		}

		s.ignisStreamMu.RLock()
		stream := s.ignisStream
		s.ignisStreamMu.RUnlock()

		if stream == nil {
			logrus.Errorf("Ignis stream not initialized, dropping message for component %s: type=%s", queuedMsg.componentID, msgType)
			continue
		}

		logrus.Debugf("Sending message to Ignis for component %s: type=%s", queuedMsg.componentID, msgType)
		if err := stream.Send(queuedMsg.msg); err != nil {
			logrus.Errorf("Failed to send message to Ignis for component %s: type=%s, error=%v", queuedMsg.componentID, msgType, err)
			// 如果发送失败，可能需要重新初始化 stream
			// 这里先记录错误，后续可以添加重连逻辑
		} else {
			logrus.Debugf("Successfully sent message to Ignis for component %s: type=%s", queuedMsg.componentID, msgType)
		}
	}
	logrus.Infof("Message send loop stopped")
}

// receiveFromIgnis 从共享的 Ignis stream 接收消息并路由到对应的 ComponentSession
func (s *Service) receiveFromIgnis() {
	s.ignisStreamMu.RLock()
	stream := s.ignisStream
	ctx := s.ignisStreamCtx
	s.ignisStreamMu.RUnlock()

	if stream == nil {
		return
	}

	logrus.Infof("Starting Ignis receiver for shared stream")
	defer logrus.Infof("Ignis receiver stopped for shared stream")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 使用 goroutine 和 channel 来非阻塞地接收消息，同时监听 context 取消
			type recvResult struct {
				msg *ignisctrlpb.Message
				err error
			}
			recvCh := make(chan recvResult, 1)

			// 在 goroutine 中调用阻塞的 Recv()
			go func() {
				msg, err := stream.Recv()
				recvCh <- recvResult{msg: msg, err: err}
			}()

			select {
			case <-ctx.Done():
				logrus.Debugf("Context canceled, stopping Ignis receiver")
				return
			case result := <-recvCh:
				if result.err == io.EOF {
					logrus.Infof("Ignis stream closed")
					return
				}
				if result.err != nil {
					logrus.Errorf("Failed to receive from ignis: %v", result.err)
					return
				}
				msg := result.msg

				// 路由消息到对应的 ComponentSession
				s.routeMessageToComponent(msg)
			}
		}
	}
}

// routeMessageToComponent 将 Ignis 消息路由到对应的 ComponentSession
func (s *Service) routeMessageToComponent(msg *ignisctrlpb.Message) {
	var targetComponentID string

	// 根据消息类型提取 component 标识
	if msg.Type == ignisctrlpb.CommandType_BK_RETURN_RESULT {
		returnResult := msg.GetReturnResult()
		if returnResult != nil {
			// 从 ReturnResult 中提取 sessionID 和 instanceID
			sessionID := returnResult.GetSessionID()
			instanceID := returnResult.GetInstanceID()
			if sessionID != "" && instanceID != "" {
				// 构建路由键：sessionID::instanceID
				routingKey := fmt.Sprintf("%s::%s", sessionID, instanceID)
				s.routingMu.RLock()
				targetComponentID = s.routingMap[routingKey]
				s.routingMu.RUnlock()
				logrus.Debugf("Routing ReturnResult: sessionID=%s, instanceID=%s, routingKey=%s, targetComponent=%s",
					sessionID, instanceID, routingKey, targetComponentID)
			}
		}
	} else if msg.Type == ignisctrlpb.CommandType_BK_RESPONSE_OBJECT {
		// ResponseObject 目前没有直接的路由信息
		// 可以通过对象 ID 查找，但需要额外的映射关系
		// 暂时使用广播机制，但可以优化为基于对象 ID 的路由
		responseObj := msg.GetResponseObject()
		if responseObj != nil {
			// TODO: 可以通过对象 ID 查找对应的 component
			// 目前暂时广播
		}
	}

	// 如果找到了目标 component，只发送给该 component
	if targetComponentID != "" {
		s.sessionsMu.RLock()
		session, exists := s.componentSessions[targetComponentID]
		s.sessionsMu.RUnlock()

		if exists {
			select {
			case <-session.ctx.Done():
				logrus.Warnf("Target component %s is done, dropping message", targetComponentID)
			default:
				go session.handleIgnisMessage(msg)
				return
			}
		} else {
			logrus.Warnf("Target component %s not found, message may be lost", targetComponentID)
		}
	}

	// 如果没有找到目标 component 或路由失败，广播到所有 session（向后兼容）
	// 但这种情况应该很少发生
	s.sessionsMu.RLock()
	sessions := make([]*ComponentSession, 0, len(s.componentSessions))
	for _, session := range s.componentSessions {
		sessions = append(sessions, session)
	}
	s.sessionsMu.RUnlock()

	for _, session := range sessions {
		select {
		case <-session.ctx.Done():
			continue
		default:
			go session.handleIgnisMessage(msg)
		}
	}
}

// Close 关闭服务
func (s *Service) Close() error {
	if s.manager != nil {
		s.manager.Stop()
	}

	// 关闭共享的 Ignis stream
	s.ignisStreamMu.Lock()
	if s.ignisStreamCancel != nil {
		s.ignisStreamCancel()
	}
	s.ignisStreamMu.Unlock()

	// 关闭消息发送队列
	close(s.sendQueue)
	s.sendWg.Wait()

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
		SupportedLanguages: s.supportedLanguages,
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
		Capacity:           capacity,
		ResourceTags:       s.resourceTags,
		SupportedLanguages: s.supportedLanguages,
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

	// 检查语言支持
	if req.Language != commonpb.Language_LANG_UNKNOWN {
		s.mu.RLock()
		supported := false
		for _, lang := range s.supportedLanguages {
			if lang == req.Language {
				supported = true
				break
			}
		}
		s.mu.RUnlock()

		if !supported {
			return &providerpb.DeployResponse{
				Error: fmt.Sprintf("unsupported language: %v, supported languages: %v", req.Language, s.supportedLanguages),
			}, nil
		}
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": req.InstanceId,
		"language":    req.Language,
		"image":       req.Image, // 已废弃，保留用于日志
		"resources":   req.ResourceRequest,
	}).Info("process provider deploy component")

	// 从环境变量中提取 ZMQ_ADDR 和 STORE_ADDR
	zmqAddr := req.EnvVars["ZMQ_ADDR"]
	if zmqAddr == "" {
		return &providerpb.DeployResponse{
			Error: "ZMQ_ADDR environment variable is required",
		}, nil
	}
	storeAddr := req.EnvVars["STORE_ADDR"]

	// 创建组件会话，建立到 Ignis 的 gRPC stream
	session, err := s.createComponentSession(ctx, req.InstanceId, zmqAddr, storeAddr)
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

// createComponentSession 为组件创建会话（不再创建独立的 stream，使用共享的 stream）
func (s *Service) createComponentSession(ctx context.Context, componentID string, zmqAddr string, storeAddr string) (*ComponentSession, error) {
	// 解析地址，如果命中 DNS 配置则替换
	if storeAddr != "" {
		storeAddr = s.resolveAddress(storeAddr)
	}
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	// 检查是否已存在会话
	if existing, ok := s.componentSessions[componentID]; ok {
		return existing, nil
	}

	// 确保共享的 Ignis stream 已初始化
	if err := s.initIgnisStream(); err != nil {
		return nil, fmt.Errorf("failed to initialize ignis stream: %w", err)
	}

	// 为每个 component 创建独立的 ZMQ socket
	zmqSocket, err := s.createZMQSocket(componentID, zmqAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create ZMQ socket: %w", err)
	}

	// 如果配置了 store 地址，获取 store ID
	storeID := ""
	if storeAddr != "" {
		storeID, err = s.getStoreID(ctx, storeAddr)
		if err != nil {
			logrus.Warnf("Failed to get store ID for component %s: %v, will retry later", componentID, err)
			// 不阻止创建 session，后续可以重试
		}
	}

	// 创建 component 的 context
	streamCtx, cancel := context.WithCancel(context.Background())

	session := &ComponentSession{
		componentID:              componentID,
		service:                  s,
		ctx:                      streamCtx,
		cancel:                   cancel,
		zmqSocket:                zmqSocket,
		storeAddr:                storeAddr,
		storeID:                  storeID,
		functionSent:             false,
		deployedFunctionName:     "",
		deployedFunctionLanguage: commonpb.Language_LANG_UNKNOWN,
	}

	s.componentSessions[componentID] = session

	return session, nil
}

// getStoreID 从 store 服务获取 store ID
func (s *Service) getStoreID(ctx context.Context, storeAddr string) (string, error) {
	// 解析地址，如果命中 DNS 配置则替换
	resolvedAddr := s.resolveAddress(storeAddr)
	conn, err := grpc.NewClient(resolvedAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("failed to connect to store: %w", err)
	}
	defer conn.Close()

	storeClient := storepb.NewServiceClient(conn)
	resp, err := storeClient.GetID(ctx, &storepb.GetIDRequest{})
	if err != nil {
		return "", fmt.Errorf("failed to get store ID: %w", err)
	}

	return resp.GetID(), nil
}

// createZMQSocket 为 component 创建独立的 ZMQ socket（用于发送和接收）
// 参考 containers/component/python/actor.py 的实现
func (s *Service) createZMQSocket(componentID string, zmqAddr string) (*goczmq.Sock, error) {
	// 使用 goczmq 创建 Dealer socket
	// 参考 Python: socket = ctx.socket(zmq.DEALER)
	//              socket.setsockopt_string(zmq.IDENTITY, component_id)
	//              socket.connect(zmq_addr)

	// 确保 ZMQ 地址包含协议前缀
	if !strings.HasPrefix(zmqAddr, "tcp://") && !strings.HasPrefix(zmqAddr, "ipc://") && !strings.HasPrefix(zmqAddr, "inproc://") {
		zmqAddr = "tcp://" + zmqAddr
	}

	// 解析地址，如果命中 DNS 配置则替换（在添加协议前缀后）
	// 对于 tcp://host:port 格式，需要提取 host:port 部分进行解析
	if strings.HasPrefix(zmqAddr, "tcp://") {
		hostPort := strings.TrimPrefix(zmqAddr, "tcp://")
		resolvedHostPort := s.resolveAddress(hostPort)
		zmqAddr = "tcp://" + resolvedHostPort
	} else {
		// 对于其他协议，直接解析整个地址
		zmqAddr = s.resolveAddress(zmqAddr)
	}

	// 创建 Dealer socket
	// 使用 NewSock 创建 DEALER 类型的 socket
	dealer := goczmq.NewSock(goczmq.Dealer)
	if dealer == nil {
		return nil, fmt.Errorf("failed to create ZMQ dealer socket")
	}

	// 设置 socket 身份标识（相当于 Python 的 socket.setsockopt_string(zmq.IDENTITY, component_id)）
	dealer.SetIdentity(componentID)

	// 连接到 ZMQ 地址
	if err := dealer.Connect(zmqAddr); err != nil {
		dealer.Destroy()
		return nil, fmt.Errorf("failed to connect ZMQ dealer socket to %s: %w", zmqAddr, err)
	}

	logrus.Infof("Created ZMQ socket for component %s, connecting to %s", componentID, zmqAddr)

	return dealer, nil
}

// Start 启动组件会话的消息处理循环
// 注意：不再需要从 Ignis 接收消息的 goroutine，因为使用共享的 stream
func (cs *ComponentSession) Start() {
	cs.wg.Add(1)

	// 启动从 ZMQ 接收消息的 goroutine
	go func() {
		defer cs.wg.Done()
		cs.receiveFromZMQ()
	}()
}

// sendAppendFunction 根据 Function 的语言类型发送相应的 append function 消息
func (cs *ComponentSession) sendAppendFunction(function *actorpb.Function) error {
	language := function.GetLanguage()
	functionName := function.GetName()
	logrus.Infof("Preparing to deploy function %s (language: %v) for component %s", functionName, language, cs.componentID)

	var appendFuncMsg *ignisctrlpb.Message

	switch language {
	case commonpb.Language_LANG_PYTHON:
		// 发送 AppendPyFunc 消息
		logrus.Debugf("Creating AppendPyFunc message for function %s, component %s", functionName, cs.componentID)
		appendPyFunc := &ignisctrlpb.AppendPyFunc{
			Name:          function.GetName(),
			Params:        function.GetParams(),
			Requirements:  function.GetRequirements(),
			PickledObject: function.GetPickledObject(),
			Language:      ignisprotopb.Language(language),
			Replicas:      1, // 默认 1 个副本
		}
		appendFuncMsg = &ignisctrlpb.Message{
			Type: ignisctrlpb.CommandType_FR_APPEND_PY_FUNC,
			Command: &ignisctrlpb.Message_AppendPyFunc{
				AppendPyFunc: appendPyFunc,
			},
		}
		logrus.Debugf("Created AppendPyFunc message for function %s: params=%v, requirements=%v, pickled_object_size=%d",
			functionName, appendPyFunc.Params, appendPyFunc.Requirements, len(appendPyFunc.PickledObject))
	case commonpb.Language_LANG_GO:
		// 发送 AppendGo 消息
		code := string(function.GetPickledObject())
		logrus.Infof("Deploying Go function %s for component %s: code_length=%d bytes, params=%v",
			functionName, cs.componentID, len(code), function.GetParams())
		logrus.Debugf("Go function %s code content:\n%s", functionName, code)

		// 验证代码格式
		hasInputStruct := strings.Contains(code, "type Input struct")
		hasImplFunc := strings.Contains(code, "func Impl")
		hasPackageMain := strings.Contains(code, "package main")

		if !hasPackageMain {
			logrus.Warnf("Go function %s code may be missing 'package main' declaration", functionName)
		}
		if !hasInputStruct {
			logrus.Warnf("Go function %s code may be missing Input struct definition", functionName)
		}
		if !hasImplFunc {
			logrus.Warnf("Go function %s code may be missing Impl function", functionName)
		}

		if hasPackageMain && hasInputStruct && hasImplFunc {
			logrus.Debugf("Go function %s code format validation passed", functionName)
		}

		appendGo := &ignisctrlpb.AppendGo{
			Name:     function.GetName(),
			Params:   function.GetParams(),
			Code:     code, // Go 代码存储在 PickledObject 中
			Language: ignisprotopb.Language(language),
			Replicas: 1, // 默认 1 个副本
		}
		appendFuncMsg = &ignisctrlpb.Message{
			Type: ignisctrlpb.CommandType_FR_APPEND_GO,
			Command: &ignisctrlpb.Message_AppendGo{
				AppendGo: appendGo,
			},
		}
		logrus.Debugf("Created AppendGo message for function %s: name=%s, params=%v, replicas=%d",
			functionName, appendGo.Name, appendGo.Params, appendGo.Replicas)
	default:
		logrus.Errorf("Unsupported language %v for function %s, component %s", language, functionName, cs.componentID)
		return fmt.Errorf("unsupported language: %v", language)
	}

	// 记录函数名称和语言类型（每个 component 只有一个函数）
	cs.deployedFunctionMu.Lock()
	cs.deployedFunctionName = functionName
	cs.deployedFunctionLanguage = language
	cs.deployedFunctionMu.Unlock()
	logrus.Debugf("Recorded function %s language %v for component %s", functionName, language, cs.componentID)

	// 通过消息队列发送，确保消息串行化
	logrus.Debugf("Sending append function message to queue for function %s, component %s", functionName, cs.componentID)
	if err := cs.service.sendMessageToIgnis(cs.componentID, appendFuncMsg); err != nil {
		logrus.Errorf("Failed to queue append function message for function %s, component %s: %v", functionName, cs.componentID, err)
		return err
	}
	logrus.Infof("Queued append function message for function %s, component %s (will be sent to Ignis)", functionName, cs.componentID)
	return nil
}

// handleIgnisMessage 处理从共享 stream 接收到的 Ignis 消息
// 这个方法会被 Service 的 routeMessageToComponent 调用
func (cs *ComponentSession) handleIgnisMessage(msg *ignisctrlpb.Message) {
	// 处理 ReturnResult 和 ResponseObject 类型的消息
	if msg.Type == ignisctrlpb.CommandType_BK_RETURN_RESULT {
		// 处理 ReturnResult 消息
		returnResult := msg.GetReturnResult()
		if returnResult == nil {
			logrus.Warnf("Received message with type BK_RETURN_RESULT but no ReturnResult command for component %s, ignoring", cs.componentID)
			return
		}

		// TODO: 根据 InstanceID 判断消息是否属于当前 component
		// 暂时处理所有消息，后续可以优化为精确路由

		// 将 ReturnResult 消息转换为 iarnet component 消息
		componentMsg, err := cs.convertReturnResultToIarnet(returnResult)
		if err != nil {
			logrus.Errorf("Failed to convert ReturnResult message for component %s: %v", cs.componentID, err)
			return
		}

		// 通过 ZMQ 发送到组件
		data, err := proto.Marshal(componentMsg)
		if err != nil {
			logrus.Errorf("Failed to marshal component message for component %s: %v", cs.componentID, err)
			return
		}

		if cs.zmqSocket != nil {
			// 发送消息到 ZMQ socket
			if err := cs.zmqSocket.SendMessage([][]byte{data}); err != nil {
				logrus.Errorf("Failed to send ZMQ message for component %s: %v", cs.componentID, err)
			}
		}
	} else if msg.Type == ignisctrlpb.CommandType_BK_RESPONSE_OBJECT {
		// 处理 ResponseObject 消息（用于 fetchAndSaveObjectFromIgnis）
		responseObj := msg.GetResponseObject()
		if responseObj == nil {
			logrus.Warnf("Received message with type BK_RESPONSE_OBJECT but no ResponseObject command for component %s, ignoring", cs.componentID)
			return
		}

		// 异步处理 ResponseObject：保存到 store 并发送响应
		go cs.handleResponseObject(responseObj)
	} else {
		logrus.Debugf("Received unsupported message type from Ignis for component %s: type=%v, ignoring", cs.componentID, msg.Type)
	}
}

// convertReturnResultToIarnet 将 Ignis 的 ReturnResult 消息转换为 iarnet component 消息
// 需要先将 ReturnResult 转换为 actor InvokeResponse，然后包装为 component Message
func (cs *ComponentSession) convertReturnResultToIarnet(returnResult *ignisctrlpb.ReturnResult) (*componentpb.Message, error) {
	// 构建完整的 RuntimeID，格式: functionName::sessionID::instanceID
	// 这与 handleInvokeRequest 中解析的格式一致
	runtimeID := fmt.Sprintf("%s::%s::%s",
		returnResult.GetName(),
		returnResult.GetSessionID(),
		returnResult.GetInstanceID())

	// 创建 actor InvokeResponse
	invokeResponse := &actorpb.InvokeResponse{
		RuntimeID: runtimeID,
	}

	// 处理结果：可能是 Value (Data) 或 Error
	if returnResult.GetError() != "" {
		// 有错误
		invokeResponse.Error = returnResult.GetError()
	} else if value := returnResult.GetValue(); value != nil {
		// 有值，需要从 Data 中提取 Flow 并转换为 ObjectRef
		var objectRef *commonpb.ObjectRef

		if ref := value.GetRef(); ref != nil {
			// Data 包含 Flow (Ref)，需要从 Ignis 获取对象并保存到 iarnet store
			// 先返回响应（使用 storeID 作为 source），然后异步获取并保存
			flowID := ref.GetID()

			// 记录原 ref 在 ignis 中的 source（使用全局映射）
			if ref.GetSource() != nil && ref.GetSource().GetID() != "" {
				ignisSourceID := ref.GetSource().GetID()
				cs.service.ignisObjectsMu.Lock()
				cs.service.ignisObjects[flowID] = ignisSourceID
				cs.service.ignisObjectsMu.Unlock()
				logrus.Debugf("Recorded object %s in ignis with source %s for component %s", flowID, ignisSourceID, cs.componentID)
			}

			objectRef = &commonpb.ObjectRef{
				ID:     flowID,
				Source: cs.storeID, // 使用 store ID 作为 source（用于 iarnet 响应）
			}

			// 异步从 Ignis 获取对象并保存到 iarnet store
			go cs.fetchAndSaveObjectFromIgnis(flowID, ref)
		} else if encoded := value.GetEncoded(); encoded != nil {
			// Data 包含 EncodedObject，可以直接保存到 iarnet store
			// 先返回响应（使用 storeID 作为 source），然后异步保存
			objectRef = &commonpb.ObjectRef{
				ID:     encoded.GetID(),
				Source: cs.storeID, // 使用 store ID 作为 source
			}

			// 异步保存到 iarnet store
			go cs.saveEncodedObjectToIarnetStore(encoded)
		}

		invokeResponse.Result = objectRef
	}

	// 创建 actor Message，类型为 INVOKE_RESPONSE
	actorMsg := &actorpb.Message{
		Type: actorpb.MessageType_INVOKE_RESPONSE,
		Message: &actorpb.Message_InvokeResponse{
			InvokeResponse: invokeResponse,
		},
	}

	// 将 actor Message 包装为 Payload
	anyMsg, err := anypb.New(actorMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to create anypb from actor message: %w", err)
	}

	componentMsg := &componentpb.Message{
		Type: componentpb.MessageType_PAYLOAD,
		Message: &componentpb.Message_Payload{
			Payload: anyMsg,
		},
	}

	return componentMsg, nil
}

// receiveFromZMQ 从 ZMQ 接收消息并转换为 Ignis 消息发送
// 参考 containers/component/python/actor.py 的 message_loop 实现
func (cs *ComponentSession) receiveFromZMQ() {
	if cs.zmqSocket == nil {
		logrus.Errorf("ZMQ socket is nil for component %s", cs.componentID)
		return
	}

	logrus.Infof("Starting ZMQ receiver for component %s", cs.componentID)

	// 注意：iarnet 的 ComponentChanneler 使用 ROUTER socket，需要 component 先发送消息
	// 才能被标记为已连接，然后 pending 的消息才会被发送
	// 所以我们需要先发送一个空消息或心跳消息来通知 Channeler 我们已经连接
	// 但是，由于我们使用 DEALER socket，直接接收消息应该也能工作
	// 让我们先尝试直接接收，如果不行再发送心跳

	// 创建 Poller 用于非阻塞接收
	poller, err := goczmq.NewPoller(cs.zmqSocket)
	if err != nil {
		logrus.Errorf("Failed to create ZMQ poller for component %s: %v", cs.componentID, err)
		return
	}
	defer poller.Destroy()

	// 发送 READY 消息来通知 Channeler 我们已经连接（这样 pending 的消息才会被发送）
	// 参考 containers/component/python/actor.py 的实现：
	// ready_msg = component.Message(Type=component.MessageType.READY, Ready=common_messages.Ready())
	readyMsg := &componentpb.Message{
		Type: componentpb.MessageType_READY,
		Message: &componentpb.Message_Ready{
			Ready: &commonpb.Ready{},
		},
	}
	readyData, err := proto.Marshal(readyMsg)
	if err != nil {
		logrus.Errorf("Failed to marshal READY message for component %s: %v", cs.componentID, err)
		return
	}
	if err := cs.zmqSocket.SendMessage([][]byte{readyData}); err != nil {
		logrus.Errorf("Failed to send READY message to ZMQ for component %s: %v", cs.componentID, err)
		return
	}
	logrus.Infof("Sent READY message to ZMQ for component %s to notify connection", cs.componentID)

	for {
		select {
		case <-cs.ctx.Done():
			logrus.Infof("ZMQ receiver stopped for component %s", cs.componentID)
			return
		default:
			// 使用 Poller 非阻塞地检查是否有消息（超时时间 100ms）
			socket := poller.Wait(100)
			if socket == nil {
				// 没有消息，继续循环
				continue
			}

			// 从 ZMQ socket 接收消息
			// goczmq 的 RecvMessage 返回 [][]byte
			// 对于 ROUTER->DEALER 通信，ROUTER 会在消息前添加 identity frame（component ID）
			// 所以消息格式是：[identity_frame, data_frame]
			msgFrames, err := cs.zmqSocket.RecvMessage()
			if err != nil {
				logrus.Errorf("Failed to receive from ZMQ for component %s: %v", cs.componentID, err)
				// 检查是否是上下文取消导致的错误
				select {
				case <-cs.ctx.Done():
					return
				default:
					// 继续重试
					continue
				}
			}

			if len(msgFrames) == 0 {
				logrus.Warnf("Received empty message from ZMQ for component %s", cs.componentID)
				continue
			}

			// 提取实际数据
			// 对于 ROUTER->DEALER 通信：
			// - ROUTER 会在消息前添加 identity frame（component ID）
			// - 所以消息格式是：[identity_frame, data_frame]
			// - 我们需要提取最后一个 frame（数据）
			var data []byte
			if len(msgFrames) == 1 {
				// 只有一个 frame，就是数据
				data = msgFrames[0]
			} else if len(msgFrames) >= 2 {
				// 多个 frame，最后一个通常是数据
				// 第一个可能是 identity（component ID），但我们不需要它
				data = msgFrames[len(msgFrames)-1]
			}

			if len(data) == 0 {
				// 空消息可能是心跳，忽略
				logrus.Debugf("Received empty data from ZMQ for component %s (possibly heartbeat), ignoring", cs.componentID)
				continue
			}

			// 解析为 component.Message
			var componentMsg componentpb.Message
			if err := proto.Unmarshal(data, &componentMsg); err != nil {
				logrus.Errorf("Failed to unmarshal component message from ZMQ for component %s: %v", cs.componentID, err)
				continue
			}

			// 转换为 Ignis 消息并发送
			if err := cs.SendToIgnis(&componentMsg); err != nil {
				logrus.Errorf("Failed to send message to Ignis for component %s: %v", cs.componentID, err)
				continue
			}

			logrus.Debugf("Successfully forwarded ZMQ message to Ignis for component %s", cs.componentID)
		}
	}
}

// SendToIgnis 将 iarnet ZMQ 消息发送到 Ignis
func (cs *ComponentSession) SendToIgnis(componentMsg *componentpb.Message) error {
	// 特殊处理 InvokeRequest：需要为每个参数发送一个 AppendArg 消息
	if payload := componentMsg.GetPayload(); payload != nil {
		var actorMsg actorpb.Message
		if err := anypb.UnmarshalTo(payload, &actorMsg, proto.UnmarshalOptions{}); err == nil {
			if invokeReq := actorMsg.GetInvokeRequest(); invokeReq != nil {
				// 处理 InvokeRequest：为每个参数发送 AppendArg
				return cs.handleInvokeRequest(invokeReq)
			}
		}
	}

	// 将 iarnet component 消息转换为 Ignis 消息
	ignisMsg, err := cs.convertIarnetToIgnis(componentMsg)
	if err != nil {
		return fmt.Errorf("failed to convert iarnet message: %w", err)
	}

	// 如果消息为 nil（例如 Function 消息已经在 Start() 中处理），直接返回
	if ignisMsg == nil {
		return nil
	}

	// 通过消息队列发送，确保消息串行化
	return cs.service.sendMessageToIgnis(cs.componentID, ignisMsg)
}

// convertIarnetToIgnis 将 iarnet component 消息转换为 Ignis 消息
// 使用 ignis-go 的 proto 类型
func (cs *ComponentSession) convertIarnetToIgnis(componentMsg *componentpb.Message) (*ignisctrlpb.Message, error) {
	switch msg := componentMsg.GetMessage().(type) {
	case *componentpb.Message_Ready:
		// Ready 消息
		return ignisctrlpb.NewReady(), nil
	case *componentpb.Message_Payload:
		// 尝试解析 Payload 中的消息
		payload := msg.Payload
		if payload == nil {
			logrus.Warnf("Payload is nil for component %s", cs.componentID)
			return ignisctrlpb.NewReady(), nil
		}

		// 尝试解析为 actor.Message
		var actorMsg actorpb.Message
		if err := anypb.UnmarshalTo(payload, &actorMsg, proto.UnmarshalOptions{}); err != nil {
			logrus.Warnf("Failed to unmarshal Payload as actor.Message for component %s: %v, using default Ready", cs.componentID, err)
			return ignisctrlpb.NewReady(), nil
		}

		// 处理 actor.Message
		switch actorMsgType := actorMsg.GetMessage().(type) {
		case *actorpb.Message_Function:
			// Function 消息：发送 append function 消息到 Ignis
			function := actorMsgType.Function

			// 检查是否已经发送过 append function 消息
			cs.functionMu.Lock()
			if cs.functionSent {
				cs.functionMu.Unlock()
				logrus.Warnf("Function message already sent for component %s, ignoring duplicate function %s", cs.componentID, function.GetName())
				return nil, nil
			}
			cs.functionSent = true
			cs.functionMu.Unlock()

			// 发送 append function 消息
			if err := cs.sendAppendFunction(function); err != nil {
				logrus.Errorf("Failed to send append function for component %s: %v", cs.componentID, err)
				return nil, err
			}
			logrus.Infof("Sent append function %s to ignis for component %s", function.GetName(), cs.componentID)

			// 注意：READY 消息已经在建立连接时发送，这里不需要再发送
			// 返回 nil 表示不需要再发送 Function 消息本身到 Ignis
			return nil, nil
		case *actorpb.Message_InvokeRequest:
			// InvokeRequest 应该由 handleInvokeRequest 处理，这里不应该到达
			// 但如果到达了，记录警告并返回 nil
			logrus.Warnf("InvokeRequest reached convertIarnetToIgnis for component %s, should be handled by handleInvokeRequest", cs.componentID)
			return nil, nil
		default:
			logrus.Warnf("Unknown actor message type for component %s: %T, using default Ready", cs.componentID, actorMsgType)
			return ignisctrlpb.NewReady(), nil
		}
	default:
		logrus.Warnf("Unknown component message type for component %s, using default Ready", cs.componentID)
		return ignisctrlpb.NewReady(), nil
	}
}

// handleInvokeRequest 处理 InvokeRequest，为每个参数发送一个 AppendArg 消息
func (cs *ComponentSession) handleInvokeRequest(invokeReq *actorpb.InvokeRequest) error {
	runtimeID := invokeReq.GetRuntimeID()
	args := invokeReq.GetArgs()

	logrus.Infof("Processing InvokeRequest for component %s: runtimeID=%s, args_count=%d", cs.componentID, runtimeID, len(args))

	if len(args) == 0 {
		logrus.Warnf("InvokeRequest has no args for component %s, runtimeID=%s", cs.componentID, runtimeID)
		return fmt.Errorf("InvokeRequest has no args")
	}

	// 从 RuntimeID 中解析 SessionID 和 InstanceID
	// RuntimeID 格式: functionName::sessionID::instanceID
	splits := strings.SplitN(runtimeID, "::", 3)
	var sessionID, instanceID, functionName string
	if len(splits) >= 3 {
		functionName = splits[0]
		sessionID = splits[1]
		instanceID = splits[2]
		logrus.Debugf("Parsed RuntimeID for component %s: functionName=%s, sessionID=%s, instanceID=%s",
			cs.componentID, functionName, sessionID, instanceID)
	} else {
		// 如果格式不正确，记录警告并使用默认值
		logrus.Warnf("Invalid RuntimeID format for component %s: %s, expected format: functionName::sessionID::instanceID", cs.componentID, runtimeID)
		sessionID = ""
		instanceID = runtimeID // 使用整个 RuntimeID 作为 InstanceID
		functionName = ""
	}

	// 为每个参数发送一个 AppendArg 消息
	for i, arg := range args {
		param := arg.GetParam()
		objectRef := arg.GetValue()

		if objectRef == nil {
			logrus.Warnf("InvokeArg[%d] has no value for component %s, param=%s, skipping", i, cs.componentID, param)
			continue
		}

		objectID := objectRef.GetID()
		logrus.Debugf("Processing arg[%d] for component %s: param=%s, objectID=%s", i, cs.componentID, param, objectID)

		// 检查对象是否在 ignis 中（使用全局映射）
		cs.service.ignisObjectsMu.RLock()
		ignisSourceID, inIgnis := cs.service.ignisObjects[objectID]
		cs.service.ignisObjectsMu.RUnlock()

		var dataValue *ignisctrlpb.Data
		if inIgnis {
			// 对象已在 ignis 中，使用 Ref，并还原其在 ignis 中的 source
			// 如果 source 为空字符串（占位符），说明 source 还未确定，使用 storeID 作为默认值
			if ignisSourceID == "" {
				ignisSourceID = cs.storeID
				logrus.Debugf("Object %s in Ignis but source unknown for component %s, using storeID %s as default", objectID, cs.componentID, ignisSourceID)
			} else {
				logrus.Debugf("Object %s already in Ignis for component %s, using Ref with source %s", objectID, cs.componentID, ignisSourceID)
			}
			dataValue = &ignisctrlpb.Data{
				Type: ignisctrlpb.Data_OBJ_REF,
				Object: &ignisctrlpb.Data_Ref{
					Ref: &ignisprotopb.Flow{
						ID: objectID,
						Source: &ignisprotopb.StoreRef{
							ID: ignisSourceID, // 还原为对象在 ignis 中的 source
						},
					},
				},
			}
		} else {
			// 对象不在 ignis 中，需要从 iarnet store 获取并随 AppendArg 一起传递
			logrus.Debugf("Object %s not in Ignis for component %s, fetching from store", objectID, cs.componentID)
			ignisEncoded, err := cs.fetchObjectFromStore(objectRef)
			if err != nil {
				logrus.Errorf("Failed to fetch object %s from store for component %s, arg[%d]: %v", objectID, cs.componentID, i, err)
				return fmt.Errorf("failed to fetch object %s: %w", objectID, err)
			}

			// 检查目标函数是否是 Go 函数，如果是且对象是 Python 编码，则转换为 JSON
			cs.deployedFunctionMu.RLock()
			deployedName := cs.deployedFunctionName
			targetFunctionLanguage := cs.deployedFunctionLanguage
			cs.deployedFunctionMu.RUnlock()

			if deployedName == functionName && targetFunctionLanguage == commonpb.Language_LANG_GO {
				// 目标函数是 Go 函数，检查对象编码格式
				if ignisEncoded.GetLanguage() == ignisprotopb.Language_LANG_PYTHON {
					logrus.Infof("Object %s is Python-encoded but target function %s is Go, converting to JSON",
						objectID, functionName)
					convertedEncoded, err := cs.convertPythonEncodedObjectToJSON(ignisEncoded)
					if err != nil {
						logrus.Errorf("Failed to convert Python object %s to JSON for component %s, arg[%d]: %v",
							objectID, cs.componentID, i, err)
						return fmt.Errorf("failed to convert Python object %s to JSON: %w", objectID, err)
					}
					ignisEncoded = convertedEncoded
					logrus.Infof("Successfully converted object %s from Python to JSON for Go function %s",
						objectID, functionName)
				}
			}

			// 记录对象已保存到 ignis（通过 AppendArg 传递）
			// 注意：此时还不知道对象在 ignis 中的 source，会在后续 ReturnResult 中更新
			// 暂时使用空字符串作为占位符，表示对象已在 ignis 中但 source 未知（使用全局映射）
			cs.service.ignisObjectsMu.Lock()
			cs.service.ignisObjects[ignisEncoded.GetID()] = "" // 占位符，后续会更新
			cs.service.ignisObjectsMu.Unlock()

			// 使用 Encoded 类型，直接传递对象数据
			dataValue = &ignisctrlpb.Data{
				Type: ignisctrlpb.Data_OBJ_ENCODED,
				Object: &ignisctrlpb.Data_Encoded{
					Encoded: ignisEncoded,
				},
			}
			logrus.Debugf("Fetched object %s from store for component %s, encoded_size=%d, language=%v",
				objectID, cs.componentID, len(ignisEncoded.GetData()), ignisEncoded.GetLanguage())
		}

		// 创建 AppendArg 消息
		appendArg := &ignisctrlpb.AppendArg{
			SessionID:  sessionID,
			InstanceID: instanceID,
			Name:       functionName,
			Param:      param,
			Value:      dataValue,
		}

		appendArgMsg := &ignisctrlpb.Message{
			Type: ignisctrlpb.CommandType_FR_APPEND_ARG,
			Command: &ignisctrlpb.Message_AppendArg{
				AppendArg: appendArg,
			},
		}

		// 通过消息队列发送，确保消息串行化
		if err := cs.service.sendMessageToIgnis(cs.componentID, appendArgMsg); err != nil {
			logrus.Errorf("Failed to send AppendArg[%d] to Ignis for component %s: %v", i, cs.componentID, err)
			return fmt.Errorf("failed to send AppendArg[%d]: %w", i, err)
		}
		logrus.Infof("Sent AppendArg[%d] to Ignis for component %s: runtimeID=%s, param=%s, objectID=%s",
			i, cs.componentID, runtimeID, param, objectID)
	}

	// 发送完所有 AppendArg 后，发送 Invoke 消息触发函数调用
	invokeMsg := &ignisctrlpb.Message{
		Type: ignisctrlpb.CommandType_FR_INVOKE,
		Command: &ignisctrlpb.Message_Invoke{
			Invoke: &ignisctrlpb.Invoke{
				SessionID:  sessionID,
				InstanceID: instanceID,
				Name:       functionName,
			},
		},
	}

	if err := cs.service.sendMessageToIgnis(cs.componentID, invokeMsg); err != nil {
		logrus.Errorf("Failed to send Invoke message to Ignis for component %s: %v", cs.componentID, err)
		return fmt.Errorf("failed to send Invoke message: %w", err)
	}
	logrus.Infof("Sent Invoke message to Ignis for component %s: runtimeID=%s, functionName=%s, sessionID=%s, instanceID=%s",
		cs.componentID, runtimeID, functionName, sessionID, instanceID)

	// 记录路由映射：sessionID::instanceID -> componentID
	// 这样当 Ignis 返回 ReturnResult 时，可以精确路由到对应的 component
	if sessionID != "" && instanceID != "" {
		routingKey := fmt.Sprintf("%s::%s", sessionID, instanceID)
		cs.service.routingMu.Lock()
		cs.service.routingMap[routingKey] = cs.componentID
		cs.service.routingMu.Unlock()
		logrus.Debugf("Recorded routing: %s -> component %s", routingKey, cs.componentID)
	}

	logrus.Infof("Successfully processed InvokeRequest for component %s: runtimeID=%s, sent %d AppendArg messages and 1 Invoke message",
		cs.componentID, runtimeID, len(args))
	return nil
}

// fetchObjectFromStore 从 iarnet store 获取对象并转换为 Ignis 的 EncodedObject
func (cs *ComponentSession) fetchObjectFromStore(objectRef *commonpb.ObjectRef) (*ignisprotopb.EncodedObject, error) {
	if cs.storeAddr == "" {
		return nil, fmt.Errorf("store address not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 连接到 iarnet store
	conn, err := grpc.NewClient(cs.storeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to store: %w", err)
	}
	defer conn.Close()

	storeClient := storepb.NewServiceClient(conn)

	// 从 store 获取对象
	encodedObj, err := storeClient.GetObject(ctx, &storepb.GetObjectRequest{
		ObjectRef: objectRef,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from store: %w", err)
	}

	if encodedObj.GetObject() == nil {
		return nil, fmt.Errorf("object not found in store")
	}

	// 转换为 Ignis 的 EncodedObject
	ignisEncoded := &ignisprotopb.EncodedObject{
		ID:       encodedObj.GetObject().GetID(),
		Data:     encodedObj.GetObject().GetData(),
		Language: ignisprotopb.Language(encodedObj.GetObject().GetLanguage()),
	}

	logrus.Debugf("Fetched object %s from store for component %s", ignisEncoded.GetID(), cs.componentID)
	return ignisEncoded, nil
}

// convertPythonToJSON 将 Python pickle 编码的对象转换为 JSON 格式
// 通过调用 Python 脚本实现转换
func convertPythonToJSON(pickleData []byte) ([]byte, error) {
	// 将 pickle 数据编码为 base64，以便通过命令行传递
	pickleBase64 := base64.StdEncoding.EncodeToString(pickleData)

	// Python 脚本：解码 pickle 并转换为 JSON
	pythonScript := `
import sys
import base64
import json
import cloudpickle

try:
    # 从 base64 解码 pickle 数据
    pickle_data = base64.b64decode(sys.argv[1])
    
    # 使用 cloudpickle 解码对象
    obj = cloudpickle.loads(pickle_data)
    
    # 转换为 JSON（只支持 JSON 兼容的类型）
    json_str = json.dumps(obj, default=str)
    
    # 输出 JSON 字符串
    print(json_str)
except Exception as e:
    print(f"ERROR: {e}", file=sys.stderr)
    sys.exit(1)
`

	// 执行 Python 脚本
	cmd := exec.Command("python3", "-c", pythonScript, pickleBase64)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Failed to convert Python pickle to JSON: %v, output: %s", err, string(output))
		return nil, fmt.Errorf("python conversion failed: %w, output: %s", err, string(output))
	}

	// 检查输出是否包含错误
	outputStr := strings.TrimSpace(string(output))
	if strings.HasPrefix(outputStr, "ERROR:") {
		return nil, fmt.Errorf("python conversion error: %s", outputStr)
	}

	// 返回 JSON 字节
	return []byte(outputStr), nil
}

// convertPythonEncodedObjectToJSON 将 Python 编码的 EncodedObject 转换为 JSON 编码
func (cs *ComponentSession) convertPythonEncodedObjectToJSON(encoded *ignisprotopb.EncodedObject) (*ignisprotopb.EncodedObject, error) {
	if encoded.GetLanguage() != ignisprotopb.Language_LANG_PYTHON {
		return encoded, nil // 不是 Python 编码，直接返回
	}

	logrus.Infof("Converting Python-encoded object %s to JSON for component %s", encoded.GetID(), cs.componentID)

	// 转换 pickle 到 JSON
	jsonData, err := convertPythonToJSON(encoded.GetData())
	if err != nil {
		return nil, fmt.Errorf("failed to convert Python object to JSON: %w", err)
	}

	// 创建新的 JSON 编码对象
	jsonEncoded := &ignisprotopb.EncodedObject{
		ID:       encoded.GetID(),
		Data:     jsonData,
		Language: ignisprotopb.Language_LANG_JSON, // 改为 JSON 编码
	}

	logrus.Infof("Successfully converted object %s from Python to JSON for component %s, json_size=%d",
		encoded.GetID(), cs.componentID, len(jsonData))
	return jsonEncoded, nil
}

// fetchAndSaveObjectFromIgnis 异步从 Ignis 获取对象并保存到 iarnet store
func (cs *ComponentSession) fetchAndSaveObjectFromIgnis(flowID string, flow *ignisprotopb.Flow) {
	if cs.storeAddr == "" {
		logrus.Warnf("Store address not configured for component %s, skipping object fetch", cs.componentID)
		return
	}

	// 向 Ignis 发送 RequestObject 请求
	requestMsg := &ignisctrlpb.Message{
		Type: ignisctrlpb.CommandType_FR_REQUEST_OBJECT,
		Command: &ignisctrlpb.Message_RequestObject{
			RequestObject: &ignisctrlpb.RequestObject{
				ID:     flowID,
				Target: "", // 可以指定目标，留空表示默认
			},
		},
	}

	if err := cs.service.sendMessageToIgnis(cs.componentID, requestMsg); err != nil {
		logrus.Errorf("Failed to send RequestObject to Ignis for component %s, flowID %s: %v", cs.componentID, flowID, err)
		return
	}

	logrus.Infof("Requested object %s from Ignis for component %s, waiting for ResponseObject", flowID, cs.componentID)
	// ResponseObject 将在 handleIgnisMessage 中通过 handleResponseObject 处理
}

// handleResponseObject 处理从 Ignis 返回的 ResponseObject 消息，保存到 iarnet store
func (cs *ComponentSession) handleResponseObject(responseObj *ignisctrlpb.ResponseObject) {
	if cs.storeAddr == "" {
		logrus.Warnf("Store address not configured for component %s, skipping object save", cs.componentID)
		return
	}

	if responseObj.GetError() != "" {
		logrus.Errorf("Received error from Ignis for object %s: %s", responseObj.GetID(), responseObj.GetError())
		return
	}

	encoded := responseObj.GetValue()
	if encoded == nil {
		logrus.Warnf("Received ResponseObject with no value for object %s", responseObj.GetID())
		return
	}

	// 记录对象已保存在 ignis 中
	// 注意：ResponseObject 不包含 source 信息，所以暂时使用空字符串作为占位符
	// 如果后续有 ReturnResult 返回该对象的 Ref，会更新 source
	objectID := encoded.GetID()
	if objectID != "" {
		cs.service.ignisObjectsMu.Lock()
		// 如果对象已存在但 source 为空，保持空字符串（占位符）
		// 如果对象不存在，添加空字符串作为占位符（使用全局映射）
		if _, exists := cs.service.ignisObjects[objectID]; !exists {
			cs.service.ignisObjects[objectID] = "" // 占位符，后续会更新
		}
		cs.service.ignisObjectsMu.Unlock()
		logrus.Debugf("Recorded object %s in ignis for component %s (source to be updated)", objectID, cs.componentID)
	}

	// 异步保存到 iarnet store
	go cs.saveEncodedObjectToIarnetStore(encoded)
}

// saveEncodedObjectToIarnetStore 异步将 EncodedObject 保存到 iarnet store
// 注意：响应已经先返回了，这里只是保存对象
// 为了避免重复保存，先检查对象是否已存在
func (cs *ComponentSession) saveEncodedObjectToIarnetStore(encoded *ignisprotopb.EncodedObject) {
	if cs.storeAddr == "" {
		logrus.Warnf("Store address not configured for component %s, skipping object save", cs.componentID)
		return
	}

	objectID := encoded.GetID()
	if objectID == "" {
		logrus.Warnf("Object ID is empty, skipping save for component %s", cs.componentID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 连接到 iarnet store
	conn, err := grpc.NewClient(cs.storeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("Failed to connect to store for component %s: %v", cs.componentID, err)
		return
	}
	defer conn.Close()

	storeClient := storepb.NewServiceClient(conn)

	// 先检查对象是否已存在，避免重复保存
	// 使用较短的超时时间，因为这只是检查操作
	checkCtx, checkCancel := context.WithTimeout(ctx, 2*time.Second)
	defer checkCancel()

	_, err = storeClient.GetObject(checkCtx, &storepb.GetObjectRequest{
		ObjectRef: &commonpb.ObjectRef{
			ID: objectID,
		},
	})
	if err == nil {
		// 对象已存在，不需要重复保存
		logrus.Debugf("Object %s already exists in store, skipping save for component %s", objectID, cs.componentID)
		return
	}
	// 如果错误是"对象不存在"，继续保存；其他错误也继续尝试保存（可能是网络问题）

	// 将 Ignis 的 EncodedObject 转换为 iarnet 的 EncodedObject
	// 注意：需要根据实际的 proto 定义进行转换
	encodedObj := &commonpb.EncodedObject{
		ID:       encoded.GetID(),
		Data:     encoded.GetData(),
		Language: commonpb.Language(encoded.GetLanguage()), // 需要确认类型转换
	}

	// 保存到 store
	objectRef, err := storeClient.SaveObject(ctx, &storepb.SaveObjectRequest{
		Object: encodedObj,
	})
	if err != nil {
		// 如果保存失败，可能是对象已被其他 component 保存（并发情况）
		// 检查是否是"已存在"的错误
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate") {
			logrus.Debugf("Object %s was already saved by another component, skipping for component %s", objectID, cs.componentID)
			return
		}
		logrus.Errorf("Failed to save object %s to store for component %s: %v", objectID, cs.componentID, err)
		return
	}

	if objectRef != nil && objectRef.ObjectRef != nil {
		logrus.Infof("Successfully saved object %s to iarnet store for component %s, new ref: %s", encoded.GetID(), cs.componentID, objectRef.ObjectRef.ID)
	} else {
		logrus.Infof("Successfully saved object %s to iarnet store for component %s", encoded.GetID(), cs.componentID)
	}
}

// Stop 停止组件会话
func (cs *ComponentSession) Stop() {
	cs.cancel()
	cs.wg.Wait()
	// 注意：不再需要关闭 stream，因为使用的是共享的 stream
	if cs.zmqSocket != nil {
		cs.zmqSocket.Destroy()
		cs.zmqSocket = nil
	}
}
