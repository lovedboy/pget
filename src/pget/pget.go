package pget

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"logger"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"tracker"

	"github.com/juju/ratelimit"
)

const (
	BATCH_TIMEOUT  = 30
	HEAD_TIMEOUT   = 10
	DOWNLOAD_RETRY = 3
)

var g = logger.GetLogger()

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

type download struct {
	sourceURL  string
	trackerURL string
	md5        string
	concurrent int
	sync.Mutex
	batchMap  map[int64]bool
	size      int64
	dst       string
	wg        sync.WaitGroup
	batchSize int64
	th        *tracker.TrackerHelper
	// close http server
	closeServer    chan bool
	httpWg         sync.WaitGroup
	httpListenPort int
	// upload
	upload bool
	// upload time
	uploadTime int
	// download rate limit
	downloadRateLimit *ratelimit.Bucket
	downloadRate      int64
	// upload rate limit
	uploadRateLimit *ratelimit.Bucket
	uploadRate      int64
	// upload concurrent
	uploadConcurrent int
	// current upload conn
	curUploadConn int
	// http header
	downloadRequestHeader [][2]string
	trackerRequestHeader  [][2]string
}

func NewDownload(sourceURL, trackerURL, dst string, concurrent int, md5 string, batchSize int64, upload bool, uploadTime int, uploadConcurrent int) *download {
	d := &download{
		sourceURL:        sourceURL,
		trackerURL:       trackerURL,
		dst:              dst,
		concurrent:       concurrent,
		md5:              md5,
		wg:               sync.WaitGroup{},
		batchSize:        batchSize,
		closeServer:      make(chan bool),
		httpWg:           sync.WaitGroup{},
		upload:           upload,
		uploadTime:       uploadTime,
		uploadConcurrent: uploadConcurrent,
	}
	if d.trackerURL != "" && d.upload {
		d.th = &tracker.TrackerHelper{SourceURL: d.sourceURL, TrackerURL: d.trackerURL}
	}
	return d
}

func (d *download) SetDownloadRate(n int64) {
	d.downloadRate = n
	d.downloadRateLimit = ratelimit.NewBucketWithRate(float64(n), n)
}

func (d *download) SetUploadRate(n int64) {
	d.uploadRate = n
	d.uploadRateLimit = ratelimit.NewBucketWithRate(float64(n), n)
}

func (d *download) SetTrackerRequestHeader(params []string) {
	if d.th == nil {
		g.Warning("dont't set tracker or disable upload")
		return
	}
	for _, param := range params {
		header := strings.Split(param, ":")
		if len(header) != 2 {
			g.Warningf("invalid header:%s", param)
			continue
		}
		d.trackerRequestHeader = append(d.trackerRequestHeader, [2]string{header[0], header[1]})

	}
	d.th.RequestHeader = d.trackerRequestHeader
}

func (d *download) SetDownloadRequestHeader(params []string) {
	for _, param := range params {
		header := strings.Split(param, ":")
		if len(header) != 2 {
			g.Warningf("invalid header:%s", param)
			continue
		}
		d.downloadRequestHeader = append(d.downloadRequestHeader, [2]string{header[0], header[1]})

	}
}

func (d *download) Start() {
	if err := d.getSize(); err != nil {
		g.Fatalf("get file size error:%v", err)
	}
	d.genBatch()
	if d.th != nil {
		d.httpServer()
	}
	d.dispatch()
	if d.md5 != "" {
		md5, err := MD5sum(d.dst)
		if err != nil {
			g.Fatal(err)
		}
		if strings.ToLower(md5) != strings.ToLower(d.md5) {
			g.Fatal("md5 verify fail")
		} else {
			g.Infof("md5 verify pass")
		}
	}
	g.Info("download finish")
	if d.th != nil {
		go func() {
			time.Sleep(time.Duration(d.uploadTime * 1e9))
			close(d.closeServer)

		}()
		<-d.closeServer
		g.Info("close http server")
		d.httpWg.Wait()
	}
}

