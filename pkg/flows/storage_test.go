package flows

import (
	"context"
	"fmt"
)

type testStorage struct {
	store map[string]bool
}

func newTestStorage(store map[string]bool) *testStorage {
	if store == nil {
		store = make(map[string]bool)
	}
	return &testStorage{
		store: store,
	}
}

func (s *testStorage) StoreDone(name StepName) {
	s.store[fmt.Sprintf("%sDone", name)] = true
}
func (s *testStorage) StoreRun(name StepName) {
	s.store[fmt.Sprintf("%sRun", name)] = true

}
func (s *testStorage) StoreConditionResult(name StepName, result bool) {
	s.store[fmt.Sprintf("%sCond", name)] = result

}
func (s *testStorage) IsDone(name StepName) bool {
	_, ok := s.store[fmt.Sprintf("%sDone", name)]
	return ok
}
func (s *testStorage) HasRun(name StepName) bool {
	_, ok := s.store[fmt.Sprintf("%sRun", name)]
	return ok
}
func (s *testStorage) GetConditionResult(name StepName) (result bool, exists bool) {
	val, ok := s.store[fmt.Sprintf("%sCond", name)]
	return val, ok
}
func (s *testStorage) Clear(ctx context.Context) error {
	s.store = make(map[string]bool)
	return nil
}
