package webtail_test

// This file holds WebTailClient methods used in tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/a8m/djson"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/wojas/genericr"

	"github.com/LeKovr/webtail"
)

type WebTailClient struct {
	t            *testing.T
	ws           *websocket.Conn
	wtServer     *webtail.Service
	htServer     *httptest.Server
	interrupt    chan os.Signal
	done         chan struct{}
	feedBackChan chan struct{}
	expect       chan string
}

func NewWebTailClient(t *testing.T, cfg *webtail.Config) (*WebTailClient, error) {
	logger := genericr.New(func(e genericr.Entry) {
		t.Log(e.String())
	})
	wtServer, err := webtail.New(logger, cfg)
	if err != nil {
		return nil, err
	}
	htServer := httptest.NewServer(wtServer)
	// Convert http://127.0.0.1 to ws://127.0.0.
	u := "ws" + strings.TrimPrefix(htServer.URL, "http")
	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		return nil, err
	}

	wtc := &WebTailClient{
		t:            t,
		ws:           ws,
		wtServer:     wtServer,
		htServer:     htServer,
		done:         make(chan struct{}),
		expect:       make(chan string, 20),
		feedBackChan: make(chan struct{}, 5),
		interrupt:    make(chan os.Signal, 1),
	}
	go wtServer.Run()
	return wtc, nil
}

func (wtc *WebTailClient) WaitSync(syncs int) {
	for i := 0; i < syncs; i++ {
		ticker := time.NewTicker(time.Duration(1) * time.Second)
		defer ticker.Stop()
		select {
		case <-wtc.feedBackChan:
			wtc.t.Log("sync received")
			continue
		case <-ticker.C:
			wtc.t.Log("sync timeout")
			continue
		}
	}
}

func (wtc *WebTailClient) Call(cmd *webtail.InMessage, replies int, ordered bool) []string {
	err := wtc.ws.WriteJSON(cmd)
	require.Nil(wtc.t, err)
	return wtc.Receive(replies, ordered)
}

func (wtc *WebTailClient) Receive(replies int, ordered bool) []string {
	rv := make([]string, replies)
	for i := 0; i < replies; i++ {
		row := <-wtc.expect
		rv[i] = row
	}
	if len(rv) > 1 && ordered {
		sort.Strings(rv)
	}
	return rv
}

func (wtc *WebTailClient) Close() {
	ticker := time.NewTicker(time.Duration(1) * time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		wtc.t.Log("Stop by timeout")
		wsclose(wtc.ws, wtc.done)
	case <-wtc.interrupt:
		wtc.t.Log("Stop by event")
		wsclose(wtc.ws, wtc.done)
	}
	wtc.wtServer.Close()
	wtc.ws.Close()
	wtc.htServer.Close()
}

func (wtc *WebTailClient) Listener(limit int) {
	defer close(wtc.done)
	start := time.Now()
	t := wtc.t
	count := 0
	for {
		_, msg, err := wtc.ws.ReadMessage()
		if err != nil || err == io.EOF {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				require.NoError(t, err)
			}
			return
		}
		result := bytes.Split(msg, []byte(newline))
		for i := range result {
			val, err := djson.DecodeObject(result[i])
			require.Nil(t, err)
			if val["type"] == "index" {
				d := val["data"].(map[string]interface{})
				delete(d, "mtime")
				val["data"] = d
				if d["name"] == RootFile {
					wtc.t.Log("SyncIndex sent")
					wtc.feedBackChan <- struct{}{}
				}
			} else if val["type"] == "attach" {
				// `val["channel"] == ""` adds 1 sec to test timing
				_, ok := val["channel"]
				if !ok {
					wtc.t.Log("SyncAttach sent")
					wtc.feedBackChan <- struct{}{}
				}
			}
			ordered, err := json.Marshal(val)
			require.NoError(t, err)
			// fmt.Printf(">>>>>>>>>>>>>%s\n\n", ordered)
			wtc.expect <- string(ordered)
		}
		count += len(result)
		t.Log("recv:", count)
		if count == limit {
			t.Logf("%d messages received for %s", count, time.Since(start).String())
			wtc.interrupt <- os.Interrupt
			return
		}
	}
}

func wsclose(c *websocket.Conn, done chan struct{}) {
	// To cleanly close a connection, a client should send a close
	// frame and wait for the server to close the connection.
	err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			return
		}
		return // err
	}
	select {
	case <-done:
	case <-time.After(time.Second):
	}
	c.Close()
}
