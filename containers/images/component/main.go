// Package main 是容器组件的主入口程序
// 负责连接 Ignis 平台、初始化运行时环境、启动 Actor 系统并处理消息路由
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/protobuf/proto"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"

	py "github.com/9triver/iarnet/component/python/runtime"
	"github.com/9triver/iarnet/component/runtime"
	"github.com/9triver/ignis/actor/compute"
	"github.com/9triver/ignis/actor/router"
	"github.com/9triver/ignis/actor/store"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/transport/ipc"
	"github.com/9triver/ignis/utils"
)

const (
	// 默认的 IPC 套接字地址
	defaultIPCAddr = "ipc:///app/executor.sock"

	// gRPC 消息大小限制（512MB）
	maxMessageSize = 512 * 1024 * 1024
)

func main() {
	ctx := context.Background()

	// 从环境变量获取配置
	ignisAddr := os.Getenv("IGNIS_ADDR")
	connId := os.Getenv("CONN_ID")

	if ignisAddr == "" {
		logrus.Fatal("IGNIS_ADDR environment variable is required")
	}
	if connId == "" {
		logrus.Fatal("CONN_ID environment variable is required")
	}

	// 创建 gRPC 连接
	conn, err := createGRPCConnection(ignisAddr)
	if err != nil {
		logrus.Fatalf("failed to connect to ignis server: %v", err)
	}
	defer conn.Close()

	// 创建会话并接收函数定义
	stream, funcMsg, err := createSessionAndReceiveFunction(ctx, conn, connId)
	if err != nil {
		logrus.Fatalf("failed to create session and receive function: %v", err)
	}

	// 初始化运行时
	initializer, err := getInitializer(funcMsg.Language)
	if err != nil {
		logrus.Fatalf("runtime error: %v", err)
	}

	// 设置并启动 IPC 管理器
	ipcAddr := getIPCAddress()
	if err := cleanupSocketFile(ipcAddr); err != nil {
		logrus.Warnf("failed to cleanup socket file: %v", err)
	}

	im := ipc.NewManager(ipcAddr)
	rm := runtime.NewManager()

	if err := im.Start(ctx); err != nil {
		logrus.Fatalf("ipc manager start failed: %v", err)
	}

	// 创建执行连接并启动函数
	execConn := runtime.NewConnection(ipcAddr, connId, im.NewExecutor(ctx, connId))
	f, err := rm.Start(ctx, execConn, funcMsg, initializer)
	if err != nil {
		logrus.Fatalf("runtime manager start failed: %v", err)
	}

	logrus.Infof("Function %s loaded and ready for execution", funcMsg.Name)

	// 创建 Actor 系统并注册路由
	sys := setupActorSystem(connId, stream, f)

	// 启动消息接收循环
	go receiveMessages(stream, sys.Root)

	// 等待优雅关闭信号
	waitForShutdown()

	logrus.Info("Shutting down component...")
}

// createGRPCConnection 创建到 Ignis 服务器的 gRPC 连接
// 参数:
//   - addr: 服务器地址
//
// 返回值:
//   - *grpc.ClientConn: gRPC 连接
//   - error: 连接错误
func createGRPCConnection(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMessageSize),
			grpc.MaxCallSendMsgSize(maxMessageSize),
		),
	)
}

// createSessionAndReceiveFunction 创建会话并接收函数定义
// 参数:
//   - ctx: 上下文
//   - conn: gRPC 连接
//   - connId: 连接标识符
//
// 返回值:
//   - grpc.BidiStreamingClient[cluster.Message, cluster.Message]: 双向流
//   - *cluster.Function: 函数定义
//   - error: 错误
func createSessionAndReceiveFunction(
	ctx context.Context,
	conn *grpc.ClientConn,
	connId string,
) (grpc.BidiStreamingClient[cluster.Message, cluster.Message], *cluster.Function, error) {
	client := cluster.NewServiceClient(conn)

	stream, err := client.Session(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session stream: %w", err)
	}

	// 发送 Ready 消息
	readyMsg := &cluster.Message{
		Type:   cluster.MessageType_READY,
		ConnID: connId,
		Message: &cluster.Message_Ready{
			Ready: &cluster.Ready{},
		},
	}

	if err = stream.Send(readyMsg); err != nil {
		return nil, nil, fmt.Errorf("failed to send ready message: %w", err)
	}

	// 接收函数定义
	msg, err := stream.Recv()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to receive message: %w", err)
	}

	if msg.Type != cluster.MessageType_FUNCTION {
		return nil, nil, fmt.Errorf("expected function message, but got: %v", msg.Type)
	}

	funcMsg := msg.GetFunction()
	logrus.Infof("Successfully received function: %s", funcMsg.Name)

	return stream, funcMsg, nil
}

// getIPCAddress 获取 IPC 地址
// 从环境变量读取，如果未设置则使用默认值
//
// 返回值:
//   - string: IPC 地址
func getIPCAddress() string {
	ipcAddr := os.Getenv("IPC_ADDR")
	if ipcAddr == "" {
		return defaultIPCAddr
	}
	return ipcAddr
}

