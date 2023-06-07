package stash

import (
	"sync"
	"time"
)

type Config struct {
	Expire   time.Duration
	GcPeriod time.Duration
}

type Stash struct {
	*stash
	// If this is confusing, see the comment at the bottom of New()
}

type item struct {
	Object interface{}
	expire *time.Time
}

func (item item) Expired() bool {
	if item.expire == nil {
		return false
	}
	return item.expire.Before(time.Now())
}

type stash struct {
	expirePeriod time.Duration
	gcPeriod     time.Duration
	items        map[string]*item
	mu           sync.RWMutex
	onExpire     OnExpireHandler
	gc           *gc
}

// Set Add an item to the stash, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the stash's default expiration time is used.
// If it is -1 (NoExpire), the item never expires.
func (c *stash) Set(k string, i ...interface{}) {
	if len(i) == 0 || i[0] == nil {
		return
	}
	var expire *time.Time
	if len(i) > 1 {
		expire = c.toTime(i[2])
	} else {
		expire = c.toTime(c.expirePeriod)
	}
	if expire != nil && expire.Before(time.Now()) {
		return
	}
	c.mu.Lock()
	if v, ok := c.items[k]; ok {
		v.Object = i[0]
		v.expire = expire
	} else {
		c.items[k] = &item{
			Object: i[0],
			expire: expire,
		}
	}
	// Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.mu.Unlock()
}

// Touch if item exist in stash stash, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the stash's default expiration time is used.
// If it is -1 (NoExpiration), the item never expires.
func (c *stash) Touch(key string, i ...interface{}) (touched bool) {
	var expire *time.Time
	if len(i) > 0 {
		expire = c.toTime(i[1])
	} else {
		expire = c.toTime(c.expirePeriod)
	}
	if expire != nil && expire.Before(time.Now()) {
		return false
	}
	c.mu.Lock()
	if item, ok := c.items[key]; ok {
		item.expire = expire
		touched = true
	}
	c.mu.Unlock()
	return
}

// Remove an item from the stash.
// Returns the item or nil,// and a bool indicating if the key was found and deleted.
func (c *stash) Remove(key string) (interface{}, bool) {
	c.mu.Lock()
	o, ok := c.items[key]
	if ok {
		delete(c.items, key)
	}
	c.mu.Unlock()
	return o, ok
}

// Get an item from the stash.
// Returns the item or nil,
// and a bool indicating if the key was found.
func (c *stash) Get(k string) (interface{}, bool) {
	c.mu.RLock()
	item, ok := c.items[k]
	c.mu.RUnlock()
	if !ok || item.Expired() {
		return nil, false
	}
	return item.Object, true
}

// Len Returns the number of items in the stash. This may include items that have
// expired, but have not yet been cleaned up.
func (c *stash) Len() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Clean Delete all items from the stash.
func (c *stash) Clean() {
	c.mu.Lock()
	c.items = make(map[string]*item)
	c.mu.Unlock()
}

// Flush Delete expired items from the stash.
func (c *stash) Flush() {
	expiredItems := make(map[string]interface{})
	c.mu.RLock()
	for key, item := range c.items {
		if item.Expired() {
			expiredItems[key] = item.Object
		}
	}
	c.mu.RUnlock()
	go func(removed map[string]interface{}) {
		for key, obj := range removed {
			c.Remove(key)
			if c.onExpire != nil {
				go c.onExpire(key, obj)
			}
		}
	}(expiredItems)
}

// OnExpire Sets an (optional) function that is called with the key and value when an
// item is evicted from the stash by expire.
func (c *stash) OnExpire(f func(string, interface{})) {
	c.mu.Lock()
	c.onExpire = f
	c.mu.Unlock()
}

func (c *stash) toTime(i interface{}) *time.Time {
	if i == nil {
		return nil
	}
	switch i.(type) {
	case int, uint, int8, uint8, int16, uint16, int32, uint32, int64, uint64:
		return c.toTime(time.Second * time.Duration(i.(int64)))
	case time.Duration:
		dur := i.(time.Duration)
		switch dur {
		case DefaultExpire:
			dur = c.expirePeriod
		case NoExpire:
			return nil
		}
		return c.toTime(time.Now().Add(dur))
	case time.Time:
		t := i.(time.Time)
		return &t
	case *time.Time:
		return i.(*time.Time)
	}
	return c.toTime(c.expirePeriod)
}

type gc struct {
	Interval time.Duration
	stop     chan bool
}

func (j *gc) Run(c *stash) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.Flush()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}