func (d *download) genBatch() {
	d.Lock()
	defer d.Unlock()
	if len(d.batchMap) == 0 {
		d.batchMap = make(map[int64]bool)
	}
	var i int64 = 0
	for ; i*d.batchSize < d.size; i++ {
		d.wg.Add(1)
		d.batchMap[i] = false
	}
	g.Debugf("the file have %d batch, size:%d ... \n", len(d.batchMap), d.size)

}

func (d *download) worker(b chan int64) {
	for {

		batch, ok := <-b
		if !ok {
			return
		}
		success := false
		for i := 1; i <= DOWNLOAD_RETRY && !success; i++ {
			for _, peer := range d.getPeers(batch) {
				if err := d.downloadBatch(peer, batch); err == nil {
					success = true
					g.Debugf("fetch batch:%d from:%s success .. \n", batch, peer)
					d.Lock()
					d.batchMap[batch] = true
					d.Unlock()
					d.wg.Done()
					d.announce(batch)
					break
				} else {
					g.Warningf("fetch batch:%d from:%s err: %v.. \n", batch, peer, err)
				}
			}
		}
		if !success {
			g.Fatalf("download batch:%d fail", batch)
		}
	}
}

func (d *download) dispatch() {
	d.Lock()
	length := len(d.batchMap)
	d.Unlock()
	batchChan := make(chan int64)
	for i := 1; i <= d.concurrent && i <= length; i++ {
		go d.worker(batchChan)
	}
	for k := 0; k < length; k++ {
		batchChan <- int64(k)
	}

	d.wg.Wait()
	close(batchChan)
}

func (d *download) setHeader(req *http.Request) {
	for _, k := range d.downloadRequestHeader {
		if strings.EqualFold(k[0], "host") {
			req.Host = k[1]
		} else {
			req.Header.Add(k[0], k[1])
		}
	}
}

func (d *download) getSize() (err error) {
	req, err := http.NewRequest("HEAD", d.sourceURL, nil)
	if err != nil {
		return
	}
	d.setHeader(req)
	hc := &http.Client{Timeout: time.Duration(time.Second * HEAD_TIMEOUT)}
	res, err := hc.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	io.Copy(ioutil.Discard, res.Body)
	if res.StatusCode != 200 {
		return errors.New(fmt.Sprintf("response http code should be 200, but real is %d", res.StatusCode))
	}
	d.size = res.ContentLength
	return

}

func (d *download) genRange(batch int64) (start int64, end int64) {
	start = batch * d.batchSize
	end = start + d.batchSize - 1
	if end > d.size-1 {
		end = d.size - 1
	}
	return start, end

}

func (d *download) announce(batch int64) {
	if d.th != nil {
		if err := d.th.PutPeer(fmt.Sprintf("%d", d.httpListenPort), batch, d.batchSize); err != nil {
			g.Warningf("announce url:%s err:%v", d.trackerURL, err)
		}
	}
}

func (d *download) getPeers(batch int64) (peers []string) {
	if d.th != nil {
		if peerFromTracker, err := d.th.GetPeer(batch, d.batchSize); err != nil {
			g.Warningf("get peer:%v", err)
		} else {
			peers = append(peers, peerFromTracker...)
		}
	}
	peers = append(peers, d.sourceURL)
	g.Debugf("peers for batch:%d is %v", batch, peers)
	return peers
}

