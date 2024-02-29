package flows

import (
	"context"
)

type testStorage struct {
	store map[string]struct{}
}

func newTestStorage(store map[string]struct{}) *testStorage {
	if store == nil {
		store = make(map[string]struct{})
	}
	return &testStorage{
		store: store,
	}
}

func (s *testStorage) Put(key string) {
	s.store[key] = struct{}{}
}
func (s *testStorage) Exists(key string) bool {
	_, ok := s.store[key]
	return ok
}
func (s *testStorage) Clear(context.Context) error {
	s.store = make(map[string]struct{})
	return nil
}
