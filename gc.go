package stash

func stopGc(c *Stash) {
	c.gc.stop <- true
}

func startGc(c *stash) {
	j := &gc{
		Interval: c.gcPeriod,
		stop:     make(chan bool),
	}
	c.gc = j
	go j.Run(c)
}
