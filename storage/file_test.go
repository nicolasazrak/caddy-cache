package storage

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
		s, err := NewFileStorage("")
		require.NoError(t, err)
		defer s.Clean()

		reader, _ := s.GetReader()
		defer reader.Close()

		content := []byte("abcdef")
		s.Write(content)

		buf := make([]byte, 32*1024)
		reader.Read(buf)

		require.Equal(t, content, buf[:len(content)])
	})

	t.Run("should create a file", func(t *testing.T) {
		s, err := NewFileStorage("")
		require.NoError(t, err)
		defer s.Clean()

		_, err = os.Stat(s.(*FileStorage).file.Name())
		require.NoError(t, err)

		s.Close()
	})

	t.Run("should delete a file when is cleaned", func(t *testing.T) {
		s, err := NewFileStorage("")
		require.NoError(t, err)

		s.Close()
		s.Clean()

		_, err = os.Stat(s.(*FileStorage).file.Name())
		if !os.IsNotExist(err) {
			t.Fail()
		}
	})
}

func TestFileReader(t *testing.T) {
	t.Run("should ignore EOF until subscription is closed", func(t *testing.T) {
		buf := new(bytes.Buffer)
		content := new(bytes.Buffer)

		subscription := make(chan int)
		ended := make(chan struct{})
		reader := &FileReader{content: ioutil.NopCloser(content), subscription: subscription}

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
		reader := &FileReader{content: ioutil.NopCloser(new(bytes.Buffer)), unsubscribe: func(a <-chan int) {
			closed = true
		}}
		reader.Close()
		require.Equal(t, true, closed)
	})
}
