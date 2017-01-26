package main

import (
	"flag"
	"fmt"
	"logger"
	"os"
	"tracker"
)

var (
	GitTag    = "2000.01.01.release"
	BuildTime = "2000-01-01T00:00:00+0800"
)

func main() {

	expire := flag.Int("t", 3600, "how many seconds the peer expire")
	addr := flag.String("a", ":12345", "listen addr")
	debug := flag.Bool("debug", false, "debug mode")
	version := flag.Bool("v", false, "version")
	flag.Parse()

	if *version {
		fmt.Printf("GitTag: %s \n", GitTag)
		fmt.Printf("BuildTime: %s \n", BuildTime)
		os.Exit(0)
	}
	flag.Parse()
	logger.InitLogger(*debug)
	t := tracker.NewTracker(*addr, *expire)
	t.Server()

}
