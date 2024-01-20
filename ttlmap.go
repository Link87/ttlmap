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

import (
	"maps"
	"sync"
	"time"
)

const version string = "0.1.0"

type Key interface {
	comparable
}

type item[V any] struct {
	value      V
	lastAccess int64
}

type TtlMap[K Key, V any] struct {
	entries map[K]*item[V]
	lock    sync.RWMutex
	stop    chan bool
}

func New[K Key, V any](size uint, ttl time.Duration, pruneInterval time.Duration) (m *TtlMap[K, V]) {
	// if pruneInterval > ttl {
	// 	print("WARNING: TtlMap: pruneInterval > ttl\n")
	// }
	m = &TtlMap[K, V]{
		entries: make(map[K]*item[V], size),
		stop:    make(chan bool),
	}
	ttl /= 1_000_000_000
	// print("ttl: ", ttl, "\n")
	go func() {
		for {
			select {
			case <-m.stop:
				return
			case now := <-time.Tick(pruneInterval):
				currentTime := now.Unix()
				m.lock.Lock()
				for k, v := range m.entries {
					// print("TICK:", currentTime, "  ", v.lastAccess, "  ", currentTime-v.lastAccess, "  ", ttl, "  ", k, "\n")
					if currentTime-v.lastAccess >= int64(ttl) {
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
		value:      value,
		lastAccess: time.Now().Unix(),
	}
}

func (m *TtlMap[K, V]) Get(key K) (value V) {
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
		m.lock.Unlock()
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
	m.stop <- true
	m.Clear()
}
