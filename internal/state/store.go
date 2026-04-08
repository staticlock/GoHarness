package state

import "sync"

// Listener observes state changes.
type Listener func(AppState)

// Store is a small observable state store.
type Store struct {
	mu        sync.RWMutex
	state     AppState
	listeners map[int]Listener
	nextID    int
}

// NewStore creates a state store with an initial snapshot.
func NewStore(initial AppState) *Store {
	return &Store{state: initial, listeners: map[int]Listener{}}
}

// Get returns the current state snapshot.
func (s *Store) Get() AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Set replaces the entire state and notifies listeners.
func (s *Store) Set(next AppState) AppState {
	s.mu.Lock()
	s.state = next
	listeners := make([]Listener, 0, len(s.listeners))
	for _, l := range s.listeners {
		listeners = append(listeners, l)
	}
	s.mu.Unlock()

	for _, listener := range listeners {
		listener(next)
	}
	return next
}

// Update applies a mutator to current state and notifies listeners.
func (s *Store) Update(mutator func(AppState) AppState) AppState {
	return s.Set(mutator(s.Get()))
}

// Subscribe registers a listener and returns an unsubscribe callback.
func (s *Store) Subscribe(listener Listener) func() {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.listeners[id] = listener
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		delete(s.listeners, id)
		s.mu.Unlock()
	}
}
