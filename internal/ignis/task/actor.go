package task

import (
	"fmt"
	"sync"

	"github.com/9triver/iarnet/internal/ignis/types"
	"github.com/9triver/iarnet/internal/ignis/utils"
	proto "github.com/9triver/iarnet/internal/proto/ignis"
	clusterpb "github.com/9triver/iarnet/internal/proto/ignis/cluster"
	"github.com/9triver/iarnet/internal/resource/component"
	pb "google.golang.org/protobuf/proto"
)

type Actor struct {
	info      *proto.ActorInfo
	id        types.ActorID
	component *component.Component
}

func NewActor(id types.ActorID, component *component.Component) *Actor {
	return &Actor{
		id:        id,
		component: component,
		info: &proto.ActorInfo{
			CalcLatency: 0,
			LinkLatency: 0,
		},
	}
}

func (a *Actor) GetID() types.ActorID {
	return a.id
}

func (a *Actor) Send(msg pb.Message) error {
	clusterMsg := clusterpb.NewMessage(msg)
	if clusterMsg == nil {
		return fmt.Errorf("failed to create cluster message")
	}
	a.component.Send(clusterMsg)
	return nil
}

func (a *Actor) GetInfo() *proto.ActorInfo {
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
