package cache

import "testing"
import "github.com/stretchr/testify/require"
import "net/http/httptest"
import "io/ioutil"
import "io"

type TestStorage struct {
	recorder *httptest.ResponseRecorder
	closed   bool
	flushed  bool
	cleaned  bool
}

func NewTestStorage() *TestStorage {
	return &TestStorage{
		recorder: httptest.NewRecorder(),
	}
}

func (ts *TestStorage) Write(p []byte) (int, error) {
	return ts.recorder.Write(p)
}

func (ts *TestStorage) Close() error {
	ts.closed = true
	return nil
}

func (ts *TestStorage) Flush() error {
	ts.flushed = true
	return nil
}

func (ts *TestStorage) Clean() error {
	ts.cleaned = true
	return nil
}

func (ts *TestStorage) GetReader() (io.ReadCloser, error) {
	return ts.recorder.Result().Body, nil
}

func (ts *TestStorage) ReadAll() []byte {
	r, _ := ts.GetReader()
	c, _ := ioutil.ReadAll(r)
	return c
}

////////////////////////////

func TestResponseSendHeaders(t *testing.T) {
	r := NewResponse()

	go func() {
		r.Header().Add("Content-Type", "application/json")
		r.WriteHeader(200)
	}()

	r.WaitHeaders()
	require.Equal(t, r.Header().Get("Content-Type"), "application/json")
}

func TestResponseWaitStorage(t *testing.T) {
	r := NewResponse()
	routineStarted := make(chan struct{}, 1)
	writtenChan := make(chan struct{}, 1)
	originalContent := []byte("abc")

	go func() {
		routineStarted <- struct{}{}
		r.Write(originalContent)
		writtenChan <- struct{}{}
	}()

	r.WaitHeaders()
	<-routineStarted
	require.Len(t, writtenChan, 0)
	storage := NewTestStorage()
	r.SetBody(storage)
	<-writtenChan

	require.Equal(t, originalContent, storage.ReadAll())
}

func TestCloseResponse(t *testing.T) {
	r := NewResponse()

	go func() {
		r.WriteHeader(200)
	}()

	r.WaitHeaders()
	storage := NewTestStorage()
	r.SetBody(storage)
	r.Close()

	require.True(t, storage.closed)
}

func TestResponseClean(t *testing.T) {
	r := NewResponse()

	r.Close()
	storage := NewTestStorage()
	r.SetBody(storage)
	r.Clean()

	require.True(t, storage.cleaned)
}
