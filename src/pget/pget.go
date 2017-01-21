package pget

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	VERSION       = "0.1"
	BATCH_SIZE    = 2 * 1024 * 1024
	BATCH_TIMEOUT = 30
	HEAD_TIMEOUT  = 10
)

const ()

type download struct {
	sourceURL  string
	trackerURL string
	md5        string
	concurrent int
	sync.Mutex
	batchMap map[int64]bool
	size     int64
	dst      string
	upload   bool
	wg       sync.WaitGroup
}

func NewDownload(source, tracker, dst string, concurrent int) *download {
	return &download{
		sourceURL:  source,
		trackerURL: tracker,
		dst:        dst,
		concurrent: concurrent,
		wg:         sync.WaitGroup{},
	}
}

func (d *download) Start() {
	if err := d.getSize(); err != nil {
		log.Fatal(err)
	}
	d.genBatch()
	d.dispatch()
}

func (d *download) genBatch() {
	d.Lock()
	defer d.Unlock()
	if len(d.batchMap) == 0 {
		d.batchMap = make(map[int64]bool)
	}
	var i int64 = 0
	for ; i*BATCH_SIZE < d.size; i++ {
		d.wg.Add(1)
		d.batchMap[i] = false
	}
	log.Printf("the file have %d batch, size:%d ... \n", len(d.batchMap), d.size)

}

func (d *download) worker(b chan int64) {
	for {

		batch, ok := <-b
		if !ok {
			return
		}
		if err := d.downloadBatch(d.sourceURL, batch); err == nil {
			log.Printf("will fetch batch:%d from:%s success .. \n", batch, d.sourceURL)
			d.Lock()
			d.batchMap[batch] = true
			d.Unlock()
			d.wg.Done()
		} else {
			log.Printf("will fetch batch:%d from:%s err: %v.. \n", batch, d.sourceURL, err)
			b <- batch
		}
	}
}

func (d *download) dispatch() {
	batchChan := make(chan int64, len(d.batchMap))
	for k := 0; k < len(d.batchMap); k++ {
		batchChan <- int64(k)
	}
	for i := 1; i <= d.concurrent && i <= len(d.batchMap); i++ {
		go d.worker(batchChan)
	}

	d.wg.Wait()
	close(batchChan)
}

func (d *download) getSize() (err error) {
	req, err := http.NewRequest("HEAD", d.sourceURL, nil)
	if err != nil {
		return
	}
	hc := &http.Client{Timeout: time.Duration(time.Second * HEAD_TIMEOUT)}
	res, err := hc.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	io.Copy(ioutil.Discard, res.Body)
	if res.StatusCode != 200 {
		return errors.New("response http code should be 200")
	}
	d.size = res.ContentLength
	return

}

func (d *download) downloadBatch(url string, batch int64) (err error) {

	log.Printf("will fetch batch:%d from:%s.. \n", batch, url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	start := batch * BATCH_SIZE
	end := start + BATCH_SIZE - 1
	if end > d.size-1 {
		end = d.size - 1
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	hc := &http.Client{Timeout: time.Duration(time.Second * BATCH_TIMEOUT)}
	res, err := hc.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 206 {
		return errors.New("response http code should be 206")
	}
	f, err := os.OpenFile(d.dst, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.Seek(start, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	n, err := io.Copy(f, res.Body)
	if n != end-start+1 {
		return errors.New("invalid length")
	}
	return err
}

func (d *download) serverHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/octet-stream")
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == ""{
		w.WriteHeader(500)
		w.Write([]byte("invalid range"))
		return
	}
	rangeHeader = rangeHeader[len("bytes="):]
	w.WriteHeader(206)
}

func (d *download) upload(){

	http.ListenAndServe("0:0", nil)
}
