/*
TtlMap.go
-John Taylor/Marvin Gazibarić
2024-01-20

TtlMap is a "time-to-live" map such that after a given amount of time, items
in the map are deleted.

When a Put() occurs, the lastAccess time is set to time.Now().Unix()
When a Get() occurs, the lastAccess time is updated to time.Now().Unix()
Therefore, only items that are not called by Get() will be deleted after the TTL occurs.
A GetNoUpdate() can be used in which case the lastAccess time will NOT be updated.

Adopted from: https://stackoverflow.com/a/25487392/452281
*/

package ttlmap

// TODO make it so that multiple operations are possible without having to unlock and lock again
// TODO refresh TTLs of items: e.g. Touch() method, PutIfNew() method
// TODO entry API
// TODO TTLs per entry (second map type)
// TODO switch from UNIX timestamp to some base time created in New()

import (
	"maps"
	"sync"
	"time"
)

const version string = "0.1.1"

type Key interface {
	comparable
}

// item is an entry in a TtlMap.
type item[V any] struct {
	// value is the value of the item.
	value V
	// expires is the nanos UNIX timestamp when the item expires.
	expires int64
}

type TtlMap[K Key, V any] struct {
	// entries are the elements in this TtlMap.
	entries map[K]*item[V]
	// ttl is the time-to-live of each element. Saved as number of nanoseconds.
	ttl time.Duration
	// lock is the lock for synchronizing access to entries.
	lock sync.RWMutex
	// stop is the channel for stopping the prune goroutine.
	stop chan<- struct{}
}

func New[K Key, V any](capacity uint, ttl time.Duration, pruneInterval time.Duration) (m *TtlMap[K, V]) {

	stop := make(chan struct{})
	m = &TtlMap[K, V]{
		entries: make(map[K]*item[V], capacity),
		ttl:     ttl,
		stop:    stop,
	}

	go func() {
		ticker := time.NewTicker(pruneInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				currentTime := now.UnixNano()
				m.lock.Lock()
				for k, v := range m.entries {
					// print("TICK:", currentTime, "  ", v.lastAccess, "  ", currentTime-v.lastAccess, "  ", ttl, "  ", k, "\n")
					if currentTime >= v.expires {
						delete(m.entries, k)
						// print("deleting: ", k, "\n")
					}
				}
				// print("----\n")
				m.lock.Unlock()
			}
		}
	}()
	return
}

func (m *TtlMap[K, V]) Len() (size uint) {
	m.lock.RLock()
	size = uint(len(m.entries))
	defer m.lock.RUnlock()
	return
}

func (m *TtlMap[K, V]) Put(key K, value V) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.entries[key] = &item[V]{
		value:   value,
		expires: time.Now().Add(m.ttl).UnixNano(),
	}
}

func (m *TtlMap[K, V]) Get(key K) (value V, ok bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	var item *item[V]
	if item, ok = m.entries[key]; ok {
		value = item.value
	}
	return
}

func (m *TtlMap[K, V]) GetOrZero(key K) (value V) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if it, ok := m.entries[key]; ok {
		value = it.value
	}
	return
}

func (m *TtlMap[K, V]) Delete(key K) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok := m.entries[key]
	if !ok {
		return false
	}
	delete(m.entries, key)
	return true
}

func (m *TtlMap[K, V]) Clear() {
	m.lock.Lock()
	defer m.lock.Unlock()
	clear(m.entries)
}

func (m *TtlMap[K, V]) Copy() map[K]*item[V] {
	m.lock.Lock()
	defer m.lock.Unlock()
	dst := make(map[K]*item[V], len(m.entries))
	maps.Copy(dst, m.entries)
	return dst
}

func (m *TtlMap[K, V]) Close() {
	m.stop <- struct{}{}
	m.Clear()
}
