package webtail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	mapper "github.com/birkirb/loggers-mapper-logrus"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gorilla/websocket"
)

//var upgrader = websocket.Upgrader{}

type ServerSuite struct {
	suite.Suite
	cfg  Config
	srv  *Service
	hook *test.Hook
}

const (
	RootFile = "file1.log"
	SubFile  = "subdir/file2.log"
)

func (ss *ServerSuite) SetupSuite() {
	// Fill config with default values
	p := flags.NewParser(&ss.cfg, flags.Default)
	_, err := p.ParseArgs([]string{})
	require.NoError(ss.T(), err)

	l, hook := test.NewNullLogger()
	ss.hook = hook
	if testing.Verbose() {
		l.SetLevel(logrus.DebugLevel)
	}
	log := mapper.NewLogger(l)

	hook.Reset()
	ss.cfg.Root = "testdata/"
	ss.cfg.Trace = testing.Verbose()
	ss.srv, err = New(log, ss.cfg)
	require.Nil(ss.T(), err)
	go ss.srv.Run()
}

/*
http.Handle("/tail", wt)

w := httptest.NewRecorder()
req, _ := http.NewRequest(tt.method, tt.url, tt.reader)
if tt.ctype != "" {
		req.Header.Set("Content-Type", tt.ctype)
}
srv.ServeHTTP(w, req)
*/
func (ss *ServerSuite) TearDownSuite() {
	os.Remove(ss.cfg.Root + RootFile)
	os.Remove(ss.cfg.Root + SubFile)
}

type received struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

func (ss *ServerSuite) TestIndex() {
	ss.hook.Reset()
	s := httptest.NewServer(ss.srv) //http.HandlerFunc(ss.srv.Handle))
	defer s.Close()
	// Convert http://127.0.0.1 to ws://127.0.0.
	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.Nil(ss.T(), err)
	defer ws.Close()

	err = ws.WriteJSON(&messageOut{Type: "attach"}) //, Channel: "#"})
	require.Nil(ss.T(), err)

	err = ws.WriteJSON(&messageOut{Type: "attach", Channel: "file.log"})
	require.Nil(ss.T(), err)
	err = ws.WriteJSON(&messageOut{Type: "attach", Channel: "file.log"})
	require.Nil(ss.T(), err)
	err = ws.WriteJSON(&messageOut{Type: "trace"})
	require.Nil(ss.T(), err)
	err = ws.WriteJSON(&messageOut{Type: "detach", Channel: "file.log"})
	require.Nil(ss.T(), err)

	start := time.Now()

	done := make(chan struct{})
	cnt := 0
	interrupt := make(chan os.Signal, 1)
	go func() {
		//      defer c.Close()
		defer close(done)
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					require.Nil(ss.T(), err)
				}
				return
			}
			result := bytes.Split(msg, newline)
			for i := range result {
				log.Println(">>>>", string(result[i]))
				x := received{} //Out{}
				err := json.Unmarshal(result[i], &x)
				require.Nil(ss.T(), err)
				log.Printf("==>> %s\n", x.Data)
			}
			cnt += len(result)
			log.Printf("recv: %d", cnt)
			if cnt == 9 {
				log.Printf("%d messages received for %s", cnt, time.Since(start).String())
				interrupt <- os.Interrupt
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	// err := c.WriteJSON(&message{Type: "stats", Channel: t.String()})
	//	for {
	select {
	case <-ticker.C:
		wsclose(ws, done, cnt)
	//	break //	return
	case <-interrupt:
		// log.Println("interrupt")
		wsclose(ws, done, cnt)
		//	break //	return
	}
	//break
	//	}

	ss.printLogs()

	//  require.NotNil(ss.T(), err)
	//  httpErr, ok := err.(interface{ Status() int })
	//  assert.True(ss.T(), ok)
	//  assert.Equal(ss.T(), http.StatusBadRequest, httpErr.Status())
	require.Nil(ss.T(), nil)
}

func wsclose(c *websocket.Conn, done chan struct{}, cnt int) {

	// log.Printf("Received %d messages", cnt)

	// To cleanly close a connection, a client should send a close
	// frame and wait for the server to close the connection.
	err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			log.Println("write close:", err)
		}
		return
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

func (ss *ServerSuite) printLogs() {
	for _, e := range ss.hook.Entries {
		fmt.Printf("ENT[%s]: %s\n", e.Level, e.Message)
	}
}