func (d *download) downloadBatch(url string, batch int64) (err error) {

	g.Debugf("will fetch batch:%d from:%s.. \n", batch, url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	d.setHeader(req)
	start, end := d.genRange(batch)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	hc := &http.Client{Timeout: time.Duration(time.Second * BATCH_TIMEOUT)}
	res, err := hc.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 206 {
		return errors.New(fmt.Sprintf("response http code should be 206, but real is %d", res.StatusCode))
	}
	f, err := os.OpenFile(d.dst, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		g.Fatal(err)
	}
	_, err = f.Seek(start, 0)
	if err != nil {
		g.Fatal(err)
	}
	defer f.Close()
	var src io.Reader
	if d.downloadRateLimit != nil {
		src = ratelimit.Reader(res.Body, d.downloadRateLimit)
	} else {
		src = res.Body
	}
	n, err := io.Copy(f, src)
	if n != end-start+1 {
		return errors.New("invalid length")
	}
	return err
}

func (d *download) parseRange(rangeHeader string) (batch int64, err error) {
	rangeHeader = rangeHeader[len("bytes="):]
	rangeArray := strings.Split(rangeHeader, "-")
	if len(rangeArray) != 2 {
		return 0, errors.New(fmt.Sprintf("invalid range header:%s", rangeHeader))
	}
	start, err := strconv.ParseInt(rangeArray[0], 10, 0)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("invalid range header: %v ", err))
	}
	end, err := strconv.ParseInt(rangeArray[1], 10, 0)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("invalid range header: %v ", err))
	}
	if start%d.batchSize != 0 {
		return 0, errors.New(fmt.Sprintf("invalid range start %d, start mod batch_size must be zero", start))
	}
	if end-start >= d.batchSize {
		return 0, errors.New(fmt.Sprintf("invalid range end %d, range size greater than batch size", end))

	}
	return start / d.batchSize, nil

}

func (d *download) httpServer() {

	srv := &http.Server{Addr: ":0", Handler: d}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		g.Fatal(err)
	}
	d.httpListenPort = ln.Addr().(*net.TCPAddr).Port
	g.Infof("listen at :%d", d.httpListenPort)
	go srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}

func (d *download) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	select {
	case <-d.closeServer:
		g.Info("receive close singal, will return")
		w.WriteHeader(500)
		w.Write([]byte("close connection"))
		return
	default:
	}

	d.Lock()
	if d.curUploadConn < d.uploadConcurrent {
		d.curUploadConn += 1
		d.Unlock()
	} else {
		d.Unlock()
		g.Warningf("upload conn is greater than upload concurrent:%d", d.uploadConcurrent)
		w.WriteHeader(500)
		w.Write([]byte("upload conn is full"))
		return
	}

	d.httpWg.Add(1)
	defer func() {
		d.httpWg.Done()
		d.Lock()
		d.curUploadConn -= 1
		d.Unlock()
	}()

	w.Header().Set("Content-type", "application/octet-stream")
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		w.WriteHeader(500)
		w.Write([]byte("invalid range"))
		return
	}
	batch, err := d.parseRange(rangeHeader)
	if err != nil {
		g.Warning(err)
		w.WriteHeader(500)
		w.Write([]byte("invalid range"))
	}
	d.Lock()
	ok := d.batchMap[batch]
	d.Unlock()
	if !ok {
		g.Warningf("batch:%d is not completed", batch)
		w.WriteHeader(500)
		w.Write([]byte("batch is not completed"))
		return
	}
	f, err := os.Open(d.dst)
	if err != nil {
		g.Error(f)
		w.WriteHeader(500)
		w.Write([]byte("error"))
		return
	}
	start, end := d.genRange(batch)
	defer f.Close()
	f.Seek(start, 0)
	length := end - start + 1
	buf := make([]byte, length)
	n, err := f.Read(buf)
	if err != nil {
		g.Error(err)
		w.WriteHeader(500)
		w.Write([]byte("error"))
		return
	}
	if int64(n) != length {
		g.Error("batch length is not euqal buf from read file")
		w.WriteHeader(500)
		w.Write([]byte("error"))
		return

	}
	w.WriteHeader(206)
	var dst io.Writer
	if d.uploadRateLimit != nil {
		dst = ratelimit.Writer(w, d.uploadRateLimit)
	} else {
		dst = w
	}
	dst.Write(buf)
}
