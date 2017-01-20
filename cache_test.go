package cache

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
	"time"
)

/* Helpers */

func alwaysTrue(a *HttpCacheEntry) bool {
	return true
}

func alwaysFalse(a *HttpCacheEntry) bool {
	return true
}

func push(m *Cache, key string, value *HttpCacheEntry) {
	m.GetOrSet(key, alwaysFalse, func(entry *HttpCacheEntry) (*HttpCacheEntry, error) {
		return value, nil
	})
}

// TODO create a get helper

/* Actual tests */

func TestGetSet(t *testing.T) {
	m := NewCache(NewMemoryStorage())
	m.Setup()
	a := &HttpCacheEntry{
		Response:   nil,
		Expiration: time.Now().Add(time.Duration(5) * time.Second),
	}

	push(m, "a", a)
	err := m.GetOrSet("a", alwaysTrue, func(found *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.Equal(t, a, found, "Could not found searched value")
		return nil, nil
	})

	assert.NoError(t, err, "Should not have been an error")
}

func TestGetNonExistentKey(t *testing.T) {
	m := NewCache(NewMemoryStorage())
	m.Setup()
	a := &HttpCacheEntry{
		Response:   nil,
		Expiration: time.Now().Add(time.Duration(5) * time.Second),
	}

	push(m, "a", a)
	err := m.GetOrSet("b", alwaysTrue, func(found *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.Nil(t, found, "Should not have found element")
		return nil, nil
	})

	assert.NoError(t, err, "Should not have been an error")
}

func TestPushManyValuesInSameKey(t *testing.T) {
	m := NewCache(NewMemoryStorage())
	m.Setup()
	inFiveSeconds := time.Now().UTC().Add(time.Duration(5) * time.Second)
	inTwoSeconds := time.Now().UTC().Add(time.Duration(2) * time.Second)

	a := &HttpCacheEntry{Response: nil, Expiration: inFiveSeconds}
	b := &HttpCacheEntry{Response: &Response{}, Expiration: inTwoSeconds}

	push(m, "a", a)
	push(m, "a", b)

	err := m.GetOrSet("a", func(a *HttpCacheEntry) bool { return a.Response == nil }, func(found *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.Equal(t, a, found, "Got another value")
		return nil, nil
	})

	assert.NoError(t, err, "Error while getting first value of a")

	err = m.GetOrSet("a", func(b *HttpCacheEntry) bool { return b.Response != nil }, func(found *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.Equal(t, b, found, "Got another value")
		return nil, nil
	})
	assert.NoError(t, err, "Error while getting second value of a")
}

func TestPushManyValuesToDifferentKeys(t *testing.T) {
	m := NewCache(NewMemoryStorage())
	m.Setup()
	inFiveSeconds := time.Now().UTC().Add(time.Duration(5) * time.Second)
	inTwoSeconds := time.Now().UTC().Add(time.Duration(2) * time.Second)

	a := &HttpCacheEntry{Response: nil, Expiration: inFiveSeconds}
	b := &HttpCacheEntry{Response: &Response{}, Expiration: inTwoSeconds}

	push(m, "a", a)
	push(m, "b", b)

	err := m.GetOrSet("a", alwaysTrue, func(found *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.Equal(t, a, found, "Got another value")
		return nil, nil
	})
	assert.NoError(t, err, "Error while getting first value of a")

	err = m.GetOrSet("b", alwaysTrue, func(found *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.Equal(t, b, found, "Got another value")
		return nil, nil
	})
	assert.NoError(t, err, "Error while getting second value of a")
}

func TestExpire(t *testing.T) {
	m := NewCache(NewMemoryStorage())
	m.Setup()
	in10Milliseconds := time.Now().UTC().Add(time.Duration(10) * time.Millisecond)
	in40Milliseconds := time.Now().UTC().Add(time.Duration(40) * time.Millisecond)
	in80Milliseconds := time.Now().UTC().Add(time.Duration(80) * time.Millisecond)

	a := &HttpCacheEntry{Response: nil, Expiration: in10Milliseconds}
	b := &HttpCacheEntry{Response: &Response{}, Expiration: in40Milliseconds}
	c := &HttpCacheEntry{Response: &Response{}, Expiration: in80Milliseconds}

	push(m, "a", a)
	push(m, "a", b)
	push(m, "b", c)

	assertExpiration := func(key string, responseIsNil bool, shouldExist bool) {
		m.GetOrSet(key, func(value *HttpCacheEntry) bool { return (value.Response == nil) == responseIsNil }, func(found *HttpCacheEntry) (*HttpCacheEntry, error) {
			if shouldExist {
				assert.NotNil(t, found, "An entry that should exist was expired")
			} else {
				assert.Nil(t, found, "An entry that should be expired was not")
			}
			return nil, nil
		})
	}

	assertExpiration("a", true, true)
	assertExpiration("a", false, true)
	assertExpiration("b", false, true)

	time.Sleep(time.Duration(20) * time.Millisecond)

	assertExpiration("a", true, false)
	assertExpiration("a", false, true)
	assertExpiration("b", false, true)

	time.Sleep(time.Duration(40) * time.Millisecond)

	assertExpiration("a", true, false)
	assertExpiration("a", false, false)
	assertExpiration("b", false, true)

	time.Sleep(time.Duration(40) * time.Millisecond)

	assertExpiration("a", true, false)
	assertExpiration("a", false, false)
	assertExpiration("b", false, false)
}

func TestMMapExpire(t *testing.T) {
	m := NewCache(NewMMapStorage("/tmp/caddy-cache-tests"))
	m.Setup()
	in10Milliseconds := time.Now().UTC().Add(time.Duration(10) * time.Millisecond)
	key := "a"

	content := []byte("Hello")
	mmap, err := m.NewContent(key)
	assert.NoError(t, err, "Failed creating new content")
	mmap.Write(content)
	mmap.Close()

	a := &HttpCacheEntry{Response: &Response{Body: mmap}, Expiration: in10Milliseconds}
	push(m, key, a)
	filename := ""
	err = m.GetOrSet(key, alwaysTrue, func(entry *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.NotNil(t, entry, "Entry was not found")
		filename = entry.Response.Body.(*MMapContent).file.Name()
		return nil, nil
	})
	assert.NoError(t, err, "There was an error in get")
	readContent, err := ioutil.ReadFile(filename)
	assert.NoError(t, err, "Error reading file")
	assert.Equal(t, content, readContent, "Saved content does not match")

	// Lock the content for 20 milliseconds
	go m.GetOrSet(key, alwaysTrue, func(entry *HttpCacheEntry) (*HttpCacheEntry, error) {
		time.Sleep(time.Duration(20) * time.Millisecond)
		return nil, nil
	})

	// After 10 ms entry should be deleted but content should still be available
	time.Sleep(time.Duration(15) * time.Millisecond)
	m.GetOrSet(key, alwaysTrue, func(entry *HttpCacheEntry) (*HttpCacheEntry, error) {
		assert.Nil(t, entry, "Content should have expired")
		return nil, nil
	})

	// Read the content again
	_, err = ioutil.ReadFile(filename)
	assert.NoError(t, err, "Error reading file")

	// Wait 10 ms and the content should be deleted
	time.Sleep(time.Duration(10) * time.Millisecond)
	_, err = ioutil.ReadFile(filename)
	assert.Error(t, err, "File still exists")
}
