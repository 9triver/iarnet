package store

import (
	"fmt"

	"github.com/9triver/iarnet/internal/resource/store/object"
	"github.com/9triver/iarnet/internal/resource/types"
	"github.com/9triver/iarnet/internal/util"
)

type Store struct {
	id      types.StoreID
	objects map[types.ObjectID]object.Interface
}

func NewStore() *Store {
	return &Store{
		id:      util.GenIDWith("store."),
		objects: make(map[types.ObjectID]object.Interface),
	}
}

func (s *Store) GetID() types.StoreID {
	return s.id
}

func (s *Store) SaveObject(obj object.Interface) {
	s.objects[obj.GetID()] = obj
}

func (s *Store) GetObject(id types.ObjectID) (object.Interface, error) {
	obj, ok := s.objects[id]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	return obj, nil
}
