package main

import (
	"flag"
	"pget"
)

func main() {

	source := flag.String("s", "", "source url")
	tracker := flag.String("t", "", "tracker url")
	dst := flag.String("d", "", "download dst")
	concurrent := flag.Int("c", 3, "concurrent")
	flag.Parse()
	p := pget.NewDownload(*source, *tracker, *dst, *concurrent)
	p.Start()

}
