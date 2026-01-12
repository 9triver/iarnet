package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	commonpb "github.com/9triver/iarnet/internal/proto/common"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
	resourcepb "github.com/9triver/iarnet/internal/proto/resource"
	componentpb "github.com/9triver/iarnet/internal/proto/resource/component"
	reslogger "github.com/9triver/iarnet/internal/proto/resource/logger"
	providerpb "github.com/9triver/iarnet/internal/proto/resource/provider"
	storepb "github.com/9triver/iarnet/internal/proto/resource/store"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"gopkg.in/zeromq/goczmq.v4"
)

const providerType = "unikernel"

// UnikernelMessage unikernel 消息格式
type UnikernelMessage struct {
	CorrID string          `json:"corr_id"`
	Topic  string          `json:"topic"`
	Value  json.RawMessage `json:"value,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Service Unikernel provider 服务实现
type Service struct {
	providerpb.UnimplementedServiceServer
	mu           sync.RWMutex
	manager      *Manager
	resourceTags *providerpb.ResourceTags

	// 资源容量管理
	totalCapacity *resourcepb.Info
	allocated     *resourcepb.Info

	// 支持的语言列表（只支持 unikernel）
	supportedLanguages []commonpb.Language

	// DNS 配置
	dnsHosts map[string]string

	// 组件会话管理
	componentSessions map[string]*ComponentSession
	sessionsMu        sync.RWMutex

	// WebSocket 服务器
	wsServer   *http.Server
	wsUpgrader websocket.Upgrader
	wsPort     int // WebSocket 服务器端口

	// Unikernel 构建相关
	unikernelBaseDir string // unikernel 代码基础目录（mirage-websocket submodule 所在目录）
	solo5HvtPath     string // solo5-hvt 可执行文件路径，为空则从 PATH 中查找

	// 构建锁：确保同一时间只有一个组件在构建（避免 OPAM 锁冲突）
	buildMu sync.Mutex

	// 关闭锁：确保 Close() 只执行一次
	closeOnce sync.Once
}

// ComponentSession 管理一个组件的会话
type ComponentSession struct {
	componentID              string
	service                  *Service
	ctx                      context.Context
	cancel                   context.CancelFunc
	wg                       sync.WaitGroup
	zmqSocket                *goczmq.Sock    // ZMQ socket（用于与 iarnet 通信）
	wsConn                   *websocket.Conn // WebSocket 连接（用于与 unikernel 通信）
	wsConnMu                 sync.RWMutex    // 保护 wsConn 的锁
	storeAddr                string
	storeID                  string
	functionSent             bool
	functionMu               sync.Mutex
	deployedFunctionName     string
	deployedFunctionLanguage commonpb.Language
	deployedFunctionMu       sync.RWMutex
	unikernelProcess         *exec.Cmd                         // unikernel 进程
	unikernelProcessMu       sync.Mutex                        // 保护 unikernelProcess 的锁
	workDir                  string                            // 工作目录（用于构建和运行 unikernel）
	pendingInvokes           map[string]chan *UnikernelMessage // corr_id -> response channel
	pendingInvokesMu         sync.RWMutex
	loggerCollector          *LoggerCollector // 日志收集器
}

// NewService 创建新的 Unikernel provider 服务
func NewService(resourceTags []string, totalCapacity *resourcepb.Info, supportedLanguages []string, dnsHosts map[string]string, wsPort int, unikernelBaseDir string, solo5HvtPath string) (*Service, error) {
	// 创建健康检查管理器
	manager := NewManager(
		90*time.Second,
		10*time.Second,
		func() {
			logrus.Debug("Provider ID cleared due to health check timeout")
		},
	)

	// 转换支持的语言列表（只支持 unikernel）
	languages := make([]commonpb.Language, 0, len(supportedLanguages))
	for _, langStr := range supportedLanguages {
		var lang commonpb.Language
		switch langStr {
		case "unikernel":
			lang = commonpb.Language_LANG_UNIKERNEL
		default:
			lang = commonpb.Language_LANG_UNKNOWN
		}
		if lang != commonpb.Language_LANG_UNKNOWN {
			languages = append(languages, lang)
		}
	}
	if len(languages) == 0 {
		languages = []commonpb.Language{commonpb.Language_LANG_UNIKERNEL}
		logrus.Info("No supported languages configured, defaulting to Unikernel")
	}

	// 确定 unikernel 基础目录（mirage-websocket submodule 所在目录）
	if unikernelBaseDir == "" {
		// 默认使用 providers/unikernel 目录（mirage-websocket 在这里）
		// 尝试从当前工作目录或可执行文件位置推断
		cwd, err := os.Getwd()
		if err == nil {
			// 尝试从当前工作目录查找
			candidate := filepath.Join(cwd, "providers", "unikernel")
			if _, err := os.Stat(filepath.Join(candidate, "mirage-websocket")); err == nil {
				unikernelBaseDir = candidate
			} else {
				// 如果当前目录就是 providers/unikernel
				if _, err := os.Stat(filepath.Join(cwd, "mirage-websocket")); err == nil {
					unikernelBaseDir = cwd
				} else {
					// 回退到相对路径
					unikernelBaseDir = filepath.Join("providers", "unikernel")
				}
			}
		} else {
			// 如果无法获取工作目录，使用相对路径
			unikernelBaseDir = filepath.Join("providers", "unikernel")
		}
	}

	// 确保路径是绝对路径
	if !filepath.IsAbs(unikernelBaseDir) {
		absPath, err := filepath.Abs(unikernelBaseDir)
		if err == nil {
			unikernelBaseDir = absPath
		}
	}

	service := &Service{
		manager:            manager,
		resourceTags:       &providerpb.ResourceTags{Cpu: contains(resourceTags, "cpu"), Memory: contains(resourceTags, "memory"), Gpu: contains(resourceTags, "gpu"), Camera: contains(resourceTags, "camera")},
		totalCapacity:      totalCapacity,
		supportedLanguages: languages,
		dnsHosts:           dnsHosts,
		allocated:          &resourcepb.Info{Cpu: 0, Memory: 0, Gpu: 0},
		componentSessions:  make(map[string]*ComponentSession),
		wsPort:             wsPort,
		unikernelBaseDir:   unikernelBaseDir,
		solo5HvtPath:       solo5HvtPath,
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源
			},
		},
	}

	// 启动健康检测超时监控
	manager.Start()

	// 启动 WebSocket 服务器
	if err := service.startWebSocketServer(); err != nil {
		return nil, fmt.Errorf("failed to start WebSocket server: %w", err)
	}

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

// startWebSocketServer 启动 WebSocket 服务器
func (s *Service) startWebSocketServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)

	// 监听所有接口（包括 IPv4 和 IPv6），确保 tap100 网络可以访问
	// 使用 0.0.0.0 明确指定 IPv4，避免只监听 IPv6
	addr := fmt.Sprintf("0.0.0.0:%d", s.wsPort)
	s.wsServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  0, // 不设置读取超时，保持连接打开
		WriteTimeout: 0, // 不设置写入超时，保持连接打开
		IdleTimeout:  0, // 不设置空闲超时，保持连接打开
	}

	go func() {
		logrus.Infof("Starting WebSocket server on %s (listening on all interfaces including tap100)", addr)
		if err := s.wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("WebSocket server error: %v", err)
		}
	}()

	return nil
}

// handleWebSocket 处理 WebSocket 连接
func (s *Service) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 从请求头获取 executor_id（unikernel 连接时会发送）
	executorID := r.Header.Get("executor_id")
	if executorID == "" {
		logrus.Warn("WebSocket connection without executor_id, rejecting")
		http.Error(w, "executor_id header required", http.StatusBadRequest)
		return
	}

	// 升级到 WebSocket
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Errorf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	logrus.Infof("WebSocket connection established for executor_id: %s", executorID)

	// 查找对应的 component session
	s.sessionsMu.RLock()
	session, exists := s.componentSessions[executorID]
	s.sessionsMu.RUnlock()

	if !exists {
		logrus.Warnf("No component session found for executor_id: %s, closing connection", executorID)
		conn.Close()
		return
	}

	// 设置 WebSocket 连接
	session.wsConnMu.Lock()
	session.wsConn = conn
	session.wsConnMu.Unlock()

	// 启动消息接收循环
	session.wg.Add(1)
	go session.receiveFromWebSocket()
}

// resolveAddress 解析地址，如果命中 DNS 配置，则替换主机名
func (s *Service) resolveAddress(addr string) string {
	if len(s.dnsHosts) == 0 {
		return addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if ip, ok := s.dnsHosts[addr]; ok {
			logrus.Debugf("DNS mapping: %s -> %s", addr, ip)
			return ip
		}
		return addr
	}

	if ip, ok := s.dnsHosts[host]; ok {
		logrus.Debugf("DNS mapping: %s -> %s", host, ip)
		return net.JoinHostPort(ip, port)
	}

	return addr
}

// createZMQSocket 为 component 创建独立的 ZMQ socket（用于与 iarnet 通信）
func (s *Service) createZMQSocket(componentID string, zmqAddr string) (*goczmq.Sock, error) {
	// 使用 goczmq 创建 Dealer socket
	// 确保 ZMQ 地址包含协议前缀
	if !strings.HasPrefix(zmqAddr, "tcp://") && !strings.HasPrefix(zmqAddr, "ipc://") && !strings.HasPrefix(zmqAddr, "inproc://") {
		zmqAddr = "tcp://" + zmqAddr
	}

	// 解析地址，如果命中 DNS 配置则替换
	if strings.HasPrefix(zmqAddr, "tcp://") {
		hostPort := strings.TrimPrefix(zmqAddr, "tcp://")
		resolvedHostPort := s.resolveAddress(hostPort)
		zmqAddr = "tcp://" + resolvedHostPort
	} else {
		zmqAddr = s.resolveAddress(zmqAddr)
	}

	// 创建 Dealer socket
	dealer := goczmq.NewSock(goczmq.Dealer)
	if dealer == nil {
		return nil, fmt.Errorf("failed to create ZMQ dealer socket")
	}

	// 设置 socket 身份标识
	dealer.SetIdentity(componentID)

	// 连接到 ZMQ 地址
	if err := dealer.Connect(zmqAddr); err != nil {
		dealer.Destroy()
		return nil, fmt.Errorf("failed to connect ZMQ dealer socket to %s: %w", zmqAddr, err)
	}

	logrus.Infof("Created ZMQ socket for component %s, connecting to %s", componentID, zmqAddr)
	return dealer, nil
}

// Connect 处理 provider 连接请求
func (s *Service) Connect(ctx context.Context, req *providerpb.ConnectRequest) (*providerpb.ConnectResponse, error) {
	if req == nil || req.ProviderId == "" {
		return &providerpb.ConnectResponse{Success: false, Error: "provider ID is required"}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager.GetProviderID() != "" && s.manager.GetProviderID() != req.ProviderId {
		return &providerpb.ConnectResponse{
			Success: false,
			Error:   fmt.Sprintf("provider already connected: %s", s.manager.GetProviderID()),
		}, nil
	}

	if s.manager != nil {
		s.manager.SetProviderID(req.ProviderId)
	}

	return &providerpb.ConnectResponse{
		Success:            true,
		ProviderType:       &providerpb.ProviderType{Name: providerType},
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
		return fmt.Errorf("provider not connected")
	}

	if s.manager.GetProviderID() != requestProviderID {
		return fmt.Errorf("provider ID mismatch: expected %s, got %s", s.manager.GetProviderID(), requestProviderID)
	}

	return nil
}

// HealthCheck 健康检查
func (s *Service) HealthCheck(ctx context.Context, req *providerpb.HealthCheckRequest) (*providerpb.HealthCheckResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, err
	}

	if s.manager != nil {
		s.manager.UpdateHealthCheck()
	}

	s.mu.RLock()
	total := s.totalCapacity
	allocated := s.allocated
	resourceTags := s.resourceTags
	supportedLanguages := s.supportedLanguages
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
		ResourceTags:       resourceTags,
		SupportedLanguages: supportedLanguages,
	}, nil
}

// GetCapacity 获取资源容量
func (s *Service) GetCapacity(ctx context.Context, req *providerpb.GetCapacityRequest) (*providerpb.GetCapacityResponse, error) {
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, err
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

	return &providerpb.GetCapacityResponse{
		Capacity: &resourcepb.Capacity{
			Total:     total,
			Used:      allocated,
			Available: available,
		},
	}, nil
}

// GetAvailable 获取可用资源
func (s *Service) GetAvailable(ctx context.Context, req *providerpb.GetAvailableRequest) (*providerpb.GetAvailableResponse, error) {
	if err := s.checkAuth(req.ProviderId, true); err != nil {
		return nil, err
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
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager != nil {
		s.manager.ClearProviderID()
	}

	return &providerpb.DisconnectResponse{}, nil
}

// GetRealTimeUsage 获取实时资源使用情况
func (s *Service) GetRealTimeUsage(ctx context.Context, req *providerpb.GetRealTimeUsageRequest) (*providerpb.GetRealTimeUsageResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return nil, err
	}

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

// Close 关闭服务（幂等操作，可以安全地多次调用）
func (s *Service) Close() error {
	// 使用 sync.Once 确保只执行一次
	s.closeOnce.Do(func() {
		logrus.Infof("Closing unikernel provider service...")

		if s.manager != nil {
			s.manager.Stop()
		}

		// 停止所有 component sessions（包括所有 unikernel 进程）
		s.sessionsMu.Lock()
		sessions := make([]*ComponentSession, 0, len(s.componentSessions))
		for _, session := range s.componentSessions {
			sessions = append(sessions, session)
		}
		s.sessionsMu.Unlock()

		// 停止所有 sessions（这会关闭所有 unikernel 进程）
		logrus.Infof("Stopping %d component session(s) and their unikernel processes...", len(sessions))
		for _, session := range sessions {
			session.Stop()
		}

		// 关闭 WebSocket 服务器
		if s.wsServer != nil {
			logrus.Infof("Closing WebSocket server...")
			if err := s.wsServer.Close(); err != nil {
				logrus.Errorf("Error closing WebSocket server: %v", err)
			}
		}

		logrus.Infof("Unikernel provider service closed successfully")
	})
	return nil
}

// Deploy 部署 unikernel 组件
func (s *Service) Deploy(ctx context.Context, req *providerpb.DeployRequest) (*providerpb.DeployResponse, error) {
	if err := s.checkAuth(req.ProviderId, false); err != nil {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	// 检查语言支持
	if req.Language != commonpb.Language_LANG_UNKNOWN && req.Language != commonpb.Language_LANG_UNIKERNEL {
		return &providerpb.DeployResponse{
			Error: fmt.Sprintf("unsupported language: %v, unikernel provider only supports UNIKERNEL", req.Language),
		}, nil
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": req.InstanceId,
		"language":    req.Language,
		"resources":   req.ResourceRequest,
	}).Info("unikernel provider deploy component")

	// 从环境变量中提取 ZMQ_ADDR、STORE_ADDR 和 LOGGER_ADDR
	zmqAddr := req.EnvVars["ZMQ_ADDR"]
	if zmqAddr == "" {
		return &providerpb.DeployResponse{
			Error: "ZMQ_ADDR environment variable is required",
		}, nil
	}
	storeAddr := req.EnvVars["STORE_ADDR"]
	loggerAddr := req.EnvVars["LOGGER_ADDR"]

	// 创建组件会话
	session, err := s.createComponentSession(ctx, req.InstanceId, zmqAddr, storeAddr, loggerAddr)
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

	// 启动会话
	session.Start()

	return &providerpb.DeployResponse{Error: ""}, nil
}

// createComponentSession 为组件创建会话
func (s *Service) createComponentSession(ctx context.Context, componentID string, zmqAddr string, storeAddr string, loggerAddr string) (*ComponentSession, error) {
	if storeAddr != "" {
		storeAddr = s.resolveAddress(storeAddr)
	}

	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if existing, ok := s.componentSessions[componentID]; ok {
		return existing, nil
	}

	// 为每个 component 创建独立的 ZMQ socket
	zmqSocket, err := s.createZMQSocket(componentID, zmqAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create ZMQ socket: %w", err)
	}

	// 获取 store ID
	storeID := ""
	if storeAddr != "" {
		var err error
		storeID, err = s.getStoreID(ctx, storeAddr)
		if err != nil {
			logrus.Warnf("Failed to get store ID for component %s: %v", componentID, err)
		}
	}

	// 创建工作目录
	workDir := filepath.Join("/tmp", "unikernel", componentID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		zmqSocket.Destroy()
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	streamCtx, cancel := context.WithCancel(context.Background())

	// 解析 logger 地址，如果命中 DNS 配置则替换
	if loggerAddr != "" {
		loggerAddr = s.resolveAddress(loggerAddr)
	}

	// 创建日志收集器
	loggerCollector, err := NewLoggerCollector(componentID, loggerAddr)
	if err != nil {
		logrus.Warnf("Failed to create logger collector for component %s: %v, continuing without log collection", componentID, err)
		// 不阻止创建 session，继续执行
	}

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
		workDir:                  workDir,
		pendingInvokes:           make(map[string]chan *UnikernelMessage),
		loggerCollector:          loggerCollector,
	}

	s.componentSessions[componentID] = session

	return session, nil
}

// getStoreID 从 store 服务获取 store ID
func (s *Service) getStoreID(ctx context.Context, storeAddr string) (string, error) {
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

// Start 启动组件会话
func (cs *ComponentSession) Start() {
	cs.wg.Add(1)
	// 启动从 ZMQ 接收消息的 goroutine
	go func() {
		defer cs.wg.Done()
		cs.receiveFromZMQ()
	}()
	logrus.Infof("Component session %s started, waiting for WebSocket connection", cs.componentID)
}

// receiveFromWebSocket 从 WebSocket 接收消息
func (cs *ComponentSession) receiveFromWebSocket() {
	defer cs.wg.Done()

	cs.wsConnMu.RLock()
	conn := cs.wsConn
	cs.wsConnMu.RUnlock()

	if conn == nil {
		return
	}

	for {
		var msg UnikernelMessage
		// 不设置读取超时，让 unikernel 侧持续读取
		// 如果 unikernel 侧连接正常，它会持续等待消息
		// 如果连接被关闭，ReadJSON 会返回错误，我们可以在那时处理
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logrus.Infof("WebSocket connection closed normally for component %s", cs.componentID)
			} else {
				logrus.Errorf("Error reading from WebSocket for component %s: %v", cs.componentID, err)
			}
			// 连接出错后，标记连接为 nil，防止后续写入操作
			cs.wsConnMu.Lock()
			cs.wsConn = nil
			cs.wsConnMu.Unlock()
			return
		}

		// 处理 unikernel 响应
		cs.handleUnikernelResponse(&msg)
	}
}

// handleUnikernelResponse 处理 unikernel 的响应消息
func (cs *ComponentSession) handleUnikernelResponse(msg *UnikernelMessage) {
	cs.pendingInvokesMu.RLock()
	responseChan, exists := cs.pendingInvokes[msg.CorrID]
	cs.pendingInvokesMu.RUnlock()

	if !exists {
		logrus.Warnf("Received response for unknown corr_id: %s", msg.CorrID)
		return
	}

	select {
	case responseChan <- msg:
	default:
		logrus.Warnf("Response channel full for corr_id: %s", msg.CorrID)
	}
}

// receiveFromZMQ 从 ZMQ 接收 iarnet 消息并转换为 unikernel websocket 消息
func (cs *ComponentSession) receiveFromZMQ() {
	if cs.zmqSocket == nil {
		logrus.Errorf("ZMQ socket is nil for component %s", cs.componentID)
		return
	}

	logrus.Infof("Starting ZMQ receiver for component %s", cs.componentID)

	// 创建 poller 用于非阻塞接收
	poller, err := goczmq.NewPoller(cs.zmqSocket)
	if err != nil {
		logrus.Errorf("Failed to create ZMQ poller for component %s: %v", cs.componentID, err)
		return
	}
	defer poller.Destroy()

	// 发送 READY 消息通知连接已建立
	readyMsg := &componentpb.Message{
		Type: componentpb.MessageType_READY,
	}
	readyData, err := proto.Marshal(readyMsg)
	if err != nil {
		logrus.Errorf("Failed to marshal READY message for component %s: %v", cs.componentID, err)
	} else {
		if err := cs.zmqSocket.SendMessage([][]byte{readyData}); err != nil {
			logrus.Errorf("Failed to send READY message to ZMQ for component %s: %v", cs.componentID, err)
		} else {
			logrus.Infof("Sent READY message to ZMQ for component %s", cs.componentID)
		}
	}

	for {
		select {
		case <-cs.ctx.Done():
			logrus.Infof("ZMQ receiver stopped for component %s", cs.componentID)
			return
		default:
			// 使用 poller 等待消息（超时 100 毫秒）
			socket := poller.Wait(100)
			if socket == nil {
				continue
			}

			// 从 ZMQ socket 接收消息
			msgFrames, err := cs.zmqSocket.RecvMessage()
			if err != nil {
				logrus.Errorf("Failed to receive from ZMQ for component %s: %v", cs.componentID, err)
				continue
			}

			if len(msgFrames) == 0 {
				logrus.Warnf("Received empty message from ZMQ for component %s", cs.componentID)
				continue
			}

			// 第一个 frame 是消息数据
			data := msgFrames[0]
			if len(data) == 0 {
				logrus.Debugf("Received empty data from ZMQ for component %s (possibly heartbeat), ignoring", cs.componentID)
				continue
			}

			// 解析 component message
			var componentMsg componentpb.Message
			if err := proto.Unmarshal(data, &componentMsg); err != nil {
				logrus.Errorf("Failed to unmarshal component message from ZMQ for component %s: %v", cs.componentID, err)
				continue
			}

			logrus.Infof("Received message from ZMQ for component %s: type=%v, size=%d bytes", cs.componentID, componentMsg.GetType(), len(data))

			// 处理消息
			if err := cs.handleIarnetMessage(&componentMsg); err != nil {
				logrus.Errorf("Failed to handle iarnet message for component %s: %v", cs.componentID, err)
			}
		}
	}
}

// buildUnikernel 构建 unikernel 代码
// 使用全局构建锁确保同一时间只有一个组件在构建（避免 OPAM 锁冲突）
func (cs *ComponentSession) buildUnikernel(functionName string, code string) error {
	// 获取构建锁，确保串行构建
	cs.service.buildMu.Lock()
	defer cs.service.buildMu.Unlock()

	mirageWebsocketDir := filepath.Join(cs.service.unikernelBaseDir, "mirage-websocket")

	// 复制单个文件
	filesToCopy := []string{"unikernel.ml", "config.ml", "build.sh"}
	for _, filename := range filesToCopy {
		src := filepath.Join(mirageWebsocketDir, filename)
		dst := filepath.Join(cs.workDir, filename)

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", src, err)
		}

		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dst, err)
		}
	}

	// 复制 websocket 目录（递归）
	websocketSrc := filepath.Join(mirageWebsocketDir, "websocket")
	websocketDst := filepath.Join(cs.workDir, "websocket")
	if err := cs.copyDirectory(websocketSrc, websocketDst); err != nil {
		return fmt.Errorf("failed to copy websocket directory: %w", err)
	}

	// 生成 handlers.ml
	// 代码格式：OCaml 函数定义，应该符合 Handler 接口：Yojson.Basic.t -> (Yojson.Basic.t, string) result
	// 用户代码应该直接实现这个接口，不需要包装函数
	handlersPath := filepath.Join(cs.workDir, "handlers.ml")

	// 从代码中提取函数名（查找 "let <function_name>" 模式）
	// 如果找不到，使用传入的 functionName（转换为 OCaml 命名风格）
	ocamlFunctionName := cs.extractFunctionName(code, functionName)

	// 生成完整的 handlers.ml
	// 包含：用户函数定义（应该已经符合 Handler 接口）+ handlers 列表
	handlersContent := cs.generateHandlersML(code, ocamlFunctionName, functionName)

	if err := os.WriteFile(handlersPath, []byte(handlersContent), 0644); err != nil {
		return fmt.Errorf("failed to write handlers.ml: %w", err)
	}

	// 执行构建脚本（使用 hvt target）
	buildScript := filepath.Join(cs.workDir, "build.sh")
	cmd := exec.Command("bash", buildScript, "hvt")
	cmd.Dir = cs.workDir

	// 确保 OPAM 环境变量被设置（mirage 命令需要）
	env := os.Environ()
	// 尝试从 opam env 获取环境变量
	if opamEnv := os.Getenv("OPAM_SWITCH_PREFIX"); opamEnv != "" {
		// OPAM 环境已设置
	} else {
		// 尝试运行 opam env 来获取环境变量
		opamEnvCmd := exec.Command("opam", "env")
		opamEnvCmd.Dir = cs.workDir
		if opamEnvOutput, err := opamEnvCmd.Output(); err == nil {
			// 解析 opam env 输出并添加到环境变量
			// opam env 输出格式：export VAR=value
			envStr := string(opamEnvOutput)
			lines := strings.Split(envStr, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "export ") {
					// 提取 VAR=value
					exportLine := strings.TrimPrefix(line, "export ")
					parts := strings.SplitN(exportLine, "=", 2)
					if len(parts) == 2 {
						key := parts[0]
						value := strings.Trim(parts[1], "\"'")
						env = append(env, fmt.Sprintf("%s=%s", key, value))
					}
				}
			}
		}
	}
	cmd.Env = env

	// 设置实时输出，让用户能看到构建进度
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logrus.Infof("Starting unikernel build for component %s (this may take a while...)", cs.componentID)
	logrus.Infof("Build directory: %s", cs.workDir)

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start build process: %w. Please ensure MirageOS is installed (opam install mirage)", err)
	}

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to build unikernel: %w. Please check the build output above for details. Ensure MirageOS and all dependencies are installed.", err)
	}

	logrus.Infof("Successfully built unikernel for component %s", cs.componentID)
	return nil
}

// extractFunctionName 从 OCaml 代码中提取函数名
// 查找 "let <function_name> <param>" 模式（函数定义，有参数），如果找不到则使用传入的 functionName（转换为 OCaml 命名风格）
func (cs *ComponentSession) extractFunctionName(code string, fallbackName string) string {
	// 使用正则表达式查找 "let <function_name> <param>" 模式
	// 函数定义通常有参数，格式为 "let function_name param1 param2 ="
	// 匹配 "let" 后面跟着函数名，然后跟着至少一个参数（标识符或括号），然后是 "="
	// 优先匹配有参数的函数定义，避免匹配普通的 let 绑定（如 "let message = ..."）
	re := regexp.MustCompile(`let\s+([a-z_][a-z0-9_]*)\s+(?:[a-z_][a-z0-9_]*|\([^)]*\))\s*=`)
	matches := re.FindStringSubmatch(code)
	if len(matches) >= 2 {
		return matches[1]
	}

	// 如果找不到，将 fallbackName 转换为 OCaml 命名风格（下划线分隔，小写）
	ocamlName := strings.ToLower(strings.ReplaceAll(fallbackName, "-", "_"))
	return ocamlName
}

// generateHandlersML 生成完整的 handlers.ml 文件
// 包含：用户函数定义（应该已经符合 Handler 接口）+ handlers 列表
// Handler 接口要求：Yojson.Basic.t -> (Yojson.Basic.t, string) result
func (cs *ComponentSession) generateHandlersML(userCode string, ocamlFunctionName string, functionName string) string {
	// 检查用户代码中是否已经包含 handlers 列表
	if strings.Contains(userCode, "let handlers =") {
		// 用户代码中已经包含了 handlers 列表，直接返回
		return userCode
	}

	// 如果用户代码中没有 handlers 列表，则生成一个
	handlersList := fmt.Sprintf(`let handlers = [
  "%s", %s;
]
`, functionName, ocamlFunctionName)

	// 组合完整的 handlers.ml
	// 用户代码应该已经包含了必要的 open 语句和函数定义
	fullContent := userCode + "\n\n"
	// 添加 handlers 列表
	fullContent += handlersList

	return fullContent
}

// copyDirectory 递归复制目录
func (cs *ComponentSession) copyDirectory(src, dst string) error {
	// 创建目标目录
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dst, err)
	}

	// 读取源目录
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// 递归复制子目录
			if err := cs.copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// 复制文件
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", dstPath, err)
			}
		}
	}

	return nil
}

// runUnikernel 运行 unikernel 可执行文件
func (cs *ComponentSession) runUnikernel() error {
	// 创建网络接口（使用 tap100，所有 component 共享）
	// 注意：如果需要为每个 component 创建独立的 tap 设备，需要生成唯一的 tap 名称
	tapName := "tap100"
	createNetworkScript := filepath.Join(cs.service.unikernelBaseDir, "mirage-websocket", "create_network.sh")

	// 执行网络创建脚本（如果 tap 设备不存在）
	cmd := exec.Command("bash", createNetworkScript)
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		// 如果 tap 设备已存在，忽略错误
		logrus.Debugf("Network setup output: %s, error: %v (may be ignored if tap already exists)", string(output), err)
	}

	// 确保 tap100 接口是 UP 状态（即使接口已存在，也可能处于 DOWN 状态）
	upCmd := exec.Command("sudo", "ip", "link", "set", "dev", tapName, "up")
	if output, err := upCmd.CombinedOutput(); err != nil {
		logrus.Warnf("Failed to bring up %s interface: %v, output: %s (may need manual setup)", tapName, err, string(output))
	} else {
		logrus.Debugf("Ensured %s interface is UP", tapName)
	}

	// 构建 WebSocket URI（使用 tap100 的 IP 地址）
	// provider 的 WebSocket 服务器监听在 wsPort，需要确保 unikernel 可以访问
	// 注意：MirageOS 的 resolver 不支持 ws:// scheme，需要使用 http://
	// WebSocket 握手会通过 HTTP Upgrade 机制完成
	wsURI := fmt.Sprintf("http://10.0.0.1:%d/ws", cs.service.wsPort)

	// 运行 unikernel（参考 run.sh）
	unikernelPath := filepath.Join(cs.workDir, "dist", "ws-handler.hvt")

	// 确定 solo5-hvt 可执行文件路径
	solo5Hvt := cs.service.solo5HvtPath
	if solo5Hvt == "" {
		// 尝试在 PATH 中查找
		solo5Hvt = "solo5-hvt"
		// 验证是否可用
		if _, err := exec.LookPath(solo5Hvt); err != nil {
			return fmt.Errorf("solo5-hvt not found in PATH. Please install solo5 or configure solo5_hvt_path in config file. Error: %w", err)
		}
	} else {
		// 验证配置的路径是否存在
		if _, err := os.Stat(solo5Hvt); err != nil {
			return fmt.Errorf("configured solo5-hvt path does not exist: %s. Error: %w", solo5Hvt, err)
		}
	}

	cmd = exec.Command(solo5Hvt,
		"--net:service="+tapName,
		unikernelPath,
		"--id", cs.componentID,
		"--uri", wsURI,
		"--ipv4", "10.0.0.10/24",
	)
	cmd.Dir = cs.workDir

	// 创建管道来实时读取 unikernel 的 stdout 和 stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	cs.unikernelProcessMu.Lock()
	cs.unikernelProcess = cmd
	cs.unikernelProcessMu.Unlock()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start unikernel: %w. Please ensure solo5-hvt is installed and accessible", err)
	}

	logrus.Infof("Started unikernel process for component %s (tap=%s, uri=%s, pid=%d)", cs.componentID, tapName, wsURI, cmd.Process.Pid)

	// 实时读取 stdout 并发送到日志收集器
	cs.wg.Add(1)
	go func() {
		defer cs.wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			// 发送到日志收集器
			if cs.loggerCollector != nil {
				cs.loggerCollector.CollectLogFromString(line)
			}
			// 同时输出到 logrus（用于本地调试）
			logrus.Debugf("Unikernel stdout for component %s: %s", cs.componentID, line)
		}
		if err := scanner.Err(); err != nil {
			logrus.Errorf("Error reading unikernel stdout for component %s: %v", cs.componentID, err)
		}
	}()

	// 实时读取 stderr 并发送到日志收集器
	cs.wg.Add(1)
	go func() {
		defer cs.wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			// 发送到日志收集器（stderr 通常包含错误信息）
			if cs.loggerCollector != nil {
				cs.loggerCollector.CollectLog(
					commonpb.LogLevel_LOG_LEVEL_ERROR,
					line,
					[]*commonpb.LogField{
						{Key: "stream", Value: "stderr"},
						{Key: "component_id", Value: cs.componentID},
					},
				)
			}
			// 同时输出到 logrus（用于本地调试）
			logrus.Debugf("Unikernel stderr for component %s: %s", cs.componentID, line)
		}
		if err := scanner.Err(); err != nil {
			logrus.Errorf("Error reading unikernel stderr for component %s: %v", cs.componentID, err)
		}
	}()

	// 监控进程退出
	go func() {
		if err := cmd.Wait(); err != nil {
			logrus.Errorf("Unikernel process exited for component %s: %v", cs.componentID, err)
			if cs.loggerCollector != nil {
				cs.loggerCollector.CollectLog(
					commonpb.LogLevel_LOG_LEVEL_ERROR,
					fmt.Sprintf("Unikernel process exited with error: %v", err),
					[]*commonpb.LogField{
						{Key: "component_id", Value: cs.componentID},
						{Key: "error", Value: err.Error()},
					},
				)
			}
		} else {
			logrus.Warnf("Unikernel process exited normally for component %s", cs.componentID)
			if cs.loggerCollector != nil {
				cs.loggerCollector.CollectLog(
					commonpb.LogLevel_LOG_LEVEL_INFO,
					"Unikernel process exited normally",
					[]*commonpb.LogField{
						{Key: "component_id", Value: cs.componentID},
					},
				)
			}
		}
	}()

	return nil
}

// deployFunction 部署 unikernel 函数（构建并运行）
func (cs *ComponentSession) deployFunction(function *actorpb.Function) error {
	language := function.GetLanguage()
	functionName := function.GetName()
	logrus.Infof("Preparing to deploy unikernel function %s for component %s", functionName, cs.componentID)

	if language != commonpb.Language_LANG_UNIKERNEL {
		return fmt.Errorf("unsupported language for unikernel provider: %v", language)
	}

	// 获取 unikernel 代码（存储在 PickledObject 中）
	code := string(function.GetPickledObject())
	if code == "" {
		return fmt.Errorf("empty unikernel code")
	}

	// 记录函数信息
	cs.deployedFunctionMu.Lock()
	cs.deployedFunctionName = functionName
	cs.deployedFunctionLanguage = language
	cs.deployedFunctionMu.Unlock()

	// 构建 unikernel（传入函数名和代码）
	if err := cs.buildUnikernel(functionName, code); err != nil {
		return fmt.Errorf("failed to build unikernel: %w", err)
	}

	// 运行 unikernel
	if err := cs.runUnikernel(); err != nil {
		return fmt.Errorf("failed to run unikernel: %w", err)
	}

	// 等待 WebSocket 连接建立（最多等待 30 秒）
	logrus.Infof("Waiting for WebSocket connection from unikernel for component %s...", cs.componentID)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for WebSocket connection from unikernel for component %s", cs.componentID)
		case <-ticker.C:
			cs.wsConnMu.RLock()
			conn := cs.wsConn
			cs.wsConnMu.RUnlock()
			if conn != nil {
				logrus.Infof("WebSocket connection established for component %s, deployment complete", cs.componentID)
				logrus.Infof("Successfully deployed unikernel function %s for component %s", functionName, cs.componentID)
				return nil
			}
		}
	}
}

// sendToUnikernel 发送消息到 unikernel
func (cs *ComponentSession) sendToUnikernel(corrID, topic string, value json.RawMessage) error {
	cs.wsConnMu.RLock()
	conn := cs.wsConn
	cs.wsConnMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket connection not established")
	}

	msg := UnikernelMessage{
		CorrID: corrID,
		Topic:  topic,
		Value:  value,
	}

	// 不设置写入超时，保持连接打开
	// WebSocket 连接应该保持长连接，不需要超时
	if err := conn.WriteJSON(msg); err != nil {
		// 写入失败，标记连接为 nil
		cs.wsConnMu.Lock()
		cs.wsConn = nil
		cs.wsConnMu.Unlock()
		return fmt.Errorf("failed to write to WebSocket: %w", err)
	}

	return nil
}

// handleInvokeRequest 处理 invoke 请求
func (cs *ComponentSession) handleInvokeRequest(invokeReq *actorpb.InvokeRequest) error {
	runtimeID := invokeReq.GetRuntimeID()
	args := invokeReq.GetArgs()

	logrus.Infof("Processing InvokeRequest for component %s: runtimeID=%s, args_count=%d", cs.componentID, runtimeID, len(args))

	// 解析 RuntimeID 获取函数名
	splits := strings.SplitN(runtimeID, "::", 3)
	var functionName string
	if len(splits) >= 1 {
		functionName = splits[0]
	} else {
		return fmt.Errorf("invalid RuntimeID format: %s", runtimeID)
	}

	// 构建参数 JSON（从 store 获取实际值）
	params := make(map[string]interface{})
	for _, arg := range args {
		param := arg.GetParam()
		value := arg.GetValue()
		if value != nil {
			// 从 store 获取实际值
			objectRef := &commonpb.ObjectRef{
				ID:     value.GetID(),
				Source: value.GetSource(),
			}

			// 获取对象
			objData, objLanguage, err := cs.fetchObjectFromStore(objectRef)
			if err != nil {
				logrus.Errorf("Failed to fetch object %s from store for component %s: %v", value.GetID(), cs.componentID, err)
				return fmt.Errorf("failed to fetch object %s: %w", value.GetID(), err)
			}

			// 如果对象是 Python 编码，先转换为 JSON
			if objLanguage == commonpb.Language_LANG_PYTHON {
				logrus.Infof("Object %s is Python-encoded, converting to JSON for unikernel provider", value.GetID())
				jsonData, err := convertPythonToJSON(objData)
				if err != nil {
					logrus.Errorf("Failed to convert Python object %s to JSON for component %s: %v", value.GetID(), cs.componentID, err)
					return fmt.Errorf("failed to convert Python object %s to JSON: %w", value.GetID(), err)
				}
				objData = jsonData
				objLanguage = commonpb.Language_LANG_JSON
				logrus.Infof("Successfully converted object %s from Python to JSON for component %s", value.GetID(), cs.componentID)
			}

			// 如果对象是 Go (gob) 编码，先转换为 JSON
			if objLanguage == commonpb.Language_LANG_GO {
				logrus.Infof("Object %s is Go (gob)-encoded, converting to JSON for unikernel provider", value.GetID())
				jsonData, err := convertGoToJSON(objData)
				if err != nil {
					logrus.Errorf("Failed to convert Go object %s to JSON for component %s: %v", value.GetID(), cs.componentID, err)
					return fmt.Errorf("failed to convert Go object %s to JSON: %w", value.GetID(), err)
				}
				objData = jsonData
				objLanguage = commonpb.Language_LANG_JSON
				logrus.Infof("Successfully converted object %s from Go (gob) to JSON for component %s", value.GetID(), cs.componentID)
			}

			// 检查对象语言类型，只支持 JSON
			if objLanguage != commonpb.Language_LANG_JSON {
				logrus.Errorf("Object %s has unsupported language %v for unikernel provider (only JSON is supported)", value.GetID(), objLanguage)
				return fmt.Errorf("object %s has unsupported language %v, unikernel provider only supports JSON", value.GetID(), objLanguage)
			}

			// 解码对象值（只支持 JSON）
			objValue, err := cs.decodeObjectValue(objData, objLanguage)
			if err != nil {
				logrus.Errorf("Failed to decode object %s for component %s: %v", value.GetID(), cs.componentID, err)
				return fmt.Errorf("failed to decode object %s: %w", value.GetID(), err)
			}

			params[param] = objValue
		}
	}

	// 将参数序列化为 JSON
	// 用户函数直接接收 Yojson.Basic.t（JSON 对象），符合 Handler 接口：Yojson.Basic.t -> (Yojson.Basic.t, string) result
	// 直接序列化 params map 即可，例如 {"name": "World"}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	// 确保 WebSocket 连接已建立（最多等待 5 秒）
	logrus.Infof("Checking WebSocket connection for component %s before sending invoke request", cs.componentID)
	wsTimeout := time.After(5 * time.Second)
	wsTicker := time.NewTicker(100 * time.Millisecond)
	defer wsTicker.Stop()

	wsReady := false
	for !wsReady {
		select {
		case <-wsTimeout:
			return fmt.Errorf("timeout waiting for WebSocket connection for component %s (invoke request cannot be sent)", cs.componentID)
		case <-wsTicker.C:
			cs.wsConnMu.RLock()
			conn := cs.wsConn
			cs.wsConnMu.RUnlock()
			if conn != nil {
				wsReady = true
				logrus.Infof("WebSocket connection ready for component %s, proceeding with invoke request", cs.componentID)
			}
		}
	}

	// 生成 corr_id
	corrID := fmt.Sprintf("%s-%d", cs.componentID, time.Now().UnixNano())

	// 创建响应 channel
	responseChan := make(chan *UnikernelMessage, 1)
	cs.pendingInvokesMu.Lock()
	cs.pendingInvokes[corrID] = responseChan
	cs.pendingInvokesMu.Unlock()

	// 发送到 unikernel
	if err := cs.sendToUnikernel(corrID, functionName, paramsJSON); err != nil {
		cs.pendingInvokesMu.Lock()
		delete(cs.pendingInvokes, corrID)
		cs.pendingInvokesMu.Unlock()
		logrus.Errorf("Failed to send invoke request to unikernel for component %s: %v", cs.componentID, err)
		return fmt.Errorf("failed to send to unikernel: %w", err)
	}

	logrus.Infof("Successfully sent invoke request to unikernel for component %s: corrID=%s, function=%s", cs.componentID, corrID, functionName)

	// 等待响应
	select {
	case response := <-responseChan:
		cs.pendingInvokesMu.Lock()
		delete(cs.pendingInvokes, corrID)
		cs.pendingInvokesMu.Unlock()

		// 处理响应，转换为 iarnet 消息并发送

		// 创建 InvokeResponse
		invokeResponse := &actorpb.InvokeResponse{
			RuntimeID: runtimeID,
		}

		if response.Error != "" {
			// 处理错误响应
			logrus.Errorf("Unikernel returned error for component %s: %s", cs.componentID, response.Error)
			invokeResponse.Error = response.Error
		} else {
			// 处理成功响应
			// 将响应值保存到 store
			responseValueJSON, err := json.Marshal(response.Value)
			if err != nil {
				return fmt.Errorf("failed to marshal response value: %w", err)
			}

			// 保存到 iarnet store
			objectID := fmt.Sprintf("obj-%s-%d", cs.componentID, time.Now().UnixNano())
			if err := cs.saveObjectToStore(objectID, responseValueJSON); err != nil {
				logrus.Warnf("Failed to save object to store for component %s: %v", cs.componentID, err)
			}

			// 创建 ObjectRef
			objectRef := &commonpb.ObjectRef{
				ID:     objectID,
				Source: cs.storeID,
			}
			invokeResponse.Result = objectRef
		}

		// 创建 actor message
		actorMsg := &actorpb.Message{
			Type: actorpb.MessageType_INVOKE_RESPONSE,
			Message: &actorpb.Message_InvokeResponse{
				InvokeResponse: invokeResponse,
			},
		}

		anyMsg, err := anypb.New(actorMsg)
		if err != nil {
			return fmt.Errorf("failed to create anypb: %w", err)
		}

		// 创建 component message
		componentMsg := &componentpb.Message{
			Type: componentpb.MessageType_PAYLOAD,
			Message: &componentpb.Message_Payload{
				Payload: anyMsg,
			},
		}

		// 发送到 iarnet
		if err := cs.sendToIarnet(componentMsg); err != nil {
			return fmt.Errorf("failed to send response to iarnet: %w", err)
		}

		logrus.Infof("Received and forwarded response from unikernel for component %s", cs.componentID)
		return nil

	case <-time.After(30 * time.Second):
		cs.pendingInvokesMu.Lock()
		delete(cs.pendingInvokes, corrID)
		cs.pendingInvokesMu.Unlock()
		return fmt.Errorf("timeout waiting for unikernel response")
	}
}

// handleIarnetMessage 处理从 iarnet 接收的消息
func (cs *ComponentSession) handleIarnetMessage(componentMsg *componentpb.Message) error {
	switch msgType := componentMsg.GetType(); msgType {
	case componentpb.MessageType_PAYLOAD:
		payload := componentMsg.GetPayload()
		if payload == nil {
			return fmt.Errorf("payload is nil")
		}

		var actorMsg actorpb.Message
		if err := payload.UnmarshalTo(&actorMsg); err != nil {
			return fmt.Errorf("failed to unmarshal actor message: %w", err)
		}

		logrus.Infof("Received actor message for component %s: type=%v", cs.componentID, actorMsg.GetType())

		// 处理不同类型的 actor 消息
		switch actorMsgType := actorMsg.GetType(); actorMsgType {
		case actorpb.MessageType_FUNCTION:
			function := actorMsg.GetFunction()
			if function == nil {
				return fmt.Errorf("function is nil")
			}

			logrus.Infof("Received FUNCTION message for component %s: name=%s", cs.componentID, function.GetName())

			// 部署 unikernel 函数（构建并运行）
			return cs.deployFunction(function)

		case actorpb.MessageType_INVOKE_REQUEST:
			invokeReq := actorMsg.GetInvokeRequest()
			if invokeReq == nil {
				return fmt.Errorf("invoke request is nil")
			}

			logrus.Infof("Received INVOKE_REQUEST message for component %s: runtimeID=%s, args_count=%d", cs.componentID, invokeReq.GetRuntimeID(), len(invokeReq.GetArgs()))

			// 处理 invoke 请求
			return cs.handleInvokeRequest(invokeReq)

		default:
			logrus.Warnf("Unhandled actor message type: %v for component %s", actorMsgType, cs.componentID)
			return nil
		}

	case componentpb.MessageType_READY:
		logrus.Debugf("Received READY message for component %s", cs.componentID)
		return nil

	default:
		logrus.Debugf("Unhandled component message type: %v for component %s", msgType, cs.componentID)
		return nil
	}
}

// sendToIarnet 将消息发送到 iarnet（通过 ZMQ）
func (cs *ComponentSession) sendToIarnet(componentMsg *componentpb.Message) error {
	if cs.zmqSocket == nil {
		return fmt.Errorf("ZMQ socket is nil for component %s", cs.componentID)
	}

	data, err := proto.Marshal(componentMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal component message: %w", err)
	}

	if err := cs.zmqSocket.SendMessage([][]byte{data}); err != nil {
		return fmt.Errorf("failed to send message to ZMQ: %w", err)
	}

	return nil
}

// fetchObjectFromStore 从 iarnet store 获取对象
func (cs *ComponentSession) fetchObjectFromStore(objectRef *commonpb.ObjectRef) ([]byte, commonpb.Language, error) {
	if cs.storeAddr == "" {
		return nil, commonpb.Language_LANG_UNKNOWN, fmt.Errorf("store address not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 连接到 iarnet store
	conn, err := grpc.NewClient(cs.storeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, commonpb.Language_LANG_UNKNOWN, fmt.Errorf("failed to connect to store: %w", err)
	}
	defer conn.Close()

	storeClient := storepb.NewServiceClient(conn)

	// 从 store 获取对象
	encodedObj, err := storeClient.GetObject(ctx, &storepb.GetObjectRequest{
		ObjectRef: objectRef,
	})
	if err != nil {
		return nil, commonpb.Language_LANG_UNKNOWN, fmt.Errorf("failed to get object from store: %w", err)
	}

	if encodedObj.GetObject() == nil {
		return nil, commonpb.Language_LANG_UNKNOWN, fmt.Errorf("object not found in store")
	}

	obj := encodedObj.GetObject()
	logrus.Debugf("Fetched object %s from store for component %s, language: %v", obj.GetID(), cs.componentID, obj.GetLanguage())
	return obj.GetData(), obj.GetLanguage(), nil
}

// decodeObjectValue 解码对象值为 Go 原生类型（只支持 JSON）
func (cs *ComponentSession) decodeObjectValue(data []byte, language commonpb.Language) (interface{}, error) {
	switch language {
	case commonpb.Language_LANG_JSON:
		// JSON 编码，直接解析
		var value interface{}
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		return value, nil
	default:
		// 其他编码格式，尝试作为字符串返回
		return string(data), nil
	}
}

// convertGoToJSON 将 Go gob 编码的对象转换为 JSON 格式
func convertGoToJSON(gobData []byte) ([]byte, error) {
	// 使用 gob 解码
	dec := gob.NewDecoder(bytes.NewReader(gobData))
	var value interface{}
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("failed to decode gob: %w", err)
	}

	// 转换为 JSON
	jsonData, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return jsonData, nil
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

// saveObjectToStore 保存对象到 iarnet store
func (cs *ComponentSession) saveObjectToStore(objectID string, data []byte) error {
	if cs.storeAddr == "" {
		return fmt.Errorf("store address not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 连接到 iarnet store
	conn, err := grpc.NewClient(cs.storeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to store: %w", err)
	}
	defer conn.Close()

	storeClient := storepb.NewServiceClient(conn)

	// 保存对象到 store
	_, err = storeClient.SaveObject(ctx, &storepb.SaveObjectRequest{
		Object: &commonpb.EncodedObject{
			ID:       objectID,
			Data:     data,
			Language: commonpb.Language_LANG_JSON,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to save object to store: %w", err)
	}

	logrus.Debugf("Saved object %s to store for component %s", objectID, cs.componentID)
	return nil
}

// Stop 停止组件会话
func (cs *ComponentSession) Stop() {
	// 停止 unikernel 进程
	cs.unikernelProcessMu.Lock()
	if cs.unikernelProcess != nil {
		cs.unikernelProcess.Process.Kill()
		cs.unikernelProcess = nil
	}
	cs.unikernelProcessMu.Unlock()

	// 关闭 WebSocket 连接
	cs.wsConnMu.Lock()
	if cs.wsConn != nil {
		cs.wsConn.Close()
		cs.wsConn = nil
	}
	cs.wsConnMu.Unlock()

	// 关闭 ZMQ socket
	if cs.zmqSocket != nil {
		cs.zmqSocket.Destroy()
		cs.zmqSocket = nil
	}

	cs.cancel()
	cs.wg.Wait()

	// 停止日志收集器
	if cs.loggerCollector != nil {
		cs.loggerCollector.Stop()
		cs.loggerCollector = nil
	}

	// 清理工作目录
	os.RemoveAll(cs.workDir)
}

// LoggerCollector 日志收集器，用于收集 unikernel 的日志并上传到 iarnet logger 服务
// 复用 process provider 中的实现
type LoggerCollector struct {
	componentID  string
	loggerAddr   string
	loggerClient reslogger.LoggerServiceClient
	loggerConn   *grpc.ClientConn
	stream       grpc.BidiStreamingClient[reslogger.LogStreamMessage, reslogger.LogStreamResponse]
	ctx          context.Context
	cancel       context.CancelFunc
	logQueue     chan *commonpb.LogEntry
	wg           sync.WaitGroup
	mu           sync.RWMutex
}

// NewLoggerCollector 创建新的日志收集器
func NewLoggerCollector(componentID string, loggerAddr string) (*LoggerCollector, error) {
	if loggerAddr == "" {
		// 如果没有配置 logger 地址，返回 nil（不收集日志）
		return nil, nil
	}

	// 连接到 logger 服务
	conn, err := grpc.NewClient(loggerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to logger service: %w", err)
	}

	client := reslogger.NewLoggerServiceClient(conn)
	ctx, cancel := context.WithCancel(context.Background())

	collector := &LoggerCollector{
		componentID:  componentID,
		loggerAddr:   loggerAddr,
		loggerClient: client,
		loggerConn:   conn,
		ctx:          ctx,
		cancel:       cancel,
		logQueue:     make(chan *commonpb.LogEntry, 1000), // 缓冲队列
	}

	// 初始化 stream
	stream, err := client.StreamLogs(ctx)
	if err != nil {
		cancel()
		conn.Close()
		return nil, fmt.Errorf("failed to create log stream: %w", err)
	}
	collector.stream = stream

	// 启动日志发送 goroutine
	collector.wg.Add(1)
	go collector.sendLogLoop()

	// 启动接收响应 goroutine
	collector.wg.Add(1)
	go collector.receiveResponseLoop()

	logrus.Infof("Created logger collector for component %s, logger_addr=%s", componentID, loggerAddr)
	return collector, nil
}

// sendLogLoop 日志发送循环
func (lc *LoggerCollector) sendLogLoop() {
	defer lc.wg.Done()
	logrus.Debugf("Started log send loop for component %s", lc.componentID)

	for {
		select {
		case <-lc.ctx.Done():
			logrus.Debugf("Log send loop stopped for component %s", lc.componentID)
			return
		case entry := <-lc.logQueue:
			if entry == nil {
				continue
			}

			msg := &reslogger.LogStreamMessage{
				ComponentId: lc.componentID,
				Message: &reslogger.LogStreamMessage_Entry{
					Entry: entry,
				},
			}

			lc.mu.RLock()
			stream := lc.stream
			lc.mu.RUnlock()

			if stream == nil {
				logrus.Warnf("Log stream is nil for component %s, dropping log entry", lc.componentID)
				continue
			}

			if err := stream.Send(msg); err != nil {
				logrus.Errorf("Failed to send log entry for component %s: %v", lc.componentID, err)
				// 如果发送失败，尝试重新连接
				lc.reconnect()
			}
		}
	}
}

// receiveResponseLoop 接收响应循环
func (lc *LoggerCollector) receiveResponseLoop() {
	defer lc.wg.Done()
	logrus.Debugf("Started log response loop for component %s", lc.componentID)

	for {
		select {
		case <-lc.ctx.Done():
			logrus.Debugf("Log response loop stopped for component %s", lc.componentID)
			return
		default:
			lc.mu.RLock()
			stream := lc.stream
			lc.mu.RUnlock()

			if stream == nil {
				select {
				case <-lc.ctx.Done():
					return
				case <-time.After(1 * time.Second):
					continue
				}
			}

			// 使用 goroutine 和 channel 来非阻塞地接收消息
			type recvResult struct {
				resp *reslogger.LogStreamResponse
				err  error
			}
			recvCh := make(chan recvResult, 1)

			go func() {
				resp, err := stream.Recv()
				recvCh <- recvResult{resp: resp, err: err}
			}()

			select {
			case <-lc.ctx.Done():
				return
			case result := <-recvCh:
				if result.err == io.EOF {
					logrus.Infof("Log stream closed for component %s", lc.componentID)
					return
				}
				if result.err != nil {
					logrus.Errorf("Failed to receive log response for component %s: %v", lc.componentID, result.err)
					lc.reconnect()
					continue
				}
				if result.resp != nil && !result.resp.GetSuccess() && result.resp.GetError() != "" {
					logrus.Warnf("Log service error for component %s: %s", lc.componentID, result.resp.GetError())
				}
			}
		}
	}
}

// reconnect 重新连接 logger 服务
func (lc *LoggerCollector) reconnect() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// 关闭旧连接
	if lc.stream != nil {
		// stream 会在 context 取消时自动关闭
		lc.stream = nil
	}

	// 重新连接
	conn, err := grpc.NewClient(lc.loggerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.Errorf("Failed to reconnect to logger service for component %s: %v", lc.componentID, err)
		return
	}

	client := reslogger.NewLoggerServiceClient(conn)
	stream, err := client.StreamLogs(lc.ctx)
	if err != nil {
		logrus.Errorf("Failed to create log stream for component %s: %v", lc.componentID, err)
		conn.Close()
		return
	}

	// 关闭旧连接
	if lc.loggerConn != nil {
		lc.loggerConn.Close()
	}

	lc.loggerConn = conn
	lc.loggerClient = client
	lc.stream = stream
	logrus.Infof("Reconnected to logger service for component %s", lc.componentID)
}

// CollectLog 收集日志条目
func (lc *LoggerCollector) CollectLog(level commonpb.LogLevel, message string, fields []*commonpb.LogField) {
	if lc == nil {
		return
	}

	entry := &commonpb.LogEntry{
		Timestamp: time.Now().UnixNano(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	select {
	case lc.logQueue <- entry:
		// 成功入队
	default:
		// 队列已满，丢弃日志
		logrus.Warnf("Log queue is full for component %s, dropping log entry", lc.componentID)
	}
}

// CollectLogFromString 从字符串收集日志（解析日志级别）
func (lc *LoggerCollector) CollectLogFromString(logLine string) {
	if lc == nil {
		return
	}

	// 尝试解析日志级别
	level := commonpb.LogLevel_LOG_LEVEL_INFO
	message := logLine

	// 简单的日志级别解析（可以根据实际格式调整）
	logLineLower := strings.ToLower(logLine)
	if strings.Contains(logLineLower, "error") || strings.Contains(logLineLower, "err") {
		level = commonpb.LogLevel_LOG_LEVEL_ERROR
	} else if strings.Contains(logLineLower, "warn") || strings.Contains(logLineLower, "warning") {
		level = commonpb.LogLevel_LOG_LEVEL_WARN
	} else if strings.Contains(logLineLower, "debug") {
		level = commonpb.LogLevel_LOG_LEVEL_DEBUG
	} else if strings.Contains(logLineLower, "fatal") || strings.Contains(logLineLower, "panic") {
		level = commonpb.LogLevel_LOG_LEVEL_FATAL
	}

	lc.CollectLog(level, message, nil)
}

// Stop 停止日志收集器
func (lc *LoggerCollector) Stop() {
	if lc == nil {
		return
	}

	logrus.Infof("Stopping logger collector for component %s", lc.componentID)

	// 发送关闭控制消息
	lc.mu.RLock()
	stream := lc.stream
	lc.mu.RUnlock()

	if stream != nil {
		controlMsg := &reslogger.LogStreamMessage{
			ComponentId: lc.componentID,
			Message: &reslogger.LogStreamMessage_Control{
				Control: &commonpb.StreamControl{
					Type: commonpb.StreamControl_CONTROL_CLOSE,
				},
			},
		}
		stream.Send(controlMsg)
	}

	// 取消 context
	lc.cancel()

	// 关闭队列
	close(lc.logQueue)

	// 等待 goroutine 完成
	lc.wg.Wait()

	// 关闭连接
	if lc.loggerConn != nil {
		lc.loggerConn.Close()
		lc.loggerConn = nil
	}

	logrus.Infof("Logger collector stopped for component %s", lc.componentID)
}
