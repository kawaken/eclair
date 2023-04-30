package main

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

type MP4Event struct {
	fsnotify.Event
	OccurredAt time.Time
	Expiration time.Duration
}

func NewMP4Event(e fsnotify.Event) *MP4Event {
	return &MP4Event{
		Event:      e,
		OccurredAt: time.Now(),
		Expiration: DefaultExpiration,
	}
}

func (me *MP4Event) String() string {
	return fmt.Sprintf("occurred: %s %s", me.OccurredAt, me.Event)
}

func (me *MP4Event) IsNone() bool {
	return me.OccurredAt.IsZero()
}

func (e *MP4Event) IsExpired() bool {
	expirationTime := e.OccurredAt.Add(e.Expiration)
	currentTime := time.Now()
	return !e.IsNone() && currentTime.After(expirationTime)
}

type MP4Events struct {
	mu     sync.Mutex
	events map[string]*MP4Event
}

func NewMP4Events() *MP4Events {
	return &MP4Events{
		events: make(map[string]*MP4Event),
	}
}

func (e *MP4Events) Lock(f func(*MP4Events)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f(e)
}

func (e *MP4Events) Set(event fsnotify.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	me := NewMP4Event(event)
	e.events[event.Name] = me
}

func (e *MP4Events) Get(event fsnotify.Event) (*MP4Event, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	me, ok := e.events[event.Name]
	return me, ok
}

func (e *MP4Events) Remove(event fsnotify.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.events, event.Name)
}

func (e *MP4Events) VerifyExpiredEvents() []*MP4Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	results := make([]*MP4Event, 0)
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

func (e *MP4Events) Len() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.events)
}
