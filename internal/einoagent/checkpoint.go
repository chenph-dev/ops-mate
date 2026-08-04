package einoagent

import (
	"context"
	"sync"
)

// memCheckpointStore 是内存版 eino CheckPointStore。
// 每 Graph 一个实例，按 sessionID 建键（一个实例可承载多个会话的 checkpoint）。
// 同时实现 CheckPointDeleter，支持轮次结束后清理。
// 局限：应用重启后审批中断态丢失（未批准命令作废），设计文档已声明可接受。
type memCheckpointStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newMemCheckpointStore() *memCheckpointStore {
	return &memCheckpointStore{m: map[string][]byte{}}
}

func (s *memCheckpointStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[id]
	return v, ok, nil
}

func (s *memCheckpointStore) Set(_ context.Context, id string, checkpoint []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = checkpoint
	return nil
}

func (s *memCheckpointStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}
