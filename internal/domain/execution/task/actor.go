package task

import (
	"context"
	"fmt"
	"sync"

	"github.com/9triver/iarnet/internal/domain/execution/types"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
	componentpb "github.com/9triver/iarnet/internal/proto/resource/component"
	"github.com/sirupsen/logrus"
	pb "google.golang.org/protobuf/proto"
)

const (
	// DefaultMaxConcurrency 默认最大并发数
	DefaultMaxConcurrency = 8
)

type Actor struct {
	info      *actorpb.ActorInfo
	id        types.ActorID
	component *component.Component

	// 并发控制
	mu             sync.Mutex
	maxConcurrency int                       // 最大并发数
	activeCount    int                       // 当前正在处理的请求数
	pendingQueue   []pendingRequest          // 等待队列
	onTaskDoneFunc func(ctx context.Context) // 任务完成回调
}

type pendingRequest struct {
	msg pb.Message
	ctx context.Context
}

func NewActor(id types.ActorID, component *component.Component) *Actor {
	return &Actor{
		id:             id,
		component:      component,
		maxConcurrency: DefaultMaxConcurrency,
		info: &actorpb.ActorInfo{
			CalcLatency: 0,
			LinkLatency: 0,
		},
		pendingQueue: make([]pendingRequest, 0),
	}
}

// SetMaxConcurrency 设置最大并发数
func (a *Actor) SetMaxConcurrency(max int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.maxConcurrency = max
}

func (a *Actor) GetID() types.ActorID {
	return a.id
}

// Send 发送消息到 actor，如果未达到并发上限则立即发送，否则加入等待队列
func (a *Actor) Send(msg pb.Message) error {
	return a.SendWithContext(context.Background(), msg)
}

// SendWithContext 发送消息到 actor（带上下文）
func (a *Actor) SendWithContext(ctx context.Context, msg pb.Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 如果未达到并发上限，立即发送
	if a.activeCount < a.maxConcurrency {
		a.activeCount++
		a.mu.Unlock()

		// 发送消息
		if err := a.sendMessage(msg); err != nil {
			a.mu.Lock()
			a.activeCount--
			a.mu.Unlock()
			return err
		}

		a.mu.Lock()
		logrus.WithFields(logrus.Fields{
			"actor":         a.id,
			"active_count":  a.activeCount,
			"pending_count": len(a.pendingQueue),
		}).Debug("task: sent message immediately")
		return nil
	}

	// 达到并发上限，加入等待队列
	a.pendingQueue = append(a.pendingQueue, pendingRequest{
		msg: msg,
		ctx: ctx,
	})
	logrus.WithFields(logrus.Fields{
		"actor":         a.id,
		"active_count":  a.activeCount,
		"pending_count": len(a.pendingQueue),
	}).Debug("task: queued message (concurrency limit reached)")

	return nil
}

// sendMessage 实际发送消息（不加锁，需要在外部加锁）
func (a *Actor) sendMessage(msg pb.Message) error {
	actorMsg := actorpb.NewMessage(msg)
	if actorMsg == nil {
		return fmt.Errorf("failed to create actor message")
	}
	componentMsg, err := componentpb.NewPayload(actorMsg)
	if err != nil {
		return err
	}
	a.component.Send(componentMsg)
	return nil
}

// OnTaskDone 当任务完成时调用，从等待队列中取出下一个任务继续执行
func (a *Actor) OnTaskDone(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 减少活跃计数
	if a.activeCount > 0 {
		a.activeCount--
	}

	// 如果等待队列不为空且未达到并发上限，取出下一个任务
	if len(a.pendingQueue) > 0 && a.activeCount < a.maxConcurrency {
		// 取出队列头部的任务
		pending := a.pendingQueue[0]
		a.pendingQueue = a.pendingQueue[1:]

		// 增加活跃计数
		a.activeCount++

		// 异步发送消息，避免阻塞
		go func(pendingReq pendingRequest) {
			if err := a.sendMessage(pendingReq.msg); err != nil {
				logrus.WithFields(logrus.Fields{
					"actor": a.id,
					"error": err,
				}).Error("task: failed to send queued message")
				// 发送失败，减少活跃计数并尝试发送下一个任务
				a.mu.Lock()
				if a.activeCount > 0 {
					a.activeCount--
				}
				// 尝试发送队列中的下一个任务
				if len(a.pendingQueue) > 0 && a.activeCount < a.maxConcurrency {
					nextPending := a.pendingQueue[0]
					a.pendingQueue = a.pendingQueue[1:]
					a.activeCount++
					a.mu.Unlock()
					// 递归发送下一个任务
					go func() {
						if err := a.sendMessage(nextPending.msg); err == nil {
							logrus.WithFields(logrus.Fields{
								"actor": a.id,
							}).Debug("task: sent next queued message after failure recovery")
						} else {
							// 再次失败，减少计数
							a.mu.Lock()
							if a.activeCount > 0 {
								a.activeCount--
							}
							a.mu.Unlock()
						}
					}()
				} else {
					a.mu.Unlock()
				}
			} else {
				// 发送成功，记录日志（需要加锁读取计数）
				a.mu.Lock()
				activeCount := a.activeCount
				pendingCount := len(a.pendingQueue)
				a.mu.Unlock()
				logrus.WithFields(logrus.Fields{
					"actor":         a.id,
					"active_count":  activeCount,
					"pending_count": pendingCount,
				}).Debug("task: sent queued message")
			}
		}(pending)
	}

	logrus.WithFields(logrus.Fields{
		"actor":         a.id,
		"active_count":  a.activeCount,
		"pending_count": len(a.pendingQueue),
	}).Debug("task: task done, processed queue")
}

