package stash

const (
	EventAny StashEvent = iota
	EventGet
	EventMiss
	EventSet
	EventUpdate
	EventTouch
	EventRemove
	EventExpire
	EventFlush
	EventClean
)

type StashEvent int

func (e StashEvent) String() string {
	switch e {
	case EventAny:
		return "*"
	case EventGet:
		return "GET"
	case EventMiss:
		return "MISS"
	case EventSet:
		return "SET"
	case EventUpdate:
		return "UPDATE"
	case EventTouch:
		return "TOUCH"
	case EventRemove:
		return "REMOVE"
	case EventExpire:
		return "EXPIRE"
	case EventFlush:
		return "FLUSH"
	}
	return "[UNDEFINED]"
}

// type OnHandler func(StashEvent, string, interface{})

type OnEventHandler func(StashEvent, string, interface{})

type eventHandler struct {
	events  []StashEvent
	handler OnEventHandler
	next    *eventHandler
}

func (h *eventHandler) fire(event StashEvent, key string, val interface{}) {
	if h == nil {
		return
	}
	if h.next != nil {
		go h.next.fire(event, key, val)
	}
	for _, e := range h.events {
		if e == EventAny || e == event {
			go h.handler(event, key, val)
			return
		}
	}
}
