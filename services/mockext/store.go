package main

import (
	"sync"
	"time"
)

const maxPerKind = 200

// Entry is one thing the mock received or did, shown in the Console.
type Entry struct {
	At    time.Time
	Kind  string
	Title string
	Body  string
	Meta  map[string]string
}

// Store keeps the newest entries per kind in memory (lost on restart).
type Store struct {
	mu    sync.Mutex
	items map[string][]Entry // oldest first
}

func NewStore() *Store { return &Store{items: map[string][]Entry{}} }

// Add records e under kind, dropping the oldest beyond 200.
func (s *Store) Add(kind string, e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.Kind = kind
	if e.At.IsZero() {
		e.At = time.Now()
	}
	l := append(s.items[kind], e)
	if len(l) > maxPerKind {
		l = l[len(l)-maxPerKind:]
	}
	s.items[kind] = l
}

// List returns entries of kind, newest first.
func (s *Store) List(kind string) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.items[kind]
	out := make([]Entry, len(l))
	for i, e := range l {
		out[len(l)-1-i] = e
	}
	return out
}
