package tracker

import (
	"testing"
	"time"

	"net/http"
	"sync"

	"io/ioutil"

	"github.com/stretchr/testify/assert"
)

var lock = &sync.Mutex{}
var gt = &track{addr: ":12345", expireTTL: 3600}
var testServerRunning = false

func runTestServer() {

	lock.Lock()
	defer lock.Unlock()
	if !testServerRunning {
		go gt.Server()
		time.Sleep(time.Duration(1e8))
		testServerRunning = true
	}

}

func TestNewTracker(t *testing.T) {
	tracker := NewTracker(":12345", 10)
	assert.Equal(t, tracker.expireTTL, 10)
}

func TestTracker_addPeer(t *testing.T) {
	tracker := &track{}
	tracker.addPeer("1", "1", 1, 1)
	tracker.addPeer("1", "2", 1, 1)

	e, ok := tracker.sourceExpire["1"]
	assert.True(t, ok)
	assert.True(t, e.Day() == time.Now().Day())
	assert.Equal(t, tracker.sourceBatchMap["1"][1][1], []string{"1", "2"})
}

func TestTracker_getPeer(t *testing.T) {
	tracker := &track{}
	peers := tracker.getPeer("1", 1, 1)
	assert.Equal(t, 0, len(peers))
}

func TestTracker_getPeer2(t *testing.T) {
	tracker := &track{}
	tracker.addPeer("1", "1", 1, 1)
	peers := tracker.getPeer("1", 1, 1)
	assert.Equal(t, 1, len(peers))
	assert.Equal(t, peers, []string{"1"})
}

func TestTracker_deleteSource(t *testing.T) {

	tracker := &track{expireTTL: 10}
	tracker.addPeer("1", "1", 1, 1)
	assert.Equal(t, 1, len(tracker.sourceBatchMap))
	assert.Equal(t, 1, len(tracker.sourceExpire))
	tracker.sourceExpire["1"] = time.Time{}
	tracker.deleteSource()
	assert.Equal(t, 0, len(tracker.sourceBatchMap))
	assert.Equal(t, 0, len(tracker.sourceExpire))

}

func TestTrack_Server(t *testing.T) {
	runTestServer()
	resp, err := http.Get("http://localhost:12345")
	body, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 500)
	assert.Equal(t, string(body), "invalid request")
}

func TestTrackerHelper_GetPeer(t *testing.T) {
	runTestServer()
	th := TrackerHelper{SourceURL: "http://source.com/test.pkg", TrackerURL: "http://localhost:12345"}
	peers, err := th.GetPeer(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, len(peers), 0)
}

func TestTrackerHelper_GetPeer2(t *testing.T) {

	runTestServer()
	th := TrackerHelper{SourceURL: "http://source.com/test.pkg", TrackerURL: "http://localhost:12345"}
	gt.addPeer(th.SourceURL, "http://localhost/test", 1, 1)
	peers, err := th.GetPeer(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, len(peers), 1)
	assert.Equal(t, peers[0], "http://localhost/test")

	gt.addPeer(th.SourceURL, "http://localhost/test2", 1, 1)

	peers, err = th.GetPeer(1, 1)
	assert.Equal(t, len(peers), 2)
}

func TestTrackerHelper_PutPeer(t *testing.T) {

	runTestServer()
	th := TrackerHelper{SourceURL: "http://source.com/test.pkg", TrackerURL: "http://localhost:12345"}
	err := th.PutPeer("12345", 100, 100)
	assert.NoError(t, err)
	peers := gt.sourceBatchMap[th.SourceURL][100][100]
	assert.Equal(t, len(peers), 1)
	assert.Contains(t, peers[0], "12345")
}
