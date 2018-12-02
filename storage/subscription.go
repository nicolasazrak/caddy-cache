package storage

import "sync"

// Subscription is a structure to sync all File Readers in a pub/sub style.
// It's useful when there are many readers that want to read from a single writter.
// The writter can create a subscription and give each other a NewSubscriber().
// Each time the producer call NotifyAll the clients will get a channel with the value.
// In case a client blocks it will miss the next reads that will not be queued.
type Subscription struct {
	closed            bool
	closedLock        *sync.RWMutex
	subscribers       []chan int
	noSubscribersChan chan struct{}
	subscribersLock   *sync.RWMutex
}

func NewSubscription() *Subscription {
	return &Subscription{
		closedLock:        new(sync.RWMutex),
		subscribersLock:   new(sync.RWMutex),
		noSubscribersChan: make(chan struct{}, 1),
	}
}

func (s *Subscription) NewSubscriber() <-chan int {
	s.closedLock.Lock()
	defer s.closedLock.Unlock()
	if s.closed {
		subscription := make(chan int)
		close(subscription)
		return subscription
	}

	s.subscribersLock.Lock()
	defer s.subscribersLock.Unlock()
	subscription := make(chan int, 1)
	s.subscribers = append(s.subscribers, subscription)
	return subscription
}

func (s *Subscription) RemoveSubscriber(subscriber <-chan int) {
	s.subscribersLock.Lock()
	defer s.subscribersLock.Unlock()
	for i, x := range s.subscribers {
		if x == subscriber {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			break
		}
	}

	noSubscribers := len(s.subscribers) == 0
	if noSubscribers {
		select {
		case s.noSubscribersChan <- struct{}{}:
		default:
		}
	}
}

func (s *Subscription) Close() {
	s.closedLock.Lock()
	defer s.closedLock.Unlock()
	if s.closed {
		return
	}

	s.closed = true
	s.subscribersLock.RLock()
	defer s.subscribersLock.RUnlock()
	for _, subscriber := range s.subscribers {
		close(subscriber)
	}
}

func (s *Subscription) NotifyAll(newBytes int) {
	s.subscribersLock.RLock()
	defer s.subscribersLock.RUnlock()
	for _, subscriber := range s.subscribers {
		select {
		case subscriber <- newBytes:
		default:
		}
	}
}

func (s *Subscription) hasSubscribers() bool {
	s.subscribersLock.RLock()
	defer s.subscribersLock.RUnlock()
	return len(s.subscribers) != 0
}

// WaitAll waits until are subscribers ends
func (s *Subscription) WaitAll() {
	if !s.hasSubscribers() {
		return
	}
	for range s.noSubscribersChan {
		if !s.hasSubscribers() {
			return
		}
	}
}
