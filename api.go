package stash

import (
	"runtime"
	"time"
)

const (
	// NoLenLimit for use with LenLimit func set unlimited stash length
	NoLenLimit int = 0

	// NoExpire - For use with functions that take an expiration duration.
	NoExpire time.Duration = -1

	// DefaultExpire For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// when the stash was created (e.g. 5 minutes.)
	DefaultExpire time.Duration = 0

	DefaultCgPeriod = 5 * time.Minute
)

func New(opts ...func(*stash)) *Stash {
	c := &stash{
		expirePeriod: DefaultExpire,
		gcPeriod:     DefaultCgPeriod,
		lenLimit:     NoLenLimit,
		items:        make(map[string]*item),
	}
	for _, opt := range opts {
		opt(c)
	}
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	C := &Stash{c}
	if c.gcPeriod > 0 {
		startGc(c)
		runtime.SetFinalizer(C, stopGc)
	}
	return C
}

func LenLimit(limit int) func(*stash) {
	return func(c *stash) {
		if limit >= NoLenLimit {
			c.lenLimit = limit
		}

	}
}

func ExpireAfter(dur time.Duration) func(*stash) {
	return func(c *stash) {
		switch {
		case dur < 0:
			dur = NoExpire
		case dur == 0:
		}
		c.expirePeriod = dur
	}
}

func GcPeriod(dur time.Duration) func(*stash) {
	return func(c *stash) {
		if dur < 10*time.Millisecond {
			dur = DefaultCgPeriod
		}
		c.gcPeriod = dur
	}
}

func CloneFrom(src *Stash) func(*stash) {
	return func(c *stash) {
		if src == nil {
			return
		}
		c.expirePeriod = src.expirePeriod
		c.gcPeriod = src.gcPeriod
		c.lenLimit = src.lenLimit
		src.mu.RLock()
		for k, v := range src.items {
			c.items[k] = &item{
				Object: v.Object,
				expire: v.expire,
			}
		}
		src.mu.RUnlock()
		c.lenCurrent = len(c.items)
	}
}

func OnEvent(handler OnEventHandler, events ...Event) func(*stash) {
	return func(c *stash) {
		if handler == nil || len(events) == 0 {
			return
		}
		c.events = &eventHandler{
			events:  events,
			handler: handler,
			next:    c.events,
		}
	}
}
