package webtail_test

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
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/wojas/genericr"

	"github.com/LeKovr/webtail"
)

const (
	newline = "\n"

	// File for testing
	RootFile = "file1.log"
)

type ServerSuite struct {
	suite.Suite
	cfg webtail.Config
}

func (ss *ServerSuite) SetupSuite() {
	// Fill config with default values
	p := flags.NewParser(&ss.cfg, flags.Default)
	_, err := p.ParseArgs([]string{})
	require.NoError(ss.T(), err)

	ss.cfg.Root = "testdata"
	ss.cfg.Trace = true // testing.Verbose()
	ss.cfg.Bytes = 20
}

func (ss *ServerSuite) TearDownSuite() {
	os.Remove(ss.cfg.Root + "/" + RootFile)
}

func (ss *ServerSuite) TestSimpleCommands() {
	wtc, err := NewWebTailClient(ss.T(), &ss.cfg)
	require.NoError(ss.T(), err)
	defer wtc.Close()
	tests := []struct {
		name string
		cmd  *webtail.InMessage
		want []string
	}{
		{
			name: "Get current trace",
			cmd:  &webtail.InMessage{Type: "trace"},
			want: []string{`{"enabled":true,"type":"trace"}`},
		}, {
			name: "Disable trace",
			cmd:  &webtail.InMessage{Type: "trace", Channel: "off"},
			want: []string{`{"enabled":false,"type":"trace"}`},
		}, {
			name: "Enable trace",
			cmd:  &webtail.InMessage{Type: "trace", Channel: "on"},
			want: []string{`{"enabled":true,"type":"trace"}`},
		}, {
			name: "Try to unsubscribe when not subscribed",
			cmd:  &webtail.InMessage{Type: "detach"},
			want: []string{`{"data":"not subscribed","type":"error"}`},
		}, {
			name: "Try to subscribe to unexistent channel",
			cmd:  &webtail.InMessage{Type: "attach", Channel: ".notexists"},
			want: []string{`{"channel":".notexists","data":"unknown channel","type":"error"}`},
		}, {
			name: "Try to unsubscribe from unexistent channel",
			cmd:  &webtail.InMessage{Type: "detach", Channel: ".notexists"},
			want: []string{`{"channel":".notexists","data":"unknown channel","type":"error"}`},
		}, {
			name: "Subscribe on file",
			cmd:  &webtail.InMessage{Type: "attach", Channel: "subdir/another.log"},
			want: []string{`{"channel":"subdir/another.log","data":"success","type":"attach"}`},
		}, {
			name: "Set subscriber count",
			cmd:  &webtail.InMessage{Type: "stats"},
			want: []string{`{"data":{"subdir/another.log":1},"type":"stats"}`},
		}, {
			name: "Try to subscribe again",
			cmd:  &webtail.InMessage{Type: "attach", Channel: "subdir/another.log"},
			want: []string{`{"channel":"subdir/another.log","data":"attached already","type":"error"}`},
		},
	}
	go wtc.Listener(len(tests) + 1)

	for _, tt := range tests {
		got := wtc.Call(tt.cmd, len(tt.want), false)
		require.Equal(ss.T(), tt.want, got)
	}

	want := []string{`{"data":"parse error","type":"error"}`}
	err = wtc.ws.WriteJSON(`bad"data`)
	require.NoError(wtc.t, err, "Incorrect request")
	got := wtc.Receive(len(want), false)
	require.Equal(ss.T(), want, got)
}

func (ss *ServerSuite) TestTail() {
	wtc, err := NewWebTailClient(ss.T(), &ss.cfg)
	require.NoError(ss.T(), err)
	defer wtc.Close()
	go wtc.Listener(12)

	want := []string{`{"data":"success","type":"attach"}`}
	got := wtc.Call(&webtail.InMessage{Type: "attach"}, len(want), false)
	require.Equal(ss.T(), want, got)
	wtc.WaitSync(1) // wait for index attach

	testFile := ss.cfg.Root + "/" + RootFile
	f, err := os.Create(testFile)
	require.NoError(ss.T(), err)
	_, err = f.WriteString("test log row zero\ntest log row one\ntest log row two\n")
	require.NoError(ss.T(), err)
	f.Close()

	wtc.WaitSync(2) // wait for RootFile create & write
	want = []string{
		`{"data":{"name":"file.log","size":28},"type":"index"}`,
		`{"data":{"name":"file1.log","size":52},"type":"index"}`,
		`{"data":{"name":"file1.log","size":52},"type":"index"}`,
		`{"data":{"name":"subdir/another.log","size":22},"type":"index"}`,
	}
	got = wtc.Receive(len(want), true)
	require.Equal(ss.T(), want, got)

	want = []string{
		`{"channel":"file1.log","data":"success","type":"attach"}`,
	}
	got = wtc.Call(&webtail.InMessage{Type: "attach", Channel: RootFile}, len(want), false)
	require.Equal(ss.T(), want, got)

	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(ss.T(), err)
	_, err = f.WriteString("test log row three\n")
	require.NoError(ss.T(), err)
	f.Close()
	wtc.WaitSync(1) // wait for RootFile update

	want = []string{
		`{"channel":"file1.log","data":"success","type":"detach"}`,
		`{"data":"test log row three","type":"log"}`,
		`{"data":"test log row two","type":"log"}`,
		`{"data":{"name":"file1.log","size":71},"type":"index"}`,
	}
	got = wtc.Call(&webtail.InMessage{Type: "detach", Channel: RootFile}, len(want), true)
	require.Equal(ss.T(), want, got)

	os.Remove(testFile)
	wtc.WaitSync(1) // wait for RootFile delete

	want = []string{
		`{"data":{"deleted":true,"name":"file1.log","size":0},"type":"index"}`,
		`{"data":"success","type":"detach"}`,
	}
	got = wtc.Call(&webtail.InMessage{Type: "detach"}, len(want), false)
	require.Equal(ss.T(), want, got)
}

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

func TestSuite(t *testing.T) {
	myTest := &ServerSuite{}
	suite.Run(t, myTest)
}
