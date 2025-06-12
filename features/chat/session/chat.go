package session

import (
	"github.com/google/uuid"
	"sync"
)

type ChatStream struct {
	L sync.RWMutex
	C map[uuid.UUID]chan string
}

func New() *ChatStream {
	panic("not implemented")
}
