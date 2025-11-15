package task

import (
	"context"
	"fmt"
	"sync"

	"github.com/9triver/iarnet/internal/domain/ignis/types"
	"github.com/9triver/iarnet/internal/domain/ignis/utils"
	"github.com/9triver/iarnet/internal/domain/resource/component"
	actorpb "github.com/9triver/iarnet/internal/proto/ignis/actor"
	componentpb "github.com/9triver/iarnet/internal/proto/resource/component"
	"github.com/sirupsen/logrus"
	pb "google.golang.org/protobuf/proto"
)

type Actor struct {
	info      *actorpb.ActorInfo
	id        types.ActorID
	component *component.Component
}

func NewActor(id types.ActorID, component *component.Component) *Actor {
	return &Actor{
		id:        id,
		component: component,
		info: &actorpb.ActorInfo{
			CalcLatency: 0,
			LinkLatency: 0,
		},
	}
}

func (a *Actor) GetID() types.ActorID {
	return a.id
}

func (a *Actor) Send(msg pb.Message) error {
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

type ActorGroup struct {
	name string
	pq   utils.PQueue[*Actor]
	cond *sync.Cond
}

func (g *ActorGroup) Select() *Actor {
	g.cond.L.Lock()
	defer g.cond.L.Unlock()

	for g.pq.Len() == 0 {
		g.cond.Wait()
	}

	return g.pq.Pop()
}

func (g *ActorGroup) Push(actor *Actor) {
	g.cond.L.Lock()
	g.pq.Push(actor)
	g.cond.L.Unlock()

	g.cond.Signal()
}

func NewGroup(name string, candidates ...*Actor) *ActorGroup {
	return &ActorGroup{
		name: name,
		pq: utils.MakePriorityQueue(func(i, j *Actor) bool {
			return i.GetLinkLatency()*2+i.GetCalcLatency() < j.GetLinkLatency()*2+j.GetCalcLatency()
		}, candidates...),
		cond: sync.NewCond(&sync.Mutex{}),
	}
}