// GetActiveCount 获取当前活跃请求数
func (a *Actor) GetActiveCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeCount
}

// GetPendingCount 获取等待队列长度
func (a *Actor) GetPendingCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pendingQueue)
}

// GetMaxConcurrency 获取最大并发数
func (a *Actor) GetMaxConcurrency() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.maxConcurrency
}

func (a *Actor) Receive(ctx context.Context) *actorpb.Message {
	msg := a.component.Receive(ctx)
	if msg == nil {
		return nil
	}
	if msg.Type != componentpb.MessageType_PAYLOAD {
		logrus.Errorf("unexpected message type: %T", msg)
		return nil
	}
	payload := msg.GetPayloadMessage()
	switch payload := payload.(type) {
	case *actorpb.Message:
		return payload
	default:
		logrus.Errorf("unexpected message type: %T", payload)
		return nil
	}
}

func (a *Actor) GetInfo() *actorpb.ActorInfo {
	return a.info
}

func (a *Actor) GetLinkLatency() int64 {
	return a.info.LinkLatency
}

func (a *Actor) GetCalcLatency() int64 {
	return a.info.CalcLatency
}

func (a *Actor) GetComponent() *component.Component {
	return a.component
}

type ActorGroup struct {
	name      string
	actors    []*Actor                 // 使用切片存储，支持轮询
	actorsMap map[types.ActorID]*Actor // 快速查找
	mu        sync.Mutex
	index     int // 当前轮询索引
	cond      *sync.Cond
}

// Select 使用轮询方式选择 actor
func (g *ActorGroup) Select() *Actor {
	g.cond.L.Lock()
	defer g.cond.L.Unlock()

	// 等待至少有一个 actor
	for len(g.actors) == 0 {
		g.cond.Wait()
	}

	// 轮询选择：从当前索引开始，找到第一个可用的 actor
	startIndex := g.index
	for {
		actor := g.actors[g.index]
		// 检查 actor 是否还有并发容量（未达到最大并发数）
		// 注意：这里只检查是否有容量，不要求完全空闲
		// 因为 actor 可以并发处理多个请求
		if actor.GetActiveCount() < actor.GetMaxConcurrency() {
			// 更新索引为下一个位置
			g.index = (g.index + 1) % len(g.actors)
			return actor
		}

		// 当前 actor 已满，尝试下一个
		g.index = (g.index + 1) % len(g.actors)

		// 如果已经遍历完所有 actor，等待一下再重试
		if g.index == startIndex {
			// 所有 actor 都满了，等待条件变量
			g.cond.Wait()
			startIndex = g.index
		}
	}
}

func (g *ActorGroup) Push(actor *Actor) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 检查是否已存在
	if _, exists := g.actorsMap[actor.id]; !exists {
		g.actors = append(g.actors, actor)
		g.actorsMap[actor.id] = actor
	}

	g.cond.Signal()
}

func NewGroup(name string, candidates ...*Actor) *ActorGroup {
	actors := make([]*Actor, 0, len(candidates))
	actorsMap := make(map[types.ActorID]*Actor, len(candidates))
	for _, actor := range candidates {
		actors = append(actors, actor)
		actorsMap[actor.id] = actor
	}

	return &ActorGroup{
		name:      name,
		actors:    actors,
		actorsMap: actorsMap,
		index:     0,
		cond:      sync.NewCond(&sync.Mutex{}),
	}
}

func (g *ActorGroup) GetAll() []*Actor {
	g.mu.Lock()
	defer g.mu.Unlock()

	actors := make([]*Actor, len(g.actors))
	copy(actors, g.actors)
	return actors
}
