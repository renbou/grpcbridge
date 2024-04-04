package main

import (
	"fmt"
	"log/slog"
	"maps"
	"math/rand"
	"slices"
	"sync"
	"time"

	pb "gotestapi/gen/proto/gotestapi/v1alpha"
)

var (
	randomUsers = []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Helen", "Irene", "John"}
	numVeggies  = func() int {
		var maxVal int32

		for _, val := range pb.Veggie_value {
			if val > maxVal {
				maxVal = val
			}
		}

		return int(maxVal)
	}()
)

const (
	chanSendTimeout     = time.Second
	randomEventInterval = time.Second * 2
	randomEventJitter   = time.Second / 3
)

type veggieShopEvent struct {
	User      string
	Added     pb.Veggie
	Removed   pb.Veggie
	Purchased map[pb.Veggie]uint32
}

type veggieShop struct {
	mu sync.RWMutex

	id         string
	createdAt  time.Time // to delete old shops
	stats      map[pb.Veggie]uint32
	eventChans []chan veggieShopEvent
	closed     chan struct{}
}

func newVeggieShop(id string) *veggieShop {
	shop := &veggieShop{
		id:        id,
		createdAt: time.Now(),
		stats:     make(map[pb.Veggie]uint32),
		closed:    make(chan struct{}),
	}

	go shop.randomEvents()

	return shop
}

func (vs *veggieShop) randomEvents() {
	slog.Debug("starting random event simulation", "id", vs.id)

	for {
		select {
		case <-time.After(randomEventInterval + time.Duration(rand.Intn(int(randomEventJitter)))):
		case <-vs.closed:
			slog.Debug("stopping random event simulation", "id", vs.id)
			return
		}

		vs.randomEvent()
	}
}

func (vs *veggieShop) close() {
	close(vs.closed)
}

func (vs *veggieShop) readStats() map[pb.Veggie]uint32 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	return maps.Clone(vs.stats)
}

func (vs *veggieShop) performPurchase(user string, purchase map[pb.Veggie]uint32) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	for veg, count := range purchase {
		vs.stats[veg] += count
	}

	vs.emitEventLocked(veggieShopEvent{
		User:      user,
		Purchased: purchase,
	})
}

func (vs *veggieShop) addEventChan() chan veggieShopEvent {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	ch := make(chan veggieShopEvent)
	vs.eventChans = append(vs.eventChans, ch)

	slog.Debug("added event chan", "id", vs.id, "chan", fmt.Sprintf("%p", ch))

	return ch
}

func (vs *veggieShop) removeEventChan(ch chan veggieShopEvent) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	i := slices.Index(vs.eventChans, ch)
	if i < 0 {
		return
	}

	vs.eventChans = slices.Delete(vs.eventChans, i, i+1)
	slog.Debug("removed event chan", "id", vs.id, "chan", fmt.Sprintf("%p", ch))
}

func (vs *veggieShop) emitEvent(event veggieShopEvent) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	vs.emitEventLocked(event)
}

func (vs *veggieShop) emitEventLocked(event veggieShopEvent) {
	for _, ch := range vs.eventChans {
		select {
		case ch <- event:
		// timeout just to avoid blocking a goroutine accidentally
		case <-time.After(chanSendTimeout):
		}
	}
}

func randomID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, 16)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

func randomUser() string {
	return randomUsers[rand.Intn(len(randomUsers))]
}

func (vs *veggieShop) randomEvent() {
	ev := veggieShopEvent{User: randomUser()}

	switch rand.Intn(3) {
	case 0:
		ev.Added = randomVeggie()
		vs.emitEvent(ev)
	case 1:
		ev.Removed = randomVeggie()
		vs.emitEvent(ev)
	default:
		ev.Purchased = randomPurchase()
		vs.performPurchase(ev.User, ev.Purchased)
	}
}

func randomVeggie() pb.Veggie {
	return pb.Veggie(rand.Intn(numVeggies) + 1) // + 1 because 0 is unspecified
}

func randomPurchase() map[pb.Veggie]uint32 {
	purchasedCount := rand.Intn(3) + 1
	veggies := make(map[pb.Veggie]uint32, purchasedCount)

	for i := 0; i < purchasedCount; i++ {
		veggies[randomVeggie()] = uint32(rand.Intn(10) + 1)
	}

	return veggies
}
