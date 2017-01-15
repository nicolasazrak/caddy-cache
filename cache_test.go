package cache

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func alwaysTrue(a *HttpCacheEntry) bool {
	return true
}

func TestGetSet(t *testing.T) {
	m := NewCache(NewMemoryStorage())
	m.Setup()
	a := &HttpCacheEntry{
		Response:   nil,
		Expiration: time.Now().Add(time.Duration(5) * time.Second),
	}

	m.Push("a", a)
	err := m.GetOrLock("a", alwaysTrue, func(found *HttpCacheEntry) error {
		assert.Equal(t, a, found, "Could not found searched value")
		return nil
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

	m.Push("a", a)
	err := m.GetOrLock("b", alwaysTrue, func(found *HttpCacheEntry) error {
		assert.Nil(t, found, "Should not have found element")
		return nil
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

	m.Push("a", a)
	m.Push("a", b)

	err := m.GetOrLock("a", func(a *HttpCacheEntry) bool { return a.Response == nil }, func(found *HttpCacheEntry) error {
		assert.Equal(t, a, found, "Got another value")
		return nil
	})

	assert.NoError(t, err, "Error while getting first value of a")

	err = m.GetOrLock("a", func(b *HttpCacheEntry) bool { return b.Response != nil }, func(found *HttpCacheEntry) error {
		assert.Equal(t, b, found, "Got another value")
		return nil
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

	m.Push("a", a)
	m.Push("b", b)

	err := m.GetOrLock("a", alwaysTrue, func(found *HttpCacheEntry) error {
		assert.Equal(t, a, found, "Got another value")
		return nil
	})
	assert.NoError(t, err, "Error while getting first value of a")

	err = m.GetOrLock("b", alwaysTrue, func(found *HttpCacheEntry) error {
		assert.Equal(t, b, found, "Got another value")
		return nil
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

	m.Push("a", a)
	m.Push("a", b)
	m.Push("b", c)

	assertExpiration := func(key string, responseIsNil bool, shouldExist bool) {
		m.GetOrLock(key, func(value *HttpCacheEntry) bool { return (value.Response == nil) == responseIsNil }, func(found *HttpCacheEntry) error {
			if shouldExist {
				assert.NotNil(t, found, "An entry that should exist was expired")
			} else {
				assert.Nil(t, found, "An entry that should be expired was not")
			}
			return nil
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
