package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/9triver/iarnet/component/py"
	"github.com/9triver/iarnet/component/runtime"
	"github.com/9triver/ignis/actor/compute"
	"github.com/9triver/ignis/actor/router"
	"github.com/9triver/ignis/actor/store"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils"
	pb "google.golang.org/protobuf/proto"
)

func main() {
	// appId := os.Getenv("APP_ID")
	ignisAddr := os.Getenv("IGNIS_ADDR")
	// funcName := os.Getenv("FUNC_NAME")
	// language := os.Getenv("LANGUAGE")

	connId := os.Getenv("CONN_ID")

	var err error
	// 创建 gRPC 连接
	conn, err := grpc.NewClient(ignisAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(512*1024*1024), // 最大接收消息大小：512MB
			grpc.MaxCallSendMsgSize(512*1024*1024), // 最大发送消息大小：512MB
		),
	)

	if err != nil {
		logrus.Fatalf("failed to connect to ignis server: %v", err)
	}
	defer conn.Close()

	// 创建 compute 服务客户端，配置消息大小限制
	client := cluster.NewServiceClient(conn)

	// 创建上下文 - 不设置超时，允许长期阻塞等待消息
	ctx := context.Background()

	// 调用 Session 方法
	stream, err := client.Session(ctx)
	if err != nil {
		logrus.Fatalf("failed to create session stream: %v", err)
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
		logrus.Fatalf("failed to send ready message: %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		logrus.Fatalf("Error receiving message: %v", err)
	}

	if msg.Type != cluster.MessageType_FUNCTION {
		logrus.Fatalf("expected function message, but got: %+v", msg)
	}

	funcMsg := msg.GetFunction()
	logrus.Infof("Successfully received function, name: %+v", funcMsg.Name)

	// 依据语言选择对应运行时，并进行环境准备（根目录接口）
	initializer, err := GetInitializer(funcMsg.Language)
	if err != nil {
		logrus.Fatalf("runtime error: %v", err)
	}

	ipcAddr := os.Getenv("IPC_ADDR")
	if ipcAddr == "" {
		// 使用应用目录下的socket文件，确保权限正确
		ipcAddr = "ipc:///app/executor.sock"
	}

	// 清理可能存在的旧socket文件
	socketPath := strings.TrimPrefix(ipcAddr, "ipc://")
	if _, err1 := os.Stat(socketPath); err1 == nil {
		logrus.Infof("Removing existing socket file: %s", socketPath)
		os.Remove(socketPath)
	}

	manager := runtime.NewManager()

	f, err := manager.Run(ctx, ipcAddr, connId, funcMsg, initializer)
	if err != nil {
		logrus.Fatalf("runtime setup failed: %v", err)
	}

	logrus.Infof("Function %s loaded and ready for execution", funcMsg.Name)

	opt := utils.WithLogger()
	sys := actor.NewActorSystem(opt)

	stubPid := sys.Root.Spawn(actor.PropsFromProducer(func() actor.Actor {
		return NewStub(connId, stream)
	}))
	router.Register("stub-"+connId, stubPid)
	router.SetDefaultTarget(stubPid)

	sr := store.Spawn(sys.Root, nil, "store"+connId)
	router.Register("store"+connId, sr.PID)

	props := compute.NewActor(connId, f, sr.PID)
	pid, err1 := sys.Root.SpawnNamed(props, connId)

	if err1 != nil {
		logrus.Fatalf("failed to spawn compute actor: %v", err1)
	}

	router.Register(connId, pid)

	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				logrus.Errorf("Error receiving message: %v", err)
				return
			}
			m := msg.Unwrap()
			logrus.Infof("Received message: %+v", m)
			// if m, ok := m.(store.RequiresReplyMessage); ok {
			// 	router.RegisterIfAbsent(m.GetReplyTo(), stubPid)
			// }
			if m, ok := m.(store.ForwardMessage); ok {
				router.Send(sys.Root, m.GetTarget(), m)
			} else {
				logrus.Warnf("unsupported message type: %+v", m)
			}
		}
	}()

	// Graceful shutdown
	sigCh1 := make(chan os.Signal, 1)
	signal.Notify(sigCh1, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh1
}

// GetManager 返回指定语言的运行时管理器
func GetInitializer(language proto.Language) (runtime.Initializer, error) {
	switch language {
	case proto.Language_LANG_PYTHON:
		// 假设Python运行时需要的参数
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

type Stub struct {
	connId string
	stream grpc.BidiStreamingClient[cluster.Message, cluster.Message]
}

func NewStub(connId string, stream grpc.BidiStreamingClient[cluster.Message, cluster.Message]) *Stub {
	return &Stub{
		connId: connId,
		stream: stream,
	}
}

func (s *Stub) Receive(ctx actor.Context) {
	if msg, ok := ctx.Message().(pb.Message); ok {
		m := cluster.NewMessage(msg)
		if m == nil {
			ctx.Logger().Warn("unsupported message type", "msg", msg)
			return
		}
		m.ConnID = s.connId
		logrus.Infof("Sending message: %+v", m)
		if err := s.stream.Send(m); err != nil {
			ctx.Logger().Error("failed to send message", "msg", m, "error", err)
		}
	} else {
		ctx.Logger().Warn("unsupported message type", "msg", ctx.Message())
	}
}
