package cache

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"os"
)

/* Storage */
type Storage interface {
	Setup() error
	NewContent(string) (StorageContent, error)
}

type StorageContent interface {
	io.WriteCloser
	Bytes() []byte
	Clear() error
}

/* Memory Storage */

type MemoryStorage struct{}

type MemoryData struct {
	content *bytes.Buffer
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

func (s *MemoryStorage) Setup() error {
	return nil
}

func (s *MemoryStorage) NewContent(key string) (StorageContent, error) {
	return &MemoryData{content: new(bytes.Buffer)}, nil
}

func (buff *MemoryData) Write(p []byte) (int, error) {
	return buff.content.Write(p)
}

func (buff *MemoryData) Bytes() []byte {
	return buff.content.Bytes()
}

func (buff *MemoryData) Close() error {
	// TODO should this return an error if it was already closed?
	return nil
}

func (buff *MemoryData) Clear() error {
	return nil
}

/*
 *
 * MMap Storage
 *
 */

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type MMapStorage struct {
	path string
}

type MMapContent struct {
	file    *os.File
	mapping []byte
}

func NewMMapStorage(path string) *MMapStorage {
	return &MMapStorage{path: path}
}

func (s *MMapStorage) Setup() error {
	return errors.New("MMap is not available on windows")
}

func (s *MMapStorage) NewContent(key string) (StorageContent, error) {
	return nil, errors.New("Not available")
}

func (data *MMapContent) Write(p []byte) (int, error) {
	return 0, errors.New("Not available")
}

func (data *MMapContent) Bytes() []byte {
	return nil
}

func (data *MMapContent) Close() error {
	return errors.New("Not available")
}

func (s *MMapContent) Clear() error {
	return errors.New("Not available")
}
