package tracker

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"gopkg.in/bufio.v1"
)

type TrackerHelper struct {
	SourceURL     string
	TrackerURL    string
	RequestHeader [][2]string
}

func (t *TrackerHelper) setHeader(req *http.Request) {
	for _, k := range t.RequestHeader {
		if strings.EqualFold(k[0], "host") {
			req.Host = k[1]
		} else {
			req.Header.Add(k[0], k[1])
		}
	}
}

func (t *TrackerHelper) PutPeer(port string, bat int64, bat_size int64) (err error) {
	req, err := http.NewRequest("PUT", t.TrackerURL, nil)
	if err != nil {
		return
	}
	t.setHeader(req)
	q := req.URL.Query()
	q.Add("source", t.SourceURL)
	q.Add("port", port)
	q.Add("batch", fmt.Sprintf("%d", bat))
	q.Add("batch_size", fmt.Sprintf("%d", bat_size))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return
	}

	defer resp.Body.Close()
	resp_body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("http code is %s, body is %s", resp.StatusCode, resp_body))
	}
	return
}

func (t *TrackerHelper) GetPeer(bat int64, bat_size int64) (peers []string, err error) {
	req, err := http.NewRequest("GET", t.TrackerURL, nil)
	if err != nil {
		return []string{}, err
	}
	t.setHeader(req)
	q := req.URL.Query()
	q.Add("source", t.SourceURL)
	q.Add("batch", fmt.Sprintf("%d", bat))
	q.Add("batch_size", fmt.Sprintf("%d", bat_size))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()
	resp_body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return []string{}, errors.New(fmt.Sprintf("http code is %d, body is %s", resp.StatusCode, resp_body))
	}
	buf := bufio.NewBuffer(resp_body)
	for {
		peer, err := buf.ReadString('\n')
		if err != nil {
			break
		}
		peers = append(peers, strings.TrimSpace(peer))
	}
	return peers, nil
}
