package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/9triver/iarnet/component/py"
	"github.com/9triver/iarnet/component/runtime"
	"github.com/9triver/iarnet/component/stub"
	"github.com/9triver/ignis/actor/compute"
	"github.com/9triver/ignis/actor/store"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils"
)

func main() {
	appId := os.Getenv("APP_ID")
	ignisAddr := os.Getenv("IGNIG_ADDR")
	funcName := os.Getenv("FUNC_NAME")
	// language := os.Getenv("LANGUAGE")

	connId := fmt.Sprintf("%s:%s", appId, funcName)

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

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
	logrus.Infof("Successfully connected to ignis server and sent ready message for connection: %s", connId)

	// 依据语言选择对应运行时，并进行环境准备（根目录接口）
	rt, err := runtime.GetManager(funcMsg.Language)
	if err != nil {
		logrus.Fatalf("runtime error: %v", err)
	}
	if err := rt.Setup(funcMsg); err != nil {
		logrus.Fatalf("runtime setup failed: %v", err)
	}

	// 处理函数注册
	handleFunctionMessage(funcMsg)

	logrus.Infof("Function %s loaded and ready for execution", funcMsg.Name)

	opt := utils.WithLogger()
	sys := actor.NewActorSystem(opt)

	store := store.Spawn(sys.Root, stub.NewRpcStub(stream), "store")

	props := compute.NewActor("compute", nil, store.PID)
	computePid, err1 := sys.Root.SpawnNamed(props, "compute")

	if err1 != nil {
		logrus.Fatalf("failed to spawn compute actor: %v", err1)
	}

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

			// 根据 MessageType 决定转发到哪个 actor
			switch msg.Type {
			case cluster.MessageType_INVOKE, cluster.MessageType_INVOKE_START:
				sys.Root.Send(computePid, msg)
			case cluster.MessageType_OBJECT_REQUEST, cluster.MessageType_OBJECT_RESPONSE, cluster.MessageType_STREAM_CHUNK:
				sys.Root.Send(store.PID, msg)
			case cluster.MessageType_FUNCTION:
				// 处理函数注册消息
				if funcMsg := msg.GetFunction(); funcMsg != nil {
					go handleFunctionMessage(funcMsg)
				}
			// case cluster.MessageType_ACK, cluster.MessageType_READY:
			// 	// 控制消息转发到 store actor
			// 	sys.Root.Send(store.PID, msg)
			// default:
			// 	logrus.Warnf("Unknown message type: %v, forwarding to store", msg.Type)
			// 	sys.Root.Send(store.PID, msg)
			default:
				logrus.Warnf("Unsupported message type: %v", msg.Type)
			}
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func runPyExecutor() {
	pythonPath := os.Getenv("PYTHON_PATH")
	executorPath := os.Getenv("EXECUTOR_PATH")

	if pythonPath == "" {
		logrus.Warn("PYTHON_PATH is empty, use default python3")
		pythonPath = "python3"
	}

	if executorPath == "" {
		logrus.Fatal("EXECUTOR_PATH is empty, please set it")
	}

	cmd := exec.Command(pythonPath, executorPath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logrus.Fatalf("failed to run python executor: %v", err)
	}
}

// handleFunctionMessage 处理函数注册消息
func handleFunctionMessage(funcMsg *cluster.Function) {
	logrus.Infof("Handling function registration: %s", funcMsg.Name)
	
	// 依据语言选择对应运行时，并进行环境准备
	rt, err := runtime.GetManager(funcMsg.Language)
	if err != nil {
		logrus.Errorf("runtime error: %v", err)
		return
	}
	
	if err := rt.Setup(funcMsg); err != nil {
		logrus.Errorf("runtime setup failed: %v", err)
		return
	}

	// 如果是Python函数，启动容器执行器
	if funcMsg.Language == proto.Language_LANG_PYTHON {
		// 类型断言获取Python运行时管理器
		pyRT, ok := rt.(*py.RuntimeManager)
		if !ok {
			logrus.Errorf("failed to cast to Python runtime manager")
			return
		}
		
		ctx := context.Background()
		containerExec := pyRT.GetContainerExecutor()
		
		if err := containerExec.Start(ctx); err != nil {
			logrus.Errorf("failed to start container executor: %v", err)
			return
		}

		// 启动Python执行器进程
		connName := fmt.Sprintf("executor-%s", funcMsg.Name)
		if err := containerExec.StartPythonExecutor(ctx, connName); err != nil {
			logrus.Errorf("failed to start python executor: %v", err)
			return
		}

		// 等待连接建立
		time.Sleep(2 * time.Second)

		// 注册函数到容器执行器
		if len(funcMsg.PickledObject) > 0 {
			// 如果有序列化的函数对象，创建临时文件
			tmpFile, err := os.CreateTemp("/tmp", "func_*.py")
			if err != nil {
				logrus.Errorf("failed to create temp file: %v", err)
				return
			}
			defer os.Remove(tmpFile.Name())

			// 将序列化的函数对象写入文件
			if _, err := tmpFile.Write(funcMsg.PickledObject); err != nil {
				logrus.Errorf("failed to write function object: %v", err)
				return
			}
			tmpFile.Close()

			// 加载函数到执行器
			if _, err := containerExec.Execute(ctx, connName, "load", map[string]interface{}{
				"file_path": tmpFile.Name(),
				"func_name": funcMsg.Name,
			}); err != nil {
				logrus.Errorf("failed to load function: %v", err)
				return
			}
		}
		
		logrus.Infof("Function %s loaded and ready for execution", funcMsg.Name)
	}
}
