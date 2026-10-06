package broker

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSSEBroker_SubscribeAndPublish(t *testing.T) {
	b := NewSSEBroker()

	ch1, unsub1 := b.Subscribe("item_4022")
	defer unsub1()

	ch2, unsub2 := b.Subscribe("item_4022")
	defer unsub2()

	chOther, unsubOther := b.Subscribe("item_4021")
	defer unsubOther()

	b.Publish("item_4022")

	select {
	case itemID := <-ch1:
		assert.Equal(t, "item_4022", itemID)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for ch1 event")
	}

	select {
	case itemID := <-ch2:
		assert.Equal(t, "item_4022", itemID)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for ch2 event")
	}

	select {
	case <-chOther:
		t.Fatal("chOther should not receive event for item_4022")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSSEBroker_MultiplePublishes(t *testing.T) {
	b := NewSSEBroker()

	ch1, unsub1 := b.Subscribe("item_4022")
	defer unsub1()

	ch2, unsub2 := b.Subscribe("item_4022")
	defer unsub2()

	b.Publish("item_4022")
	assert.Equal(t, "item_4022", <-ch1)
	assert.Equal(t, "item_4022", <-ch2)

	b.Publish("item_4022")
	assert.Equal(t, "item_4022", <-ch1)
	assert.Equal(t, "item_4022", <-ch2)
}

func TestSSEBroker_Unsubscribe(t *testing.T) {
	b := NewSSEBroker()

	ch, unsub := b.Subscribe("item_4022")
	unsub()

	b.Publish("item_4022")

	select {
	case <-ch:
		t.Fatal("Unsubscribed channel should not receive event")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSSEBroker_RaceCondition(t *testing.T) {
	b := NewSSEBroker()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			itemID := "item_4022"
			if id%2 == 0 {
				itemID = "item_4021"
			}

			ch, unsub := b.Subscribe(itemID)
			go func() {
				time.Sleep(10 * time.Millisecond)
				unsub()
			}()

			timeout := time.After(50 * time.Millisecond)
			for {
				select {
				case <-ch:
				case <-timeout:
					return
				}
			}
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			itemID := "item_4022"
			if id%2 == 0 {
				itemID = "item_4021"
			}
			b.Publish(itemID)
		}(i)
	}

	wg.Wait()
}
