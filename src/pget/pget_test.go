package pget

import (
	"testing"

	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
	"tracker"

	"github.com/stretchr/testify/assert"
)

var lock = &sync.Mutex{}
var gt = tracker.NewTracker("localhost:22345", 600)
var testServerRunning = false

func runTestTrackerServer() {

	lock.Lock()
	defer lock.Unlock()
	if !testServerRunning {
		go gt.Server()
		go http.ListenAndServe("localhost:33345", http.FileServer(http.Dir("/tmp")))
		time.Sleep(time.Duration(1e8))
		testServerRunning = true
	}

}

func TestNewDownload(t *testing.T) {
	d := NewDownload("", "", "", 1, "", 1, false, 0, 3)
	assert.NotEqual(t, d, nil)
}

func TestNewDownload_genBatch(t *testing.T) {
	d := NewDownload("", "", "", 1, "", 2, false, 0, 3)
	d.size = 10
	d.genBatch()
	assert.Equal(t, len(d.batchMap), 5)
}

func TestNewDownload_genRange(t *testing.T) {
	d := NewDownload("", "", "", 1, "", 2, false, 0, 3)
	d.size = 10
	start, end := d.genRange(5)
	assert.Equal(t, start, int64(10))
	assert.Equal(t, end, int64(9))
}

func TestNewDownload_genBatch2(t *testing.T) {
	d := NewDownload("", "", "", 1, "", 3, false, 0, 3)
	d.size = 100
	d.genBatch()
	assert.Equal(t, len(d.batchMap), 34)
}

func TestNewDownload_genRange2(t *testing.T) {
	d := NewDownload("", "", "", 1, "", 3, false, 0, 3)
	d.size = 100
	start, end := d.genRange(33)
	assert.Equal(t, start, int64(99))
	assert.Equal(t, end, int64(99))
}

func TestDownload_parseRange(t *testing.T) {

	batch_size := 3
	d := NewDownload("", "", "", 1, "", int64(batch_size), false, 0, 3)

	batch, err := d.parseRange("bytes=0-2")
	assert.NoError(t, err)
	assert.Equal(t, batch, int64(0))

	batch, err = d.parseRange("bytes=3-5")
	assert.NoError(t, err)
	assert.Equal(t, batch, int64(1))

	batch, err = d.parseRange("bytes=0-3")
	assert.Error(t, err)
	assert.Equal(t, batch, int64(0))

	batch, err = d.parseRange("bytes=0-ttt")
	assert.Error(t, err)
	assert.Equal(t, batch, int64(0))
}

func TestDownload_announce(t *testing.T) {
	d := NewDownload("http://localhost/", "http://localhost", "", 1, "", 0, true, 0, 3)
	d.announce(1)
}

func TestDownload_getPeers(t *testing.T) {
	d := NewDownload("http://localhost/", "http://localhost:11111", "", 1, "", 0, true, 0, 3)
	peers := d.getPeers(1)
	assert.Len(t, peers, 1)
	assert.Equal(t, peers[0], "http://localhost/")
}

func TestDownload_getPeers2(t *testing.T) {
	runTestTrackerServer()
	sourceURL := "http://test.com/test"
	trackURL := "http://localhost:22345"

	var batchSize int64 = 10
	th := tracker.TrackerHelper{SourceURL: sourceURL, TrackerURL: trackURL}
	err := th.PutPeer("12345", 1, batchSize)
	assert.NoError(t, err)
	d := NewDownload(sourceURL, trackURL, "", 1, "", batchSize, true, 0, 3)
	peers := d.getPeers(1)
	assert.Len(t, peers, 2)
	assert.Equal(t, peers[1], sourceURL)
	assert.Equal(t, peers[0], "http://127.0.0.1:12345")
}

func TestDownload_SetDownloadRate(t *testing.T) {

	d := NewDownload("http://localhost/", "http://localhost", "", 1, "", 0, true, 0, 3)
	d.SetDownloadRate(100)
	n := d.downloadRateLimit.TakeAvailable(100)
	assert.Equal(t, n, int64(100))

	n = d.downloadRateLimit.TakeAvailable(100)
	assert.Equal(t, n, int64(0))
}

func TestDownload_SetUploadRate(t *testing.T) {

	d := NewDownload("http://localhost/", "http://localhost", "", 1, "", 0, true, 0, 3)
	d.SetUploadRate(100)
	n := d.uploadRateLimit.TakeAvailable(100)
	assert.Equal(t, n, int64(100))

	n = d.uploadRateLimit.TakeAvailable(100)
	assert.Equal(t, n, int64(0))
}

