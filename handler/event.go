package handler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	DefaultExpiration = 2 * time.Minute
)

type Event struct {
	fsnotify.Event
	OccurredAt time.Time
	Expiration time.Duration
}

func NewEvent(e fsnotify.Event) *Event {
	return &Event{
		Event:      e,
		OccurredAt: time.Now(),
		Expiration: DefaultExpiration,
	}
}

func (me *Event) String() string {
	return fmt.Sprintf("occurred: %s %s", me.OccurredAt, me.Event)
}

func (me *Event) IsNone() bool {
	return me.OccurredAt.IsZero()
}

func (e *Event) IsExpired() bool {
	expirationTime := e.OccurredAt.Add(e.Expiration)
	currentTime := time.Now()
	return !e.IsNone() && currentTime.After(expirationTime)
}

type Events struct {
	mu     sync.Mutex
	events map[string]*Event
}

func NewEvents() *Events {
	return &Events{
		events: make(map[string]*Event),
	}
}

func (e *Events) Lock(f func(*Events)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f(e)
}

func (e *Events) Set(event fsnotify.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	me := NewEvent(event)
	e.events[event.Name] = me
}

func (e *Events) Get(event fsnotify.Event) (*Event, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	me, ok := e.events[event.Name]
	return me, ok
}

func (e *Events) Remove(event fsnotify.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.events, event.Name)
}

func (e *Events) VerifyExpiredEvents() []*Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	results := make([]*Event, 0)
	deletion := make([]string, 0)

	for k, v := range e.events {
		expired := v.IsExpired()
		if expired {
			log.Printf("Expired: %s, %s", k, v.OccurredAt)
			results = append(results, v)
			deletion = append(deletion, k)
		}
	}

	for _, k := range deletion {
		delete(e.events, k)
	}

	return results
}

func (e *Events) Len() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.events)
}
