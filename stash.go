package stash

import (
	"sync"
	"time"
)

type StashEvent int

func (e StashEvent) String() string {
	switch e {
	case OnGet:
		return "GET"
	case OnSet:
		return "SET"
	case OnUpdate:
		return "UPDATE"
	case OnTouch:
		return "TOUCH"
	case OnRemove:
		return "REMOVE"
	case OnExpire:
		return "EXPIRE"
	case OnFlush:
		return "FLUSH"
	}
	return "[UNDEFINED]"
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
	lenLimit     int
	lenCurrent   int
	items        map[string]*item
	mu           sync.RWMutex
	onEvent      OnEventHandler
	gc           *gc
}

// Set Add an item to the stash, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the stash's default expiration time is used.
// If it is -1 (NoExpire), the item never expires.
func (c *stash) Set(k string, i ...interface{}) bool {
	if len(i) == 0 || i[0] == nil {
		return false
	}
	if c.lenLimit > 0 && c.lenCurrent >= c.lenLimit {
		return false
	}
	var expire *time.Time
	if len(i) > 1 {
		expire = c.toTime(i[1])
	} else {
		expire = c.toTime(c.expirePeriod)
	}
	if expire != nil && expire.Before(time.Now()) {
		return false
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
		c.lenCurrent = len(c.items)
	}
	// Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.mu.Unlock()
	return true
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
	return c.remove(key, OnRemove)
}

// Remove an item from the stash.
// Returns the item or nil,// and a bool indicating if the key was found and deleted.
func (c *stash) remove(key string, event StashEvent) (interface{}, bool) {
	c.mu.Lock()
	o, ok := c.items[key]
	if ok {
		delete(c.items, key)
		c.lenCurrent = len(c.items)
	}
	c.mu.Unlock()
	if ok && c.onEvent != nil {
		go c.onEvent(event, key, o.Object)
	}
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
	return c.lenCurrent
}

// Clean Delete all items from the stash.
func (c *stash) Clean() {
	c.mu.Lock()
	c.items = make(map[string]*item)
	c.lenCurrent = len(c.items)
	c.mu.Unlock()
	if c.onEvent != nil {
		go c.onEvent(OnClean, "", nil)
	}
}

// Flush Delete expired items from the stash.
func (c *stash) Flush() {
	c.expire(OnFlush)
}

// Flush Delete expired items from the stash.
func (c *stash) expire(event StashEvent) {
	expiredItems := make(map[string]interface{})
	c.mu.RLock()
	for key, item := range c.items {
		if item.Expired() {
			expiredItems[key] = item.Object
		}
	}
	c.mu.RUnlock()
	go func(removed map[string]interface{}) {
		for key, _ := range removed {
			c.remove(key, event)
		}
	}(expiredItems)
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
			c.expire(OnExpire)
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}
