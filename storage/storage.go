// +build !linux

package storage

func Build(path string, size int64) (ResponseStorage, error) {
	return NewFileStorage(path)
}
