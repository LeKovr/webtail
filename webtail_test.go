package webtail_test

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

	"github.com/gorilla/websocket"
	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/wojas/genericr"

	"github.com/LeKovr/webtail"
)

// var upgrader = websocket.Upgrader{}

const (
	newline = "\n"

	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

type ServerSuite struct {
	suite.Suite
	cfg webtail.Config
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

	ss.cfg.Root = "testdata"
	ss.cfg.Trace = true // testing.Verbose()
	ss.cfg.Bytes = 20
}

func (ss *ServerSuite) TearDownSuite() {
	//	os.Remove(ss.cfg.Root + RootFile)
	os.Remove(ss.cfg.Root + SubFile)
}

type received struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (ss *ServerSuite) TestIndex() {
	logger := genericr.New(func(e genericr.Entry) {
		//		if testing.Verbose() {
		//			ss.printLogs(recorded)
		//		}
		ss.T().Log(e.String())
		// e.FieldsMap()
		//fmt.Println(e.String())
	})
	srv, err := webtail.New(logger, &ss.cfg)
	require.NoError(ss.T(), err)
	go srv.Run()
	defer srv.Close()
	s := httptest.NewServer(srv) // http.HandlerFunc(ss.srv.Handle))
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

	//err = ws.WriteJSON(&webtail.InMessage{Type: "detach"})
	//require.Nil(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "attach"}) // , Channel: "#"})
	require.NoError(ss.T(), err)

	waitSync(feedBackChan) // wait for indexer started
	testFile := ss.cfg.Root + "/" + RootFile
	fmt.Println("file:", testFile)

	f, err := os.Create(testFile)
	require.NoError(ss.T(), err)
	_, err = f.WriteString("test log row zero\ntest log row one\ntest log row two\n")
	require.NoError(ss.T(), err)
	f.Sync()
	f.Close()

	err = ws.WriteJSON(&webtail.InMessage{Type: "trace"})
	require.Nil(ss.T(), err)
	err = ws.WriteJSON(&webtail.InMessage{Type: "trace", Channel: "on"})
	require.Nil(ss.T(), err)

	waitSync(feedBackChan) // wait for RootFile create
	waitSync(feedBackChan) // wait for RootFile write

	err = ws.WriteJSON(&webtail.InMessage{Type: "attach", Channel: RootFile})
	require.Nil(ss.T(), err)
	err = ws.WriteJSON(&webtail.InMessage{Type: "attach", Channel: RootFile})
	require.Nil(ss.T(), err)

	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	_, err = f.WriteString("test log row three\n")
	require.NoError(ss.T(), err)
	f.Sync()
	f.Close()
	waitSync(feedBackChan) // wait for RootFile write
	os.Remove(testFile)

	err = ws.WriteJSON(&webtail.InMessage{Type: "detach", Channel: RootFile})
	require.Nil(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "detach", Channel: RootFile})
	require.Nil(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "stats"})
	require.NoError(ss.T(), err)

	err = ws.WriteJSON(&webtail.InMessage{Type: "detach"}) // , Channel: "#"})
	require.NoError(ss.T(), err)

	//	start := time.Now()
	fmt.Println("file3")

	ticker := time.NewTicker(time.Duration(2) * time.Second)
	defer ticker.Stop()

	log.Println("final")
	select {
	case <-ticker.C:
		log.Println("by timout")
		wsclose(ws, done)
	case <-interrupt:
		log.Println("by flag")
		wsclose(ws, done)
	}
}

func (ss *ServerSuite) listenWS(ws *websocket.Conn, done, back chan struct{}, interrupt chan os.Signal) {
	//      defer c.Close()
	defer close(done)
	cnt := 0

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			//	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			//		require.Nil(ss.T(), err)
			//	}
			return
		}
		result := bytes.Split(msg, []byte(newline))
		for i := range result {
			// log.Println(">>>>", string(result[i]))
			x := received{} // Out{}
			err := json.Unmarshal(result[i], &x)
			require.Nil(ss.T(), err)
			fmt.Printf("==>> %s\n", x) //.Data)
			if x.Type == "attach" && x.Channel == "" {
				fmt.Println("--------------->>attach index")
				back <- struct{}{}
			} else if x.Type == "index" {
				//fmt.Println(">>updated index")
				file := &webtail.IndexItemEvent{}
				err := json.Unmarshal(x.Data, &file)
				require.Nil(ss.T(), err)
				fmt.Println("--------------->>updated index for " + file.Name)
				if file.Name == RootFile {
					back <- struct{}{}
				}
			}

		}
		cnt += len(result)
		log.Printf("recv: %d", cnt)
		if cnt == 12 {
			// log.Printf("%d messages received for %s", cnt, time.Since(start).String())
			interrupt <- os.Interrupt
			return
		}
	}
}

func wsclose(c *websocket.Conn, done chan struct{}) {
	// log.Printf("Received %d messages", cnt)

	// To cleanly close a connection, a client should send a close
	// frame and wait for the server to close the connection.
	err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			err = nil // log.Println("write close:", err)
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

func waitSync(back chan struct{}) {

	//	<-back
	//	fmt.Println(">>>>>>>>>>>>>>> received message")

	ticker1 := time.NewTicker(time.Duration(1) * time.Second)
	defer ticker1.Stop()
	//	for {
	select {
	case <-back:
		fmt.Println("received message")
		return
	case <-ticker1.C:
		fmt.Println("no message received")
		return
		//		default:
		//			time.Sleep(time.Duration(5) * time.Millisecond)
	}
	//	}

}
