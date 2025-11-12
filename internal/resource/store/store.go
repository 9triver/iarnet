package store

import (
	"fmt"

	"github.com/9triver/iarnet/internal/resource"
	"github.com/9triver/iarnet/internal/resource/store/object"
)

type Store struct {
	objects map[string]object.Interface
}

func NewStore() *Store {
	return &Store{
		objects: make(map[string]object.Interface),
	}
}

func (s *Store) SaveObject(obj object.Interface) {
	s.objects[obj.GetID()] = obj
}

func (s *Store) GetObject(id resource.ObjectID) (object.Interface, error) {
	obj, ok := s.objects[id]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	return obj, nil
}
