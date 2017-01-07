package storage

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type TestEntry struct {
	a string
	b int
}

func TestGetSet(t *testing.T) {
	m := MemoryStorage{}
	m.Setup()
	a := TestEntry{
		a: "One",
		b: 4,
	}
	m.Push("a", a, time.Now().Add(time.Duration(5)*time.Second))
	found, err := m.Get("a", func(a Value) bool {
		return true
	})

	assert.NoError(t, err, "Error while getting key")
	assert.Equal(t, a, found, "Could not found searched value")
}

func TestGetNonExistentKey(t *testing.T) {
	m := MemoryStorage{}
	m.Setup()
	found, err := m.Get("a", func(a Value) bool {
		return true
	})
	assert.NoError(t, err, "Error while getting key")
	assert.Nil(t, found, "Should not have found anything")
}

func TestPushManyValuesInSameKey(t *testing.T) {
	m := MemoryStorage{}
	m.Setup()
	valueOne := TestEntry{a: "One", b: 1}
	valueTwo := TestEntry{a: "Two", b: 2}

	m.Push("a", valueOne, time.Now().Add(time.Duration(5)*time.Second))
	m.Push("a", valueTwo, time.Now().Add(time.Duration(5)*time.Second))

	foundOne, err := m.Get("a", func(a Value) bool {
		return a.(TestEntry).a == "One"
	})

	assert.NoError(t, err, "Error while getting first value of a")
	assert.Equal(t, 1, foundOne.(TestEntry).b, "Could not found searched value")

	foundTwo, err := m.Get("a", func(a Value) bool {
		return a.(TestEntry).a == "Two"
	})

	assert.NoError(t, err, "Error while getting second value of a")
	assert.Equal(t, 2, foundTwo.(TestEntry).b, "Could not found searched value")
}

func TestPushManyValuesToDifferentKeys(t *testing.T) {
	m := MemoryStorage{}
	m.Setup()
	valueOne := TestEntry{a: "One", b: 1}
	valueTwo := TestEntry{a: "Two", b: 2}

	m.Push("a", valueOne, time.Now().Add(time.Duration(5)*time.Second))
	m.Push("b", valueTwo, time.Now().Add(time.Duration(5)*time.Second))

	foundOne, err := m.Get("a", func(a Value) bool {
		return a.(TestEntry).a == "One"
	})

	assert.NoError(t, err, "Error while getting first value of a")
	assert.Equal(t, 1, foundOne.(TestEntry).b, "Could not found searched value")

	foundTwo, err := m.Get("b", func(a Value) bool {
		return a.(TestEntry).a == "Two"
	})

	assert.NoError(t, err, "Error while getting second value of a")
	assert.Equal(t, 2, foundTwo.(TestEntry).b, "Could not found searched value")

	notFound, err := m.Get("a", func(a Value) bool {
		return a.(TestEntry).a == "Two"
	})

	assert.NoError(t, err, "Got erro while getting non existent key")
	assert.Nil(t, notFound, "Should not have found value")
}

func TestExpire(t *testing.T) {
	m := MemoryStorage{}
	m.Setup()
	valueOne := TestEntry{a: "One", b: 1}
	valueTwo := TestEntry{a: "Two", b: 2}
	valueThree := TestEntry{a: "Three", b: 3}

	m.Push("a", valueOne, time.Now().Add(time.Duration(1)*time.Second))
	m.Push("b", valueTwo, time.Now().Add(time.Duration(2)*time.Second))
	m.Push("b", valueThree, time.Now().Add(time.Duration(3)*time.Second))

	assertExpiration := func(key string, aValue string, shouldExist bool) {
		found, err := m.Get(key, func(a Value) bool {
			return a.(TestEntry).a == aValue
		})
		assert.NoError(t, err, "Got error while searching value")
		if shouldExist {
			assert.NotNil(t, found, "Searched key should not be nil, but it was")
		} else {
			assert.Nil(t, found, "Searched key should be nil but was not")
		}
	}

	assertExpiration("a", "One", true)
	assertExpiration("b", "Two", true)
	assertExpiration("b", "Three", true)

	time.Sleep(time.Duration(1100) * time.Millisecond)

	assertExpiration("a", "One", false)
	assertExpiration("b", "Two", true)
	assertExpiration("b", "Three", true)

	time.Sleep(time.Duration(1100) * time.Millisecond)

	assertExpiration("a", "One", false)
	assertExpiration("b", "Two", false)
	assertExpiration("b", "Three", true)

	time.Sleep(time.Duration(1100) * time.Millisecond)

	assertExpiration("a", "One", false)
	assertExpiration("b", "Two", false)
	assertExpiration("b", "Three", false)
}
