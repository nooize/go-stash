package stash

import (
	"runtime"
	"time"
)

const (
	OnGet StashEvent = iota
	OnSet
	OnUpdate
	OnTouch
	OnRemove
	OnExpire
	OnFlush
	OnClean
)

const (
	// NoSizeLimit - For use with functions that take an expiration time.
	NoSizeLimit int = 0

	// NoExpire - For use with functions that take an expiration time.
	NoExpire time.Duration = -1
	// DefaultExpire For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// when the stash was created (e.g. 5 minutes.)
	DefaultExpire time.Duration = 0

	DefaultCgPeriod = 5 * time.Minute
)

type OnEventHandler func(StashEvent, string, interface{})

func New(opts ...func(*stash)) *Stash {
	c := &stash{
		items: make(map[string]*item),
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

func EventHandler(h OnEventHandler) func(*stash) {
	return func(c *stash) {
		c.onEvent = h
	}
}
