// !build linux

package storage

import (
	"strings"
	"testing"

	"io/ioutil"

	"github.com/stretchr/testify/require"
)

func TestMmap(t *testing.T) {

	t.Run("it should be able to write content", func(t *testing.T) {
		content := []byte(strings.Repeat("-", 30000))

		s, err := NewMmapStorage("", 30000)
		require.NoError(t, err)
		defer s.Clean()

		r, _ := s.GetReader()
		defer r.Close()

		go func() {
			s.Write(content)
			s.Flush()
			s.Close()
		}()

		read, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, content, read)
	})

	t.Run("it should ignore if content larger than content-length", func(t *testing.T) {
		content := []byte("abcdef")

		s, err := NewMmapStorage("", 2)
		defer s.Clean()

		require.NoError(t, err)

		s.Write(content)
		s.Close()

		r, _ := s.GetReader()
		defer r.Close()
		read, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, []byte("ab"), read)
	})

	t.Run("it should not return garbage if content-lenght is larger than response", func(t *testing.T) {
		content := []byte("abcdef")

		s, err := NewMmapStorage("", 10)
		defer s.Clean()

		require.NoError(t, err)

		s.Write(content)
		s.Close()

		r, _ := s.GetReader()
		defer r.Close()
		read, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, content, read)
	})
}
