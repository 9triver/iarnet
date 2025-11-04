package deployer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/9triver/iarnet/internal/application"
	"github.com/9triver/iarnet/internal/config"
	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/ignis/actor/router"
	"github.com/9triver/ignis/actor/store"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/proto/controller"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/sirupsen/logrus"
	pb "google.golang.org/protobuf/proto"
)

// Deployer 负责部署和管理函数副本的部署器
// 它协调应用管理器、资源管理器、连接管理器和配置
type Deployer struct {
	am  *application.Manager // 应用管理器，用于注册组件
	rm  *resource.Manager    // 资源管理器，用于部署容器
	cm  *ConnectionManager   // 连接管理器，用于管理与容器的连接
	cfg *config.Config       // 配置信息，包含镜像和地址信息
}

// NewDeployer 创建一个新的部署器实例
func NewDeployer(am *application.Manager, rm *resource.Manager, cm *ConnectionManager, cfg *config.Config) *Deployer {
	return &Deployer{
		am:  am,
		rm:  rm,
		cm:  cm,
		cfg: cfg,
	}
}

// DeployPyFunc 部署 Python 函数的多个副本
// 参数:
//   - ctx: actor 上下文
//   - appId: 应用 ID
//   - f: Python 函数配置，包含函数代码、依赖和资源需求
//   - sr: 存储引用
//
// 返回:
//   - []*proto.ActorInfo: 部署的 actor 信息列表
//   - error: 错误信息
func (d *Deployer) DeployPyFunc(ctx actor.Context, appId string, f *controller.AppendPyFunc, sr *proto.StoreRef) ([]*proto.ActorInfo, error) {
	// 参数验证
	if f == nil {
		return nil, fmt.Errorf("function configuration is nil")
	}
	if f.Replicas <= 0 {
		return nil, fmt.Errorf("invalid replicas count: %d", f.Replicas)
	}
	if f.Resources == nil {
		return nil, fmt.Errorf("function resources configuration is nil")
	}

	// 获取 Python 环境的容器镜像
	image, ok := d.cfg.ComponentImages["python"]
	if !ok {
		return nil, fmt.Errorf("actor image not found for environment: %s", "python")
	}

	// 创建函数消息，包含函数定义和参数
	funcMsg := cluster.NewFunction(f.Name, f.Params,
		f.Requirements, f.PickledObject, f.Language)

	infos := make([]*proto.ActorInfo, f.Replicas)
	var deployedCount int32 // 用于跟踪已部署的副本数量，失败时进行清理

	ctx.Logger().Info("deploying function replicas", "name", f.Name, "replicas", f.Replicas)

	// 为每个副本创建独立的容器和连接
	for i := range f.Replicas {
		connId := fmt.Sprintf("%s:%s-%d", appId, f.Name, i)

		// 创建与容器的连接流
		stream := d.cm.NewConn(context.Background(), connId)
		// 发送函数定义消息给容器
		stream.SendChan() <- funcMsg

		// 部署容器，配置资源需求和环境变量
		cf, err := d.rm.Deploy(context.Background(), resource.ContainerSpec{
			Image: image,
			Requirements: resource.Info{
				CPU:    f.Resources.CPU,
				Memory: f.Resources.Memory,
				GPU:    f.Resources.GPU,
			},
			Env: map[string]string{
				"IGNIS_ADDR": d.cfg.ExternalAddr + ":" + strconv.Itoa(int(25565)),
				"CONN_ID":    connId,
			},
		})
		if err != nil {
			// TODO: 部署失败时应清理已创建的资源（已部署的容器、连接等）
			return nil, fmt.Errorf("failed to deploy replica %d: %w", i, err)
		}

		ctx.Logger().Info("deployed container", "provider", cf.Provider.GetID(), "container_id", cf.ID, "conn_id", connId)
		d.am.RegisterComponent(appId, connId, cf)
		deployedCount++

		ctx.Logger().Debug("spawning actor stub for connection", "conn_id", connId)

		// 创建 actor stub 用于处理与容器的消息交互
		pid := ctx.Spawn(actor.PropsFromProducer(func() actor.Actor {
			return NewStub(stream)
		}))

		ctx.Logger().Debug("registering routes for actor", "conn_id", connId, "pid", pid)

		// 注册路由，使消息能够路由到对应的 actor
		router.Register(connId, pid)
		router.Register("store-"+connId, pid)

		// 启动消息接收协程，处理从容器返回的消息
		// 注意：这里捕获 connId 和 pid 的值，避免闭包问题
		go func(connId string, pid *actor.PID, stream *ClusterStreamImpl) {
			defer func() {
				// 恢复 panic，防止单个 goroutine 崩溃影响整个程序
				if r := recover(); r != nil {
					logrus.Errorf("panic in message receiver goroutine for %s: %v", connId, r)
				}
				stream.Close()
			}()

			// 持续接收并处理来自容器的消息
			for msg := range stream.RecvChan() {
				m := msg.Unwrap()

				// 如果消息需要回复，注册回复路由
				if mt, ok := m.(store.RequiresReplyMessage); ok {
					router.RegisterIfAbsent(mt.GetReplyTo(), pid)
				}

				// 如果是转发消息，转发到目标 actor
				if mt, ok := m.(store.ForwardMessage); ok {
					router.Send(ctx, mt.GetTarget(), mt)
				} else {
					logrus.Warnf("unsupported message type from %s: %+v", connId, m)
				}
			}
		}(connId, pid, stream)

		// 创建 actor 信息并添加到结果列表
		info := &proto.ActorInfo{
			Ref: &proto.ActorRef{
				ID:    connId,
				PID:   pid,
				Store: sr,
			},
			CalcLatency: 0,
			LinkLatency: 0,
		}
		infos[i] = info
	}

	ctx.Logger().Info("successfully deployed all function replicas", "name", f.Name, "count", deployedCount)
	return infos, nil
}

// Stub 是一个 actor，作为本地 actor 系统与远程容器之间的桥梁
// 它接收本地消息并通过 stream 转发给远程容器
type Stub struct {
	stream *ClusterStreamImpl // 与远程容器的连接流
}

// NewStub 创建一个新的 Stub actor 实例
func NewStub(stream *ClusterStreamImpl) *Stub {
	return &Stub{
		stream: stream,
	}
}

// Receive 实现 actor.Actor 接口，处理接收到的消息
// 它将 protobuf 消息转换为集群消息格式并发送给远程容器
func (s *Stub) Receive(ctx actor.Context) {
	// 检查消息是否为 protobuf 消息
	if msg, ok := ctx.Message().(pb.Message); ok {
		// 将 protobuf 消息转换为集群消息格式
		m := cluster.NewMessage(msg)
		if m == nil {
			ctx.Logger().Warn("failed to create cluster message from protobuf", "msg_type", fmt.Sprintf("%T", msg))
		} else {
			// 通过流发送消息到远程容器
			s.stream.SendChan() <- m
		}
	} else {
		// 非 protobuf 消息类型，记录警告
		ctx.Logger().Warn("received non-protobuf message", "msg_type", fmt.Sprintf("%T", ctx.Message()))
	}
}
