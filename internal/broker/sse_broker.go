package broker

import (
	"sync"
)

type SSEBroker interface {
	Subscribe(itemID string) (<-chan string, func())
	Publish(itemID string)
}

type sseBroker struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan string]struct{}
}

func NewSSEBroker() SSEBroker {
	return &sseBroker{
		subscribers: make(map[string]map[chan string]struct{}),
	}
}

func (b *sseBroker) Subscribe(itemID string) (<-chan string, func()) {
	ch := make(chan string, 10)

	b.mu.Lock()
	if b.subscribers[itemID] == nil {
		b.subscribers[itemID] = make(map[chan string]struct{})
	}
	b.subscribers[itemID][ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			if subMap, ok := b.subscribers[itemID]; ok {
				delete(subMap, ch)
				if len(subMap) == 0 {
					delete(b.subscribers, itemID)
				}
			}
			b.mu.Unlock()
			close(ch)
		})
	}

	return ch, unsubscribe
}

func (b *sseBroker) Publish(itemID string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subMap, ok := b.subscribers[itemID]
	if !ok || len(subMap) == 0 {
		return
	}

	for ch := range subMap {
		select {
		case ch <- itemID:
		default:
		}
	}
}
