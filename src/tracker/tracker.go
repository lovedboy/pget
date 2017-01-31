package tracker

import (
	"fmt"
	"logger"
	"net/http"
	"strconv"
	"sync"
	"time"

	"strings"
)

const (
	EXPITE_TTL = 3600
)

var g = logger.GetLogger()

type track struct {
	addr           string
	sourceBatchMap map[string]map[int64]map[int64][]string
	sourceExpire   map[string]time.Time
	sync.Mutex
	expireTTL int
}

func NewTracker(addr string, expireTTL int) *track {
	if expireTTL == 0 {
		expireTTL = EXPITE_TTL
	}
	return &track{addr: addr, expireTTL: expireTTL}
}

func (t *track) addPeer(source string, peer string, batch int64, batch_size int64) {

	t.Lock()
	defer t.Unlock()
	if t.sourceExpire == nil {
		t.sourceExpire = make(map[string]time.Time)
	}

	if t.sourceBatchMap == nil {
		t.sourceBatchMap = make(map[string]map[int64]map[int64][]string)
	}

	if _, ok := t.sourceBatchMap[source]; !ok {
		t.sourceBatchMap[source] = make(map[int64]map[int64][]string)
	}
	if _, ok := t.sourceBatchMap[source][batch]; !ok {
		t.sourceBatchMap[source][batch] = make(map[int64][]string)
	}

	t.sourceBatchMap[source][batch][batch_size] = append(t.sourceBatchMap[source][batch][batch_size], peer)

	t.sourceExpire[source] = time.Now()
}

func (t *track) getPeer(source string, batch int64, batch_size int64) []string {

	t.Lock()
	defer t.Unlock()
	v, ok := t.sourceBatchMap[source][batch][batch_size]
	if ok {
		return v
	} else {
		return []string{}
	}
}

func (t *track) deleteSource() {

	t.Lock()
	defer t.Unlock()
	for k, v := range t.sourceExpire {
		if int(time.Since(v).Seconds()) > t.expireTTL {
			g.Debugf("source:%s expire, will delete ... \n", k)
			delete(t.sourceExpire, k)
			delete(t.sourceBatchMap, k)
		}
	}
}

func (t *track) Server() {
	http.HandleFunc("/", t.serverHTTP)
	g.Infof("will listen at:%s ...\n", t.addr)
	go func() {
		for {

			time.Sleep(time.Second * EXPITE_TTL)
			t.deleteSource()
		}
	}()
	g.Fatal(http.ListenAndServe(t.addr, nil))
}

func (t *track) serverHTTP(w http.ResponseWriter, r *http.Request) {

	source := r.URL.Query().Get("source")
	batch := r.URL.Query().Get("batch")
	batch_size := r.URL.Query().Get("batch_size")
	if source == "" || batch == "" || batch_size == "" {
		g.Debugf("source or batch or batch_size is null")
		w.WriteHeader(500)
		w.Write([]byte("invalid request"))
		return
	}
	bat, err := strconv.ParseInt(batch, 10, 0)
	if err != nil {
		g.Error(err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	bat_size, err := strconv.ParseInt(batch_size, 10, 0)
	if err != nil {
		g.Error(err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	switch r.Method {
	case "GET":
		w.WriteHeader(200)
		peers := t.getPeer(source, bat, bat_size)
		for _, peer := range peers {
			fmt.Fprintln(w, peer)
		}
		return
	case "PUT":
		port := r.URL.Query().Get("port")
		if port == "" {
			g.Debug("peer is null")
			w.WriteHeader(500)
			w.Write([]byte("invalid peer"))
			return
		}
		var ip string
		if r.Header.Get("X-Real-IP") != "" {
			ip = r.Header.Get("X-Real-IP")
		} else {
			ip = strings.Split(r.RemoteAddr, ":")[0]
		}
		peer := fmt.Sprintf("http://%s:%s", ip, port)
		t.addPeer(source, peer, bat, bat_size)
		w.WriteHeader(200)
		g.Debugf("%s have batch:%d", peer, bat)
		return

	}
	g.Debug("unsuported method")
	w.WriteHeader(500)
	w.Write([]byte("invalid method"))
}
