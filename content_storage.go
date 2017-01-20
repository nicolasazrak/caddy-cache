package cache

import (
	"bytes"
	"encoding/base32"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"syscall"
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
	return os.MkdirAll(s.path, 0700)
}

func (s *MMapStorage) NewContent(key string) (StorageContent, error) {
	filename := path.Join(s.path, base32.StdEncoding.EncodeToString([]byte(key))+randSeq(10))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return &MMapContent{file: file}, nil
}

func (data *MMapContent) Write(p []byte) (int, error) {
	return data.file.Write(p)
}

func (data *MMapContent) Bytes() []byte {
	return data.mapping
}

func (data *MMapContent) Close() error {
	if err := data.file.Sync(); err != nil {
		return err
	}

	info, err := data.file.Stat()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	fd := int(data.file.Fd())
	flags := syscall.PROT_READ | syscall.PROT_WRITE
	mapping, err := syscall.Mmap(fd, 0, int(info.Size()), flags, syscall.MAP_SHARED)
	if err != nil {
		return err
	}
	data.mapping = mapping
	return nil
}

func (s *MMapContent) Clear() error {
	err := syscall.Munmap(s.mapping)
	if err != nil {
		return err
	}
	filePath := s.file.Name()
	err = s.file.Close()
	if err != nil {
		return err
	}
	return os.Remove(filePath)
}
