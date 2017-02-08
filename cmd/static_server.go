package main

import (
	"flag"
	"fmt"
	"logger"
	"net/http"
	"os"
)

var (
	GitTag    = "2000.01.01.release"
	BuildTime = "2000-01-01T00:00:00+0800"
)

func main() {
	addr := flag.String("a", "", "listen addr")
	dir := flag.String("d", "", "static dir")
	version := flag.Bool("v", false, "version")
	flag.Parse()

	if *version {
		fmt.Printf("GitTag: %s \n", GitTag)
		fmt.Printf("BuildTime: %s \n", BuildTime)
		os.Exit(0)
	}
	g := logger.GetLogger()
	if *addr == "" {
		g.Fatalf("addr is null")
	}
	if *dir == "" {
		g.Fatalf("static dir is null")
	}
	http.ListenAndServe(*addr, http.FileServer(http.Dir(*dir)))
}
