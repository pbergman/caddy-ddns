package dyndns_handler

import (
	"sync"
	"sync/atomic"
)

type WaitableLocker interface {
	sync.Locker
	Wait()
}

func NewSemaphore(x int) WaitableLocker {
	return &semaphore{
		pool: make(chan struct{}, x),
	}
}

type semaphore struct {
	pool chan struct{}
	wg   sync.WaitGroup
	size atomic.Int32
}

func (s *semaphore) Lock() {
	s.wg.Add(1)
	s.pool <- struct{}{}
	s.size.Add(1)
}

func (s *semaphore) Unlock() {
	if s.size.Load() > 0 {
		<-s.pool
		s.wg.Done()
		s.size.Add(-1)
	}
}

func (s *semaphore) Wait() {
	s.wg.Wait()
}
