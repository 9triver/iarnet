package provider

import (
	"context"
	"fmt"
	"io"
	"net"
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

	// 组件会话管理：componentID -> session
	componentSessions map[string]*ComponentSession
	sessionsMu        sync.RWMutex
}

// ComponentSession 管理一个组件在 ignis 中的 gRPC stream 会话
type ComponentSession struct {
	componentID  string
	stream       grpc.BidiStreamingClient[ignisctrlpb.Message, ignisctrlpb.Message] // 使用 ignis-go 的 Message 类型
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	zmqSocket    *goczmq.Sock // 每个 component 独立的 ZMQ socket（用于发送和接收）
	ignisClient  ignisctrlpb.ServiceClient
	storeAddr    string          // iarnet store 地址
	storeID      string          // iarnet store ID（用于在响应中设置 source 字段）
	ignisObjects map[string]bool // 记录 ignis 中保存的对象 ID（用于判断是否需要从 iarnet store 获取）
	objectsMu    sync.RWMutex    // 保护 ignisObjects 的锁
	functionSent bool            // 标记是否已发送 append function 消息
	functionMu   sync.Mutex      // 保护 functionSent 的锁
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
		allocated: &resourcepb.Info{
			Cpu:    0,
			Memory: 0,
			Gpu:    0,
		},
		componentSessions: make(map[string]*ComponentSession),
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

// createComponentSession 为组件创建到 Ignis 的 gRPC stream 会话
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

	// 创建新的 gRPC stream
	// 注意：使用 context.Background() 而不是请求的 ctx，因为 stream 需要在 Deploy() 返回后继续运行
	streamCtx, cancel := context.WithCancel(context.Background())
	stream, err := s.ignisClient.Session(streamCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create ignis session: %w", err)
	}

	// 立即发送 READY 消息，通知 Ignis 连接已就绪
	readyMsg := ignisctrlpb.NewReady()
	if err := stream.Send(readyMsg); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to send READY message to ignis: %w", err)
	}
	logrus.Debugf("Sent initial READY message to ignis for component %s", componentID)

	// 为每个 component 创建独立的 ZMQ socket
	zmqSocket, err := s.createZMQSocket(componentID, zmqAddr)
	if err != nil {
		cancel()
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

	session := &ComponentSession{
		componentID:  componentID,
		stream:       stream,
		ctx:          streamCtx,
		cancel:       cancel,
		zmqSocket:    zmqSocket,
		ignisClient:  s.ignisClient,
		storeAddr:    storeAddr,
		storeID:      storeID,
		ignisObjects: make(map[string]bool),
		functionSent: false,
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
func (cs *ComponentSession) Start() {
	cs.wg.Add(2)

	// 启动从 Ignis 接收消息的 goroutine
	go func() {
		defer cs.wg.Done()
		cs.receiveFromIgnis()
	}()

	// 启动从 ZMQ 接收消息的 goroutine
	go func() {
		defer cs.wg.Done()
		cs.receiveFromZMQ()
	}()
}

// sendAppendFunction 根据 Function 的语言类型发送相应的 append function 消息
func (cs *ComponentSession) sendAppendFunction(function *actorpb.Function) error {
	language := function.GetLanguage()
	var appendFuncMsg *ignisctrlpb.Message

	switch language {
	case commonpb.Language_LANG_PYTHON:
		// 发送 AppendPyFunc 消息
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
	case commonpb.Language_LANG_GO:
		// 发送 AppendGo 消息
		appendGo := &ignisctrlpb.AppendGo{
			Name:     function.GetName(),
			Params:   function.GetParams(),
			Code:     string(function.GetPickledObject()), // Go 代码存储在 PickledObject 中
			Language: ignisprotopb.Language(language),
			Replicas: 1, // 默认 1 个副本
		}
		appendFuncMsg = &ignisctrlpb.Message{
			Type: ignisctrlpb.CommandType_FR_APPEND_GO,
			Command: &ignisctrlpb.Message_AppendGo{
				AppendGo: appendGo,
			},
		}
	default:
		return fmt.Errorf("unsupported language: %v", language)
	}

	return cs.stream.Send(appendFuncMsg)
}

// receiveFromIgnis 从 Ignis 接收消息并转换为 ZMQ 消息
// 只处理 ReturnResult 类型的消息，其他消息输出警告
func (cs *ComponentSession) receiveFromIgnis() {
	logrus.Infof("Starting Ignis receiver for component %s", cs.componentID)
	defer logrus.Infof("Ignis receiver stopped for component %s", cs.componentID)

	for {
		// 使用 goroutine 和 channel 来非阻塞地接收消息，同时监听 context 取消
		type recvResult struct {
			msg *ignisctrlpb.Message
			err error
		}
		recvCh := make(chan recvResult, 1)

		// 在 goroutine 中调用阻塞的 Recv()
		go func() {
			msg, err := cs.stream.Recv()
			recvCh <- recvResult{msg: msg, err: err}
		}()

		select {
		case <-cs.ctx.Done():
			logrus.Debugf("Context canceled for component %s, stopping Ignis receiver", cs.componentID)
			return
		case result := <-recvCh:
			if result.err == io.EOF {
				logrus.Infof("Ignis stream closed for component %s", cs.componentID)
				return
			}
			if result.err != nil {
				logrus.Errorf("Failed to receive from ignis for component %s: %v", cs.componentID, result.err)
				return
			}
			msg := result.msg

			// 处理 ReturnResult 和 ResponseObject 类型的消息
			if msg.Type == ignisctrlpb.CommandType_BK_RETURN_RESULT {
				// 处理 ReturnResult 消息
				returnResult := msg.GetReturnResult()
				if returnResult == nil {
					logrus.Warnf("Received message with type BK_RETURN_RESULT but no ReturnResult command for component %s, ignoring", cs.componentID)
					continue
				}

				// 将 ReturnResult 消息转换为 iarnet component 消息
				componentMsg, err := cs.convertReturnResultToIarnet(returnResult)
				if err != nil {
					logrus.Errorf("Failed to convert ReturnResult message for component %s: %v", cs.componentID, err)
					continue
				}

				// 通过 ZMQ 发送到组件
				data, err := proto.Marshal(componentMsg)
				if err != nil {
					logrus.Errorf("Failed to marshal component message for component %s: %v", cs.componentID, err)
					continue
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
					continue
				}

				// 异步处理 ResponseObject：保存到 store 并发送响应
				go cs.handleResponseObject(responseObj)
			} else {
				logrus.Warnf("Received unsupported message type from Ignis for component %s: type=%v, ignoring", cs.componentID, msg.Type)
				continue
			}

		}
	}
}

// convertReturnResultToIarnet 将 Ignis 的 ReturnResult 消息转换为 iarnet component 消息
// 需要先将 ReturnResult 转换为 actor InvokeResponse，然后包装为 component Message
func (cs *ComponentSession) convertReturnResultToIarnet(returnResult *ignisctrlpb.ReturnResult) (*componentpb.Message, error) {
	// 创建 actor InvokeResponse
	invokeResponse := &actorpb.InvokeResponse{
		RuntimeID: returnResult.GetInstanceID(), // 使用 InstanceID 作为 RuntimeID
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
			objectRef = &commonpb.ObjectRef{
				ID:     flowID,
				Source: cs.storeID, // 使用 store ID 作为 source
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

	// 创建 Poller 用于非阻塞接收
	poller, err := goczmq.NewPoller(cs.zmqSocket)
	if err != nil {
		logrus.Errorf("Failed to create ZMQ poller for component %s: %v", cs.componentID, err)
		return
	}
	defer poller.Destroy()

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
			// goczmq 的 RecvMessage 返回 [][]byte，第一个 frame 是 identity（如果有），后续是数据
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

			// 提取实际数据（最后一个 frame 是数据，前面的可能是 identity）
			var data []byte
			if len(msgFrames) > 0 {
				data = msgFrames[len(msgFrames)-1]
			}

			if len(data) == 0 {
				logrus.Warnf("Received message with empty data from ZMQ for component %s", cs.componentID)
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
	// 将 iarnet component 消息转换为 Ignis 消息
	ignisMsg, err := cs.convertIarnetToIgnis(componentMsg)
	if err != nil {
		return fmt.Errorf("failed to convert iarnet message: %w", err)
	}

	// 如果消息为 nil（例如 Function 消息已经在 Start() 中处理），直接返回
	if ignisMsg == nil {
		return nil
	}

	return cs.stream.Send(ignisMsg)
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
			// InvokeRequest 需要转换为多个 AppendArg 消息
			// 注意：这里只返回第一个 AppendArg，实际应该发送多个消息
			// 为了简化，这里先处理第一个参数，后续可以扩展为发送多个消息
			invokeReq := actorMsgType.InvokeRequest
			if len(invokeReq.GetArgs()) == 0 {
				logrus.Warnf("InvokeRequest has no args for component %s", cs.componentID)
				return ignisctrlpb.NewReady(), nil
			}

			// 处理第一个参数（实际应该处理所有参数）
			arg := invokeReq.GetArgs()[0]
			objectRef := arg.GetValue()
			if objectRef == nil {
				logrus.Warnf("InvokeArg has no value for component %s", cs.componentID)
				return ignisctrlpb.NewReady(), nil
			}

			// 检查对象是否在 ignis 中
			objectID := objectRef.GetID()
			cs.objectsMu.RLock()
			inIgnis := cs.ignisObjects[objectID]
			cs.objectsMu.RUnlock()

			var dataValue *ignisctrlpb.Data
			if inIgnis {
				// 对象已在 ignis 中，使用 Ref
				dataValue = &ignisctrlpb.Data{
					Type: ignisctrlpb.Data_OBJ_REF,
					Object: &ignisctrlpb.Data_Ref{
						Ref: &ignisprotopb.Flow{
							ID: objectID,
							Source: &ignisprotopb.StoreRef{
								ID: cs.storeID, // 使用 store ID 作为 source
							},
						},
					},
				}
			} else {
				// 对象不在 ignis 中，需要从 iarnet store 获取并随 AppendArg 一起传递
				ignisEncoded, err := cs.fetchObjectFromStore(objectRef)
				if err != nil {
					logrus.Errorf("Failed to fetch object %s from store for component %s: %v", objectID, cs.componentID, err)
					return nil, err
				}

				// 记录对象已保存到 ignis（通过 AppendArg 传递）
				cs.objectsMu.Lock()
				cs.ignisObjects[ignisEncoded.GetID()] = true
				cs.objectsMu.Unlock()

				// 使用 Encoded 类型，直接传递对象数据
				dataValue = &ignisctrlpb.Data{
					Type: ignisctrlpb.Data_OBJ_ENCODED,
					Object: &ignisctrlpb.Data_Encoded{
						Encoded: ignisEncoded,
					},
				}
			}

			// 从 RuntimeID 中解析 SessionID 和 InstanceID
			// RuntimeID 格式: functionName::sessionID::instanceID
			runtimeID := invokeReq.GetRuntimeID()
			splits := strings.SplitN(runtimeID, "::", 3)
			var sessionID, instanceID, functionName string
			if len(splits) >= 3 {
				functionName = splits[0]
				sessionID = splits[1]
				instanceID = splits[2]
			} else {
				// 如果格式不正确，记录警告并使用默认值
				logrus.Warnf("Invalid RuntimeID format for component %s: %s, expected format: functionName::sessionID::instanceID", cs.componentID, runtimeID)
				sessionID = ""
				instanceID = runtimeID // 使用整个 RuntimeID 作为 InstanceID
				functionName = ""
			}

			// 创建 AppendArg 消息
			appendArg := &ignisctrlpb.AppendArg{
				SessionID:  sessionID,
				InstanceID: instanceID,
				Name:       functionName,
				Param:      arg.GetParam(),
				Value:      dataValue,
			}

			return &ignisctrlpb.Message{
				Type: ignisctrlpb.CommandType_FR_APPEND_ARG,
				Command: &ignisctrlpb.Message_AppendArg{
					AppendArg: appendArg,
				},
			}, nil
		default:
			logrus.Warnf("Unknown actor message type for component %s: %T, using default Ready", cs.componentID, actorMsgType)
			return ignisctrlpb.NewReady(), nil
		}
	default:
		logrus.Warnf("Unknown component message type for component %s, using default Ready", cs.componentID)
		return ignisctrlpb.NewReady(), nil
	}
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

	if err := cs.stream.Send(requestMsg); err != nil {
		logrus.Errorf("Failed to send RequestObject to Ignis for component %s, flowID %s: %v", cs.componentID, flowID, err)
		return
	}

	logrus.Infof("Requested object %s from Ignis for component %s, waiting for ResponseObject", flowID, cs.componentID)
	// ResponseObject 将在 receiveFromIgnis 中通过 handleResponseObject 处理
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
	objectID := encoded.GetID()
	if objectID != "" {
		cs.objectsMu.Lock()
		cs.ignisObjects[objectID] = true
		cs.objectsMu.Unlock()
		logrus.Debugf("Recorded object %s in ignis for component %s", objectID, cs.componentID)
	}

	// 异步保存到 iarnet store
	go cs.saveEncodedObjectToIarnetStore(encoded)
}

// saveEncodedObjectToIarnetStore 异步将 EncodedObject 保存到 iarnet store
// 注意：响应已经先返回了，这里只是保存对象
func (cs *ComponentSession) saveEncodedObjectToIarnetStore(encoded *ignisprotopb.EncodedObject) {
	if cs.storeAddr == "" {
		logrus.Warnf("Store address not configured for component %s, skipping object save", cs.componentID)
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
		logrus.Errorf("Failed to save object to store for component %s: %v", cs.componentID, err)
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
	if cs.stream != nil {
		cs.stream.CloseSend()
	}
	if cs.zmqSocket != nil {
		cs.zmqSocket.Destroy()
		cs.zmqSocket = nil
	}
}
