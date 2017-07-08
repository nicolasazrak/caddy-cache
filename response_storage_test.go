package cache

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileStorage(t *testing.T) {
	t.Run("should be abile to read after write", func(t *testing.T) {
		s, err := NewFileStorage()
		require.NoError(t, err)

		reader, _ := s.GetReader()

		content := []byte("abcdef")
		s.Write(content)

		buf := make([]byte, 32*1024)
		reader.Read(buf)

		require.Equal(t, content, buf[:len(content)])
	})

	t.Run("should create a file", func(t *testing.T) {
		s, err := NewFileStorage()
		require.NoError(t, err)

		_, err = os.Stat(s.(*FileStorage).file.Name())
		require.NoError(t, err)

		s.Close()
	})

	t.Run("should delete a file when is cleaned", func(t *testing.T) {
		s, err := NewFileStorage()
		require.NoError(t, err)

		s.Close()
		s.Clean()

		_, err = os.Stat(s.(*FileStorage).file.Name())
		if !os.IsNotExist(err) {
			t.Fail()
		}
	})
}

func TestStorageReader(t *testing.T) {
	t.Run("should ignore EOF until subscription is closed", func(t *testing.T) {
		buf := new(bytes.Buffer)
		content := new(bytes.Buffer)

		subscription := make(chan struct{})
		ended := make(chan struct{})
		reader := &StorageReader{content: ioutil.NopCloser(content), subscription: subscription}

		content.Write([]byte("123"))
		go func() {
			io.Copy(buf, reader)
			ended <- struct{}{}
		}()

		time.Sleep(time.Duration(1) * time.Millisecond)
		content.Write([]byte("456"))
		close(subscription)
		<-ended
		require.Equal(t, []byte("123456"), buf.Bytes())
	})
	t.Run("should call unsubscribe when close is called", func(t *testing.T) {
		closed := false
		reader := &StorageReader{content: ioutil.NopCloser(new(bytes.Buffer)), unsubscribe: func(a <-chan struct{}) {
			closed = true
		}}
		reader.Close()
		require.Equal(t, true, closed)
	})
}

func TestSubscription(t *testing.T) {
	t.Run("should notify every subscriber", func(t *testing.T) {
		s := NewSubscription()

		s1 := s.NewSubscriber()
		s2 := s.NewSubscriber()

		notified := make(chan struct{})

		go func() {
			s.NotifyAll()
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
	})

	t.Run("should not notify a subscriber if it was unsubscribed", func(t *testing.T) {
		s := NewSubscription()

		s1 := s.NewSubscriber()

		s.RemoveSubscriber(s1)
		s.NotifyAll()

		require.Len(t, s1, 0)
	})

	t.Run("should wait until all subscribers unsubscribe to continue", func(t *testing.T) {
		s := NewSubscription()

		s1 := s.NewSubscriber()
		s2 := s.NewSubscriber()

		s.NotifyAll()

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