// cleanupSocketFile 清理可能存在的旧 socket 文件
// 参数:
//   - ipcAddr: IPC 地址（格式：ipc://path/to/socket）
//
// 返回值:
//   - error: 清理错误
func cleanupSocketFile(ipcAddr string) error {
	socketPath := strings.TrimPrefix(ipcAddr, "ipc://")
	if _, err := os.Stat(socketPath); err == nil {
		logrus.Infof("Removing existing socket file: %s", socketPath)
		return os.Remove(socketPath)
	}
	return nil
}

// setupActorSystem 创建 Actor 系统并注册所有路由
// 参数:
//   - connId: 连接标识符
//   - stream: gRPC 双向流
//   - f: 函数实例
//
// 返回值:
//   - *actor.ActorSystem: Actor 系统实例
func setupActorSystem(
	connId string,
	stream grpc.BidiStreamingClient[cluster.Message, cluster.Message],
	f *runtime.Funciton,
) *actor.ActorSystem {
	sys := actor.NewActorSystem(utils.WithLogger())

	// 创建并注册 Stub Actor
	stubPid := sys.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewStub(connId, stream)
	}))
	router.Register("stub-"+connId, stubPid)
	router.SetDefaultTarget(stubPid)

	// 创建并注册 Store Actor
	sr := store.Spawn(sys.Root, nil, "store-"+connId)
	router.Register("store"+connId, sr.PID)

	// 创建并注册 Compute Actor
	props := compute.NewActor(connId, f, sr.PID)
	pid, err := sys.Root.SpawnNamed(props, connId)
	if err != nil {
		logrus.Fatalf("failed to spawn compute actor: %v", err)
	}
	router.Register(connId, pid)

	logrus.Infof("Actor system initialized with connId: %s", connId)
	return sys
}

// receiveMessages 持续接收并处理来自流的消息
// 参数:
//   - stream: gRPC 双向流
//   - root: Actor 系统的根上下文
func receiveMessages(
	stream grpc.BidiStreamingClient[cluster.Message, cluster.Message],
	root *actor.RootContext,
) {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			logrus.Info("Stream closed by server")
			return
		}
		if err != nil {
			logrus.Errorf("Error receiving message: %v", err)
			return
		}

		m := msg.Unwrap()
		logrus.Infof("Received message: %+v", m)

		// 处理转发消息
		if forwardMsg, ok := m.(store.ForwardMessage); ok {
			router.Send(root, forwardMsg.GetTarget(), forwardMsg)
		} else {
			logrus.Warnf("unsupported message type: %+v", m)
		}
	}
}

// waitForShutdown 等待优雅关闭信号
func waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logrus.Infof("Received shutdown signal: %v", sig)
}

// getInitializer 根据语言类型返回对应的运行时初始化器
// 参数:
//   - language: 编程语言类型
//
// 返回值:
//   - runtime.Initializer: 运行时初始化器
//   - error: 不支持的语言错误
func getInitializer(language proto.Language) (runtime.Initializer, error) {
	switch language {
	case proto.Language_LANG_PYTHON:
		venvPath := os.Getenv("VENV_PATH")
		if venvPath == "" {
			venvPath = "/path/to/venv"
		}
		executorPath := os.Getenv("EXECUTOR_PATH")
		if executorPath == "" {
			executorPath = "/path/to/executor"
		}
		return py.NewInitializer(venvPath, executorPath)

	default:
		return nil, fmt.Errorf("unsupported language: %v", language)
	}
}

// Stub 是消息存根 Actor
// 负责将本地 Actor 消息转发到 gRPC 流
type Stub struct {
	connId string                                                     // 连接标识符
	stream grpc.BidiStreamingClient[cluster.Message, cluster.Message] // gRPC 双向流
}

// NewStub 创建一个新的 Stub Actor 实例
// 参数:
//   - connId: 连接标识符
//   - stream: gRPC 双向流
//
// 返回值:
//   - *Stub: Stub Actor 实例
func NewStub(connId string, stream grpc.BidiStreamingClient[cluster.Message, cluster.Message]) *Stub {
	return &Stub{
		connId: connId,
		stream: stream,
	}
}

// Receive 实现 Actor 接口，处理接收到的消息
// 参数:
//   - ctx: Actor 上下文
//
// 功能:
//   - 接收本地 Actor 消息
//   - 将 protobuf 消息包装为 cluster.Message
//   - 通过 gRPC 流发送到服务器
func (s *Stub) Receive(ctx actor.Context) {
	originalMsg := ctx.Message()
	logrus.Infof("stub: receive message: %+v", originalMsg)

	msg, ok := originalMsg.(pb.Message)
	if !ok {
		ctx.Logger().Warn("unsupported message type", "msg", originalMsg)
		return
	}

	m := cluster.NewMessage(msg)
	if m == nil {
		ctx.Logger().Warn("failed to create cluster message", "msg", msg)
		return
	}

	m.ConnID = s.connId
	if err := s.stream.Send(m); err != nil {
		ctx.Logger().Error("failed to send message", "msg", m, "error", err)
	}
}
