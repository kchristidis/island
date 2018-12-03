package cmap

import (
	"sync"
)

// Container is a wrapper around Go's built-in map type that will only carry up
// to cap key-value pairs. An attempt to add a key/value pair to a cmap
// container that is already at capacity will succeed but will evict an existing
// key-value pair from the Container according to the LRU (least-recently used)
// policy.
type Container struct {
	len, cap int

	vals               map[interface{}]*node
	fakeHead, fakeTail *node
	rwm                *sync.RWMutex
}

// New returns a cmap container that can carry up to cap key/value pairs. The
// parameter cap should be a positive integer.
func New(cap int) (*Container, error) {
	if cap < 1 {
		return nil, ErrInvalidCapacity
	}

	fakeHead, fakeTail := new(node), new(node)
	fakeHead.next, fakeTail.prev = fakeTail, fakeHead

	cm := &Container{
		cap:      cap,
		vals:     make(map[interface{}]*node),
		fakeHead: fakeHead,
		fakeTail: fakeTail,
		rwm:      new(sync.RWMutex),
	}

	return cm, nil
}

// Get returns the value that corresponds to the given key, if it exists.
func (cm *Container) Get(key interface{}) (interface{}, bool) {
	cm.rwm.RLock()
	defer cm.rwm.RUnlock()

	v, ok := cm.vals[key]
	if !ok {
		return nil, ok
	}
	moveToTail(cm.fakeTail, v)

	return v.val, ok
}

// Put adds the given key/value pair to the cmap container.
func (cm *Container) Put(key, val interface{}) {
	newNode := &node{
		key: key,
		val: val,
	}

	cm.rwm.Lock()
	defer cm.rwm.Unlock()

	oldNode, ok := cm.vals[key]

	// Identify the cases where we need to remove elements first.
	if ok {
		delete(cm.vals, oldNode.key)
		removeNode(oldNode)
	} else {
		if cm.len == cm.cap {
			excessNode := cm.fakeHead.next
			delete(cm.vals, excessNode.key)
			removeNode(excessNode)
			cm.len--
		}
	}

	// The key does NOT exist in the map and we're below capacity.
	cm.vals[key] = newNode
	addToTail(cm.fakeTail, newNode)
	cm.len++

	return
}

// Delete removes the given key from the cmap container, if it exists.
func (cm *Container) Delete(key interface{}) bool {
	cm.rwm.Lock()
	defer cm.rwm.Unlock()

	v, ok := cm.vals[key]
	if !ok {
		return false
	}

	delete(cm.vals, v.key)
	removeNode(v)
	cm.len--
	return true
}
