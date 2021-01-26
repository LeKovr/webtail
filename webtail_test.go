package webtail_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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

type received struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (ss *ServerSuite) TestIndex() {
	logger := genericr.New(func(e genericr.Entry) {
		ss.T().Log(e.String())
	})
	srv, err := webtail.New(logger, &ss.cfg)
	require.NoError(ss.T(), err)
	go srv.Run()
	defer srv.Close()
	s := httptest.NewServer(srv)
	defer s.Close()
	// Convert http://127.0.0.1 to ws://127.0.0.
	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.Nil(ss.T(), err)
	defer ws.Close()

	done := make(chan struct{})
	feedBackChan := make(chan struct{}, 3)
	interrupt := make(chan os.Signal, 1)
	go ss.listenWS(ws, done, feedBackChan, interrupt)

	err = ws.WriteJSON(&webtail.InMessage{Type: "detach"})
	require.Nil(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "attach"})
	require.NoError(ss.T(), err)

	ss.waitSync(feedBackChan) // wait for indexer started
	testFile := ss.cfg.Root + "/" + RootFile

	f, err := os.Create(testFile)
	require.NoError(ss.T(), err)
	_, err = f.WriteString("test log row zero\ntest log row one\ntest log row two\n")
	require.NoError(ss.T(), err)
	f.Close()

	err = ws.WriteJSON(&webtail.InMessage{Type: "trace"})
	require.Nil(ss.T(), err)
	err = ws.WriteJSON(&webtail.InMessage{Type: "trace", Channel: "on"})
	require.Nil(ss.T(), err)

	ss.waitSync(feedBackChan) // wait for RootFile create
	ss.waitSync(feedBackChan) // wait for RootFile write

	err = ws.WriteJSON(&webtail.InMessage{Type: "attach", Channel: RootFile})
	require.Nil(ss.T(), err)
	err = ws.WriteJSON(&webtail.InMessage{Type: "attach", Channel: RootFile})
	require.Nil(ss.T(), err)

	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(ss.T(), err)
	_, err = f.WriteString("test log row three\n")
	require.NoError(ss.T(), err)
	f.Close()
	ss.waitSync(feedBackChan) // wait for RootFile write
	os.Remove(testFile)

	err = ws.WriteJSON(&webtail.InMessage{Type: "detach", Channel: RootFile})
	require.Nil(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "detach", Channel: RootFile})
	require.Nil(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "stats"})
	require.NoError(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "detach"})
	require.NoError(ss.T(), err)

	ss.T().Log("final")
	ticker := time.NewTicker(time.Duration(2) * time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		ss.T().Log("by timout")
		wsclose(ws, done)
	case <-interrupt:
		ss.T().Log("by event")
		wsclose(ws, done)
	}
}

func (ss *ServerSuite) listenWS(ws *websocket.Conn, done, back chan struct{}, interrupt chan os.Signal) {
	defer close(done)
	cnt := 0
	start := time.Now()
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				require.Nil(ss.T(), err)
			}
			return
		}
		result := bytes.Split(msg, []byte(newline))
		for i := range result {
			x := received{}
			err := json.Unmarshal(result[i], &x)
			require.Nil(ss.T(), err)
			if x.Type == "attach" && x.Channel == "" {
				ss.T().Log("Sync 1 sent")
				back <- struct{}{}
			} else if x.Type == "index" {
				file := &webtail.IndexItemEvent{}
				err := json.Unmarshal(x.Data, &file)
				require.Nil(ss.T(), err)
				ss.T().Log(fmt.Sprintf("<<  %s(%s) - %s / %d", x.Type, x.Channel, file.Name, file.Size))
				if file.Name == RootFile {
					ss.T().Log("Sync 2 sent")
					back <- struct{}{}
				}
				continue
			}
			ss.T().Log(fmt.Sprintf("<<  %s(%s) - %s", x.Type, x.Channel, string(x.Data)))
		}
		cnt += len(result)
		ss.T().Log("recv:", cnt)
		if cnt == 14 {
			ss.T().Logf("%d messages received for %s", cnt, time.Since(start).String())
			interrupt <- os.Interrupt
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

func (ss *ServerSuite) waitSync(back chan struct{}) {
	ticker := time.NewTicker(time.Duration(1) * time.Second)
	defer ticker.Stop()
	select {
	case <-back:
		ss.T().Log("sync received")
		return
	case <-ticker.C:
		ss.T().Log("sync timeout")
		return
	}
}
