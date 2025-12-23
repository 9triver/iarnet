package store

import (
	"fmt"
	"sync"

	"github.com/9triver/iarnet/internal/domain/resource/store/object"
	"github.com/9triver/iarnet/internal/domain/resource/types"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	"github.com/9triver/iarnet/internal/util"
)

type Store struct {
	id           types.StoreID
	objects      map[types.ObjectID]object.Interface
	streamChunks map[string]map[int64]*commonpb.StreamChunk
	mu           sync.Mutex
	cond         *sync.Cond
}

func NewStore() *Store {
	s := &Store{
		id:      util.GenIDWith("store."),
		objects: make(map[types.ObjectID]object.Interface),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *Store) GetID() types.StoreID {
	return s.id
}

func (s *Store) SaveObject(obj object.Interface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[obj.GetID()] = obj
}

func (s *Store) SaveStreamChunk(chunk *commonpb.StreamChunk) error {
	// TODO: 锁的粒度细化
	if chunk == nil {
		return fmt.Errorf("stream chunk is nil")
	}
	if chunk.ObjectID == "" {
		return fmt.Errorf("stream chunk missing object id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.streamChunks == nil {
		s.streamChunks = make(map[string]map[int64]*commonpb.StreamChunk)
	}

	if _, ok := s.streamChunks[chunk.ObjectID]; !ok {
		s.streamChunks[chunk.ObjectID] = make(map[int64]*commonpb.StreamChunk)
	}
	s.streamChunks[chunk.ObjectID][chunk.Offset] = chunk

	if s.cond != nil {
		s.cond.Broadcast()
	}

	return nil
}

func (s *Store) GetObject(id types.ObjectID) (object.Interface, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[id]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	return obj, nil
}

func (s *Store) GetStreamChunk(objectID types.ObjectID, offset int64) (*commonpb.StreamChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cond == nil {
		s.cond = sync.NewCond(&s.mu)
	}

	for {
		if chunk := s.lookupChunkLocked(objectID, offset); chunk != nil {
			return chunk, nil
		}
		s.cond.Wait()
	}
}

func (s *Store) lookupChunkLocked(objectID types.ObjectID, offset int64) *commonpb.StreamChunk {
	if s.streamChunks == nil {
		return nil
	}
	if offsets, ok := s.streamChunks[objectID]; ok {
		if chunk, exists := offsets[offset]; exists {
			return chunk
		}
	}
	return nil
}
