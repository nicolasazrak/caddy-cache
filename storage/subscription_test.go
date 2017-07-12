package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscription(t *testing.T) {
	t.Run("should notify every subscriber", func(t *testing.T) {
		s := NewSubscription()

		s1 := s.NewSubscriber()
		s2 := s.NewSubscriber()

		notified := make(chan struct{})

		go func() {
			s.NotifyAll(6)
			notified <- struct{}{}
		}()

		<-notified
		require.Len(t, s1, 1)
		require.Len(t, s2, 1)
	})

	t.Run("should return a closed subscription if is closed", func(t *testing.T) {
		s := NewSubscription()
		s.Close()

		s1 := s.NewSubscriber()

		for range s1 { // If s1 is not closed it will hang in here
			t.FailNow()
		}

		s.Close() // Do it again to check it does no tries to close the subscribers again
	})

	t.Run("should not notify a subscriber if it was unsubscribed", func(t *testing.T) {
		s := NewSubscription()

		s1 := s.NewSubscriber()

		s.RemoveSubscriber(s1)
		s.NotifyAll(10)

		require.Len(t, s1, 0)
	})

	t.Run("should wait until all subscribers unsubscribe to continue", func(t *testing.T) {
		s := NewSubscription()

		s1 := s.NewSubscriber()
		s2 := s.NewSubscriber()

		s.NotifyAll(9)

		waitCalled := make(chan struct{}, 1)
		ended := make(chan struct{}, 1)

		go func() {
			waitCalled <- struct{}{}
			s.WaitAll()
			ended <- struct{}{}
		}()

		require.Len(t, ended, 0)
		<-waitCalled
		s.RemoveSubscriber(s1)
		require.Len(t, ended, 0)
		s.RemoveSubscriber(s2)
		<-ended
	})
}
