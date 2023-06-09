package stash

const (
	EventAny Event = iota
	EventGet
	EventMiss
	EventSet
	EventUpdate
	EventTouch
	EventError
	EventRemove
	EventExpire
	EventFlush
	EventClean
)

// Event represents a stash event type.
type Event int

func (e Event) String() string {
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

// OnEventHandler is the type of the function called when an stash event is fired.
type OnEventHandler func(Event, string, interface{})

type eventHandler struct {
	events  []Event
	handler OnEventHandler
	next    *eventHandler
}

func (h *eventHandler) fire(event Event, key string, val interface{}) {
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
