package schats

import (
	"github.com/google/uuid"
	"sync"
)

type StreamedChats struct {
	l sync.RWMutex
	c map[uuid.UUID]chan string
}

func New() *StreamedChats {
	return &StreamedChats{
		c: make(map[uuid.UUID]chan string),
	}
}

func (s *StreamedChats) Alloc(id uuid.UUID) chan string {
	c := make(chan string, 100)
	s.l.Lock()
	defer s.l.Unlock()
	s.c[id] = c
	return c
}

func (s *StreamedChats) Free(id uuid.UUID) {
	s.l.Lock()
	defer s.l.Unlock()
	s.c[id] = nil
}

func (s *StreamedChats) Get(id uuid.UUID) (c chan string, ok bool) {
	s.l.RLock()
	defer s.l.RUnlock()
	c, ok = s.c[id]
	return
}
