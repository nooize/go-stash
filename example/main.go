package main

import (
	"github.com/nooize/go-stash"
	"log"
	"time"
)

func main() {

	stats := new(stash.Stats)

	cache := stash.New(
		stash.GcPeriod(50*time.Millisecond),
		stash.ExpireAfter(30*time.Second),
		stash.OnEvent(stats.EventHandler, stash.EventAny),
		stash.OnEvent(handleChange, stash.EventExpire, stash.EventFlush),
	)

	cache.Set("key-0", "000", stash.NoExpire)
	cache.Set("key-1", "str")
	cache.Set("key-2", 12, 10*time.Second)

	log.Printf("sleep...len: %v", cache.Len())
	time.Sleep(15 * time.Second)
	log.Printf("wake ip !  len: %v", cache.Len())
	time.Sleep(20 * time.Second)
	log.Printf("wake ip again !  len: %v", cache.Len())
	cache.Clean()
	log.Printf("finish. len: %v", cache.Len())
	log.Printf("Stats > %s", stats.String())
}

func handleChange(e stash.StashEvent, k string, v interface{}) {
	log.Printf("%s: %s = %v", e, k, v)
}
