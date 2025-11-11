package task

import (
	"sync"

	"github.com/9triver/iarnet/internal/ignis/types"
	"github.com/9triver/iarnet/internal/resource/component"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/utils"
)

type ActorInfo struct {
	proto.ActorInfo
	ID        types.ActorID
	Component *component.Component
}

type ActorGroup struct {
	name string
	pq   utils.PQueue[*ActorInfo]
	cond *sync.Cond
}

func (g *ActorGroup) Select() *ActorInfo {
	g.cond.L.Lock()
	defer g.cond.L.Unlock()

	for g.pq.Len() == 0 {
		g.cond.Wait()
	}

	return g.pq.Pop()
}

func (g *ActorGroup) Push(info *ActorInfo) {
	g.cond.L.Lock()
	g.pq.Push(info)
	g.cond.L.Unlock()

	g.cond.Signal()
}

func NewGroup(name string, candidates ...*ActorInfo) *ActorGroup {
	return &ActorGroup{
		name: name,
		pq: utils.MakePriorityQueue(func(i, j *ActorInfo) bool {
			return i.LinkLatency*2+i.CalcLatency < j.LinkLatency*2+j.CalcLatency
		}, candidates...),
		cond: sync.NewCond(&sync.Mutex{}),
	}
}
