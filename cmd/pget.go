package main

import (
	"flag"
	"fmt"
	"logger"
	"os"
	"pget"
)

var (
	GitTag    = "2000.01.01.release"
	BuildTime = "2000-01-01T00:00:00+0800"
)

func main() {

	source := flag.String("s", "", "source url")
	tracker := flag.String("t", "", "tracker url")
	dst := flag.String("d", "", "the dst path")
	concurrent := flag.Int("c", 3, "download concurrent")
	md5 := flag.String("m", "", "md5")
	batchSize := flag.Int64("b", 2, "batch size, unit is MB")
	debug := flag.Bool("debug", false, "debug mode")
	upload := flag.Bool("upload", true, "as a upload peer")
	uploadTime := flag.Int("upload-time", 60, "wait how many seconds to return when download finish")
	downloadRate := flag.Int64("download-rate", 0, "download rate limit, unit is Mb")
	uploadRate := flag.Int64("upload-rate", 0, "upload rate limit, unit is Mb")
	uploadConcurrent := flag.Int("upload-concurrent", 3, "upload concurrent")
	version := flag.Bool("v", false, "version")
	flag.Parse()

	if *version {
		fmt.Printf("GitTag: %s \n", GitTag)
		fmt.Printf("BuildTime: %s \n", BuildTime)
		os.Exit(0)
	}
	logger.InitLogger(*debug)

	g := logger.GetLogger()

	if *dst == "" {
		g.Fatal("dst is required")
	}

	if *source == "" {
		g.Fatal("source url is required")
	}

	p := pget.NewDownload(*source, *tracker, *dst, *concurrent, *md5, *batchSize*1024*1024, *upload, *uploadTime, *uploadConcurrent)

	if *downloadRate > 0 {
		p.SetDownloadRate(*downloadRate * 1024 * 1024 / 8)
	}
	if *uploadRate > 0 {
		p.SetUploadRate(*uploadRate * 1024 * 1024 / 8)
	}
	p.Start()

}
