package stream

import (
	"sync"
)

type Stream[T any] struct {
	name     string
	elements []T
	closed   bool
	*sync.Cond
}

func NewStream[T any](name string) *Stream[T] {
	return &Stream[T]{
		Cond: sync.NewCond(&sync.Mutex{}),
		name: name,
	}
}

func (s *Stream[T]) Push(msg T) {
	s.Cond.L.Lock()
	defer s.Cond.L.Unlock()
	if s.closed {
		return
	}
	s.elements = append(s.elements, msg)
	s.Cond.Signal()
}

func (s *Stream[T]) Pull() []T {
	for {
		s.Cond.L.Lock()
		if len(s.elements) == 0 && !s.closed {
			s.Cond.Wait()
			s.Cond.L.Unlock()
			continue
		} else {
			msgs := s.elements
			s.elements = []T{}
			s.Cond.L.Unlock()
			return msgs
		}
	}
}

func (s *Stream[T]) TryPull() []T {
	s.Cond.L.Lock()
	defer s.Cond.L.Unlock()
	msgs := s.elements
	s.elements = []T{}
	return msgs
}

func (s *Stream[T]) Close() {
	s.Cond.L.Lock()
	s.closed = true
	s.Cond.Signal()
	s.Cond.L.Unlock()
}

func (s *Stream[T]) Closed() bool {
	s.Cond.L.Lock()
	closed := s.closed
	s.Cond.L.Unlock()
	return closed
}
