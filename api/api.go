// Package api contains all API interaction
package api

import (
	"encoding/json"
	"golang.org/x/net/websocket"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LeKovr/go-base/log"
	"github.com/LeKovr/webtail/manager"
)

// -----------------------------------------------------------------------------

// Config defines local application flags
type Config struct {
	Host      string `long:"host"        default:""        description:"Hostname for page title"`
	ListCache int    `long:"cache"       default:"2"       description:"Time to cache file listing (sec)"`
}

// -----------------------------------------------------------------------------

// FileAttr holds File Attrs
type FileAttr struct {
	MTime time.Time `json:"mtime"`
	Size  int64     `json:"size"`
}

// FileStore holds all log files attrs
type FileStore map[string]*FileAttr

// Server holds service objects
type Server struct {
	Config      Config
	Root        string
	Log         log.Logger
	Manager     *manager.Manager
	List        *FileStore
	ListUpdated time.Time
	lock        *sync.RWMutex
}

// -----------------------------------------------------------------------------

type message struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Channel string `json:"channel,omitempty"`
	Error   string `json:"error,omitempty"`
}

type listmessage struct {
	Type  string    `json:"type"`
	Data  FileStore `json:"data"`
	Error string    `json:"error,omitempty"`
}

// -----------------------------------------------------------------------------

// Client holds channel name & ws connection
type Client struct {
	Channel string
	ws      *websocket.Conn
}

// Send sends line to client
func (cl Client) Send(line string) error {
	reply := message{Type: "log", Channel: cl.Channel, Data: line}
	if err := websocket.JSON.Send(cl.ws, reply); err != nil {
		// TODO: print if not "write: broken pipe" error - lg.Println("info: Can't send:", err)
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------

// LoadLogs loads file list
func (srv *Server) LoadLogs() (*FileStore, error) {
	files := FileStore{}
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.ListUpdated.Add(time.Duration(srv.Config.ListCache) * time.Second).After(time.Now()) {
		return srv.List, nil
	}
	srv.Log.Print("debug: Load file list")
	dir := strings.TrimSuffix(srv.Root, "/")
	err := filepath.Walk(srv.Root, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			p := strings.TrimPrefix(path, dir+"/")
			// srv.Log.Printf("debug: found logfile %s", p)
			files[p] = &FileAttr{MTime: f.ModTime(), Size: f.Size()}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	srv.ListUpdated = time.Now()
	srv.List = &files
	return srv.List, nil
}

// -----------------------------------------------------------------------------

// Init inits server struct
func (srv *Server) Init() {
	srv.lock = &sync.RWMutex{}
}

// -----------------------------------------------------------------------------

// Handler process client interaction
func (srv Server) Handler(ws *websocket.Conn) {
	tails := make(map[string]*Client)
	for {
		var err error
		var m message
		var list *FileStore
		// receive a message using the codec
		if err = websocket.JSON.Receive(ws, &m); err != nil {
			if err != io.EOF {
				srv.Log.Println("info: read:", err)
			}
			srv.Log.Println("info: readEOF:", err)
			break
		}
		srv.Log.Printf("debug: Received message type %s for channel (%s)", m.Type, m.Channel)

		var reply interface{}

		// load or refresh list
		if m.Type == "list" || (m.Type == "attach" || m.Type == "detach") {
			if l, err := srv.LoadLogs(); err != nil {
				reply = message{Type: "error", Data: err.Error()}
				srv.send(ws, reply)
				continue
			} else {
				list = l
			}
		}

		switch m.Type {
		case "host": // hostname
			reply = message{Type: "host", Data: srv.Config.Host}
		case "ping": // ping
			reply = message{Type: "pong"}
		case "list":
			reply = listmessage{Type: "list", Data: *list}

		case "attach":

			if _, ok := (*list)[m.Channel]; !ok {
				reply = message{Type: "error", Channel: m.Channel, Error: "Unknown channel"}
			} else if _, ok := tails[m.Channel]; ok {
				reply = message{Type: "error", Channel: m.Channel, Error: "Already attached"}
			} else {

				cl := &Client{Channel: m.Channel, ws: ws}
				if err = srv.Manager.Attach(m.Channel, cl); err != nil {
					reply = message{Type: "error", Channel: m.Channel, Error: err.Error()}
				} else {
					reply = message{Type: "attach", Channel: m.Channel}
					tails[m.Channel] = cl
				}
			}
		case "detach":
			if _, ok := (*list)[m.Channel]; !ok {
				reply = message{Type: "error", Channel: m.Channel, Error: "Unknown channel"}
			} else if cl, ok := tails[m.Channel]; !ok {
				reply = message{Type: "error", Channel: m.Channel, Error: "Not attached"}
			} else if err = srv.Manager.Detach(m.Channel, cl); err != nil {
				reply = message{Type: "error", Channel: m.Channel, Error: err.Error()}
			} else {
				reply = message{Type: "detach", Channel: m.Channel}
				delete(tails, m.Channel)
			}
		default:
			reply = message{Type: "error", Error: "Unknown type"}
		}

		if reply != nil {
			srv.send(ws, reply)
		}
	}

	if len(tails) != 0 {
		for k, v := range tails {
			srv.Manager.Detach(k, v)
		}
	}

}

// -----------------------------------------------------------------------------

// send a response
func (srv Server) send(ws *websocket.Conn, m interface{}) {

	srv.Log.Printf("debug: Sending line: %+v", m)
	if err := websocket.JSON.Send(ws, m); err != nil {
		// TODO: print if not "write: broken pipe" error - lg.Println("info: Can't send:", err)
	}
}

// -----------------------------------------------------------------------------

// StatsHandler processes stats request
func (srv Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	data := srv.Manager.Stats()

	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
