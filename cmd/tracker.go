package main

import (
	"flag"
	"logger"
	"tracker"
)

func main() {

	expire := flag.Int("t", 3600, "how many seconds the peer expire")
	addr := flag.String("a", ":12345", "listen addr")
	debug := flag.Bool("debug", false, "debug mode")
	flag.Parse()
	logger.InitLogger(*debug)
	t := tracker.NewTracker(*addr, *expire)
	t.Server()

}
