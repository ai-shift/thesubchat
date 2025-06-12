package schats

import (
	"github.com/google/uuid"
	"sync"
)

type StreamedChats struct {
	l sync.RWMutex
	c map[uuid.UUID]*Stream
}

type Stream struct {
	Chunks chan string
	Done   chan struct{}
}

func New() *StreamedChats {
	return &StreamedChats{
		c: make(map[uuid.UUID]*Stream),
	}
}

func (s *StreamedChats) Alloc(id uuid.UUID) *Stream {
	st := &Stream{
		Chunks: make(chan string, 100),
		Done:   make(chan struct{}),
	}
	s.l.Lock()
	defer s.l.Unlock()
	s.c[id] = st
	return st
}

func (s *StreamedChats) Get(id uuid.UUID) (st *Stream, ok bool) {
	s.l.RLock()
	defer s.l.RUnlock()
	st, ok = s.c[id]
	return
}