func TestDownload_ServeHTTP(t *testing.T) {
	d := NewDownload("http://localhost/", "http://localhost", "", 1, "", 0, true, 0, 3)
	assert.True(t, d.httpListenPort == 0)
	d.httpServer()
	assert.True(t, d.httpListenPort != 0)
}

func TestDownload_ServeHTTP2(t *testing.T) {
	d := NewDownload("http://localhost/", "http://localhost", "", 1, "", 0, true, 0, 3)
	d.httpServer()
	res, err := http.Get(fmt.Sprintf("http://localhost:%d", d.httpListenPort))
	assert.NoError(t, err)
	assert.Equal(t, res.StatusCode, 500)
}

func TestDownload_downloadBatch(t *testing.T) {
	dst := "/tmp/pget"
	d := NewDownload("http://localhost/", "http://localhost", dst, 1, "", 11, true, 0, 3)
	d.size = 11
	d.batchMap = make(map[int64]bool)
	d.batchMap[0] = true
	d.httpServer()
	f, _ := os.Create(dst)
	f.WriteString("hello,world")
	f.Close()
	dst2 := "/tmp/pget2"
	d2 := NewDownload("http://localhost/", "http://localhost", dst2, 1, "", 11, true, 0, 3)
	d2.size = 11
	err := d2.downloadBatch(fmt.Sprintf("http://localhost:%d", d.httpListenPort), 0)
	assert.NoError(t, err)
	buf, err := ioutil.ReadFile(dst2)
	assert.NoError(t, err)
	defer func() {
		os.Remove(dst)
		os.Remove(dst2)
	}()
	assert.Equal(t, string(buf), "hello,world")
}

func TestDownload_downloadBatch2(t *testing.T) {
	runTestTrackerServer()
	f, _ := os.Create("/tmp/source")
	f.WriteString("hello,world")
	f.Close()
	defer os.Remove("/tmp/source")
	dst := "/tmp/pget"
	defer os.Remove(dst)
	d := NewDownload("http://localhost:33345/source", "", dst, 1, "", 11, false, 0, 3)
	d.getSize()
	d.genBatch()
	err := d.downloadBatch(d.sourceURL, 0)
	assert.NoError(t, err)
	buf, _ := ioutil.ReadFile(dst)
	assert.Equal(t, string(buf), "hello,world")
}

func TestDownload_getSize(t *testing.T) {
	runTestTrackerServer()
	f, _ := os.Create("/tmp/source")
	f.WriteString("hello,world")
	f.Close()
	defer os.Remove("/tmp/source")
	dst := "/tmp/pget"
	d := NewDownload("http://localhost:33345/source", "", dst, 1, "", 11, false, 0, 3)
	d.getSize()
	assert.Equal(t, d.size, int64(11))
}

func TestDownload_Start(t *testing.T) {

	runTestTrackerServer()
	f, _ := os.Create("/tmp/source")
	f.WriteString("hello,world")
	f.Close()
	defer os.Remove("/tmp/source")
	dst := "/tmp/pget"
	d := NewDownload("http://localhost:33345/source", "", dst, 1, "", 3, false, 0, 3)
	done := make(chan bool)
	go func() {
		d.Start()
		close(done)
	}()
	select {
	case <-done:
		defer os.Remove(dst)
		buf, _ := ioutil.ReadFile(dst)
		assert.Equal(t, string(buf), "hello,world")
	case <-time.After(1e9):
		assert.True(t, false)
	}

}

func TestDownload_SetDownloadRequestHeader(t *testing.T) {
	dst := "/tmp/test"
	d := NewDownload("http://localhost:33345/source", "", dst, 1, "", 11, false, 0, 3)
	d.SetDownloadRequestHeader([]string{"Host:127.0.0.1", "invalid", "User-Agent:pget"})
	assert.Len(t, d.downloadRequestHeader, 2)
	assert.Equal(t, d.downloadRequestHeader[0], [2]string{"Host", "127.0.0.1"})
	assert.Equal(t, d.downloadRequestHeader[1], [2]string{"User-Agent", "pget"})
}

func TestDownload_SetTrackerRequestHeader(t *testing.T) {
	dst := "/tmp/test"
	d := NewDownload("http://localhost:33345/source", "http://tracker.com", dst, 1, "", 11, true, 0, 3)
	d.SetTrackerRequestHeader([]string{"Host:127.0.0.1", "invalid", "User-Agent:pget"})
	assert.Len(t, d.th.RequestHeader, 2)
	assert.Equal(t, d.th.RequestHeader[0], [2]string{"Host", "127.0.0.1"})
	assert.Equal(t, d.th.RequestHeader[1], [2]string{"User-Agent", "pget"})
}
