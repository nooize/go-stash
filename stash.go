package stash

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Stash struct {
	*stash
	// If this is confusing, see the comment at the bottom of New()
}

type item struct {
	Object interface{}
	expire int64
}

type stash struct {
	expirePeriod time.Duration
	gcPeriod     time.Duration
	lenLimit     int
	lenCurrent   int
	items        map[string]*item
	events       *eventHandler
	mu           sync.RWMutex
	gc           *gc
}

// Set Add an item to the stash, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the stash's default expiration time is used.
// If it is -1 (NoExpire), the item never expires.
func (c *stash) Set(key string, i ...interface{}) error {
	if len(i) == 0 || i[0] == nil {
		return errors.New("object not provided")
	}
	if c.lenLimit > 0 && c.lenCurrent >= c.lenLimit {
		return errors.New("len limit reached")
	}
	key = strings.Clone(key)
	expire, err := c.resolveTime(i...)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if v, ok := c.items[key]; ok {
		v.Object = i[0]
		v.expire = expire
		go c.events.fire(EventUpdate, key, v.Object)
	} else {
		c.items[key] = &item{
			Object: i[0],
			expire: expire,
		}
		c.lenCurrent = len(c.items)
		go c.events.fire(EventSet, key, i[0])
	}
	// Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.mu.Unlock()
	return nil
}

// Touch if item exist in stash stash, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the stash's default expiration time is used.
// If it is -1 (NoExpiration), the item never expires.
func (c *stash) Touch(key string, i ...interface{}) error {
	expire, err := c.resolveTime(i...)
	if err != nil {
		return err
	}
	c.mu.Lock()
	item, ok := c.items[key]
	if ok {
		item.expire = expire
		go c.events.fire(EventTouch, key, item.Object)
	}
	c.mu.Unlock()
	return nil
}

// Remove an item from the stash.
// Returns the item or nil,// and a bool indicating if the key was found and deleted.
func (c *stash) Remove(key string) (interface{}, bool) {
	return c.remove(key, EventRemove)
}

// Remove an item from the stash.
// Returns the item or nil,// and a bool indicating if the key was found and deleted.
func (c *stash) remove(key string, event Event) (interface{}, bool) {
	key = strings.Clone(key)
	c.mu.Lock()
	o, ok := c.items[key]
	if ok {
		delete(c.items, key)
		c.lenCurrent = len(c.items)
	}
	c.mu.Unlock()
	go c.events.fire(event, key, o.Object)
	return o, ok
}

// Get an item from the stash.
// Returns the item or nil,
// and a bool indicating if the key was found.
func (c *stash) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || (item.expire > 0 && item.expire < time.Now().UnixNano()) {
		go c.events.fire(EventMiss, key, nil)
		return nil, false
	}
	go c.events.fire(EventGet, key, item)
	return item.Object, true
}

func (c *stash) GetString(key string) (string, bool) {
	v, ok := c.Get(key)
	if !ok {
		return "", ok
	}
	switch v.(type) {
	case string:
		return v.(string), ok
	}
	return fmt.Sprintf("%v", v), ok
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
	go c.events.fire(EventClean, "", nil)
}

// Flush Delete expired items from the stash.
func (c *stash) Flush() {
	c.expire(EventFlush)
}

// Flush Delete expired items from the stash.
func (c *stash) expire(event Event) {
	expiredItems := make([]string, 0)
	curTime := time.Now().UnixNano()
	c.mu.RLock()
	for key, item := range c.items {
		if item.expire > 0 && item.expire < curTime {
			expiredItems = append(expiredItems, key)
		}
	}
	c.mu.RUnlock()
	go func(removed []string) {
		for _, key := range removed {
			c.remove(key, event)
		}
	}(expiredItems)
}

func (c *stash) resolveTime(i ...interface{}) (expire int64, err error) {
	if len(i) > 1 {
		expire = toTime(i[1])
	}
	if expire == int64(DefaultExpire) {
		expire = toTime(c.expirePeriod)
	}
	if expire > 0 && expire < time.Now().UnixNano() {
		err = errors.New("expire time is in the past")
	}
	return
}

func toTime(i interface{}) int64 {
	if i == nil {
		return int64(NoExpire)
	}
	switch i.(type) {
	case int, int8, int16, int32, int64:
		return toTime(time.Second * time.Duration(i.(int64)))
	case uint, uint8, uint16, uint32, uint64:
		return toTime(time.Second * time.Duration(i.(uint64)))
	case time.Duration:
		dur := i.(time.Duration)
		switch dur {
		case DefaultExpire:
			return int64(DefaultExpire)
		case NoExpire:
			return int64(NoExpire)
		}
		return toTime(time.Now().Add(dur))
	case time.Time:
		return i.(time.Time).UnixNano()
	case *time.Time:
		if t := i.(*time.Time); t != nil {
			return i.(*time.Time).UnixNano()
		}
	}
	return int64(DefaultExpire)
}

type events struct {
	Interval time.Duration
	stop     chan bool
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
			c.expire(EventExpire)
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}
