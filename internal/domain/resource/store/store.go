package store

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	// 唤醒等待该对象的线程
	if s.cond != nil {
		s.cond.Broadcast()
	}
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
	return s.GetObjectWithContext(context.Background(), id)
}

// GetObjectWithContext 获取对象，如果对象不存在则等待一段时间
// sync.Cond.Wait() 本身不支持超时，这里通过结合 context 和定时器来实现超时等待
func (s *Store) GetObjectWithContext(ctx context.Context, id types.ObjectID) (object.Interface, error) {
	// 确保条件变量已初始化
	s.mu.Lock()
	if s.cond == nil {
		s.cond = sync.NewCond(&s.mu)
	}

	// 首先尝试获取对象
	if obj, ok := s.objects[id]; ok {
		s.mu.Unlock()
		return obj, nil
	}

	// 对象不存在，等待一段时间
	// 设置默认超时时间为 30 秒
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
		if timeout <= 0 {
			s.mu.Unlock()
			return nil, fmt.Errorf("object not found: %s (context deadline exceeded)", id)
		}
	}

	// 创建超时 timer
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// 启动 goroutine 来处理超时和 context 取消
	// 当超时或 context 被取消时，通过 Broadcast 唤醒所有等待的线程
	go func() {
		select {
		case <-timer.C:
			// 超时，唤醒等待的线程
			s.mu.Lock()
			s.cond.Broadcast()
			s.mu.Unlock()
		case <-ctx.Done():
			// Context 被取消，唤醒等待的线程
			s.mu.Lock()
			s.cond.Broadcast()
			s.mu.Unlock()
		}
	}()

	// 循环等待对象出现
	for {
		// 再次检查对象是否存在（可能在等待期间被保存）
		if obj, ok := s.objects[id]; ok {
			s.mu.Unlock()
			return obj, nil
		}

		// 检查是否超时或 context 被取消（在调用 Wait 之前检查）
		select {
		case <-timer.C:
			// 超时，再次检查对象是否存在
			if obj, ok := s.objects[id]; ok {
				s.mu.Unlock()
				return obj, nil
			}
			s.mu.Unlock()
			return nil, fmt.Errorf("object not found: %s (timeout)", id)
		case <-ctx.Done():
			// Context 被取消，再次检查对象是否存在
			if obj, ok := s.objects[id]; ok {
				s.mu.Unlock()
				return obj, nil
			}
			s.mu.Unlock()
			return nil, fmt.Errorf("object not found: %s (context canceled)", id)
		default:
			// 没有超时，继续等待
		}

		// 等待条件变量被唤醒（Wait 会自动释放锁，唤醒后重新获取锁）
		// 注意：Wait() 必须在持有锁的情况下调用，并且会阻塞直到被唤醒
		// 超时 goroutine 会通过 Broadcast() 唤醒所有等待的线程
		s.cond.Wait()
		// 被唤醒后，继续循环检查对象是否存在
	}
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
