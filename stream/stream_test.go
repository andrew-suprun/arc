package stream

import (
	"sync"
	"testing"
)

func TestStream(t *testing.T) {
	s := NewStream[string]("test")
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		t.Log(s.Pull())
		t.Log(s.TryPull())
		t.Log(s.TryPull())
		wg.Done()

	}()
	s.Push("a")
	s.Push("b")
	s.Push("c")
	s.Push("d")
	s.Push("e")
	s.Push("f")
	wg.Wait()
}
