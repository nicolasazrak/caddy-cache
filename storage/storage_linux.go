// +build linux

package storage

func Build(path string, size int64) (ResponseStorage, error) {
	if size > 0 && size == int64(int(size)) {
		return NewMmapStorage(path, int(size))
	}
	return NewFileStorage(path)
}
