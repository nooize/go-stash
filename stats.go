package stash

import (
	"strconv"
	"strings"
	"sync/atomic"
)

type Stats struct {
	gets    atomic.Uint64
	misses  atomic.Uint64
	sets    atomic.Uint64
	updates atomic.Uint64
	touches atomic.Uint64
	errors  atomic.Uint64
	removes atomic.Uint64
	expires atomic.Uint64
	flushes atomic.Uint64
}

func (i *Stats) Gets() uint64 {
	return i.gets.Load()
}

func (i *Stats) Misses() uint64 {
	return i.misses.Load()
}

func (i *Stats) Sets() uint64 {
	return i.sets.Load()
}

func (i *Stats) Updates() uint64 {
	return i.updates.Load()
}

func (i *Stats) Touches() uint64 {
	return i.touches.Load()
}

func (i *Stats) Errors() uint64 {
	return i.errors.Load()
}

func (i *Stats) Removes() uint64 {
	return i.removes.Load()
}

func (i *Stats) Expires() uint64 {
	return i.expires.Load()
}

func (i *Stats) Flushes() uint64 {
	return i.flushes.Load()
}

func (i *Stats) String() string {
	parts := make([]string, 0)
	if v := i.Gets(); v > 0 {
		parts = append(parts, "get:", strconv.FormatUint(v, 10))
	}
	if v := i.Misses(); v > 0 {
		parts = append(parts, "miss:", strconv.FormatUint(v, 10))
	}
	if v := i.Sets(); v > 0 {
		parts = append(parts, "set:", strconv.FormatUint(v, 10))
	}
	if v := i.Updates(); v > 0 {
		parts = append(parts, "update:", strconv.FormatUint(v, 10))
	}
	if v := i.Touches(); v > 0 {
		parts = append(parts, "touch:", strconv.FormatUint(v, 10))
	}
	if v := i.Errors(); v > 0 {
		parts = append(parts, "errors:", strconv.FormatUint(v, 10))
	}
	if v := i.Removes(); v > 0 {
		parts = append(parts, "remove:", strconv.FormatUint(v, 10))
	}
	if v := i.Expires(); v > 0 {
		parts = append(parts, "expire:", strconv.FormatUint(v, 10))
	}
	if v := i.Flushes(); v > 0 {
		parts = append(parts, "flush:", strconv.FormatUint(v, 10))
	}
	return strings.Join(parts, " ")
}

func (i *Stats) EventHandler(event Event, key string, obj interface{}) {
	switch event {
	case EventGet:
		i.gets.Add(1)
	case EventMiss:
		i.misses.Add(1)
	case EventSet:
		i.sets.Add(1)
	case EventUpdate:
		i.updates.Add(1)
	case EventTouch:
		i.touches.Add(1)
	case EventError:
		i.errors.Add(1)
	case EventRemove:
		i.removes.Add(1)
	case EventExpire:
		i.expires.Add(1)
	case EventFlush:
		i.flushes.Add(1)
	}
}
