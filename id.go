package fastcgi

import (
	"sync"
)

type ID struct {
	Id   uint16
	Lock *sync.RWMutex
}

func (id *ID) getId() uint16 {
	id.Lock.Lock()
	defer id.Lock.Unlock()
	var n = id.Id
	id.Id++
	if id.Id >= FCGI_MAX_ID {
		id.Id = 0
	}
	return n
}
