package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/websocket"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"

	"github.com/LeKovr/go-base/log"
	"github.com/LeKovr/webtail/tailman"
)

// -----------------------------------------------------------------------------

// Flags defines local application flags
type Flags struct {
	Addr    string `long:"http_addr"   default:":8080"          description:"Http listen address"`
	Host    string `long:"host"        default:""               description:"Hostname for page title"`
	Version bool   `long:"version"     description:"Show version and exit"`
}

// Config holds all config vars
type Config struct {
	Flags
	Tail tailman.Config `group:"Tail Options"`
	Log  LogConfig      `group:"Logging Options"`
}

// -----------------------------------------------------------------------------

// Server holds service objects
type Server struct {
	Config  Config
	Log     log.Logger
	TailMan *tailman.TailMan
}

// -----------------------------------------------------------------------------

// FileAttr holds File Attrs
type FileAttr struct {
	MTime time.Time `json:"mtime"`
	Size  int64     `json:"size"`
}

// FileStore holds all log files attrs
type FileStore map[string]*FileAttr

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

type Client struct {
	Channel string
	ws      *websocket.Conn
}

func (cl Client) Send(line string) error {
	reply := message{Type: "log", Channel: cl.Channel, Data: line}
	if err := websocket.JSON.Send(cl.ws, reply); err != nil {
		// TODO: print if not "write: broken pipe" error - lg.Println("info: Can't send:", err)
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------

func (srv Server) loadLogs() (files FileStore, err error) {
	files = FileStore{}
	dir := strings.TrimSuffix(srv.Config.Tail.Root, "/")
	err = filepath.Walk(srv.Config.Tail.Root, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			p := strings.TrimPrefix(path, dir+"/")
			srv.Log.Printf("debug: found logfile %s", p)
			files[p] = &FileAttr{MTime: f.ModTime(), Size: f.Size()}
		}
		return nil
	})
	return
}

// -----------------------------------------------------------------------------

func (srv Server) handler(ws *websocket.Conn) {
	var list *FileStore
	tails := make(map[string]*Client)
	for {
		var err error
		var m message
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
		if m.Type == "list" || list == nil && (m.Type == "attach" || m.Type == "detach") {

			if l, err := srv.loadLogs(); err != nil {
				reply = message{Type: "error", Data: err.Error()}
				srv.send(ws, reply)
				continue
			} else {
				list = &l
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
				if err = srv.TailMan.Attach(m.Channel, cl); err != nil {
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
			} else if err = srv.TailMan.Detach(m.Channel, cl); err != nil {
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
			srv.TailMan.Detach(k, v)
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

func (srv Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	data := srv.TailMan.Stats()

	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// -----------------------------------------------------------------------------

func main() {

	cfg, lg := setUp()
	lg.Printf("info: %s v %s. WebTail, tail logfiles via web", path.Base(os.Args[0]), Version)
	lg.Print("info: Copyright (C) 2016, Alexey Kovrizhkin <ak@elfire.ru>")

	_, err := os.Stat(cfg.Tail.Root)
	panicIfError(lg, err, "Logfile root dir")

	tm, err := tailman.New(lg, cfg.Tail)
	panicIfError(lg, err, "Create tail manager")

	srv := Server{
		Config:  *cfg,
		Log:     lg,
		TailMan: tm,
	}

	logs, err := srv.loadLogs()
	panicIfError(lg, err, "Load logfile list")
	lg.Printf("info: Logfiles root %s contains %d item(s)", cfg.Tail.Root, len(logs))

	http.Handle("/tail", websocket.Handler(srv.handler))
	http.HandleFunc("/stats", srv.statsHandler)
	http.Handle("/", http.FileServer(assetFS()))

	lg.Printf("info: Listen at http://%s", cfg.Addr)
	err = http.ListenAndServe(cfg.Addr, nil)
	panicIfError(lg, err, "Listen")
}

// -----------------------------------------------------------------------------

func setUp() (cfg *Config, lg log.Logger) {
	cfg = &Config{}
	p := flags.NewParser(cfg, flags.Default)

	_, err := p.Parse()
	if err != nil {
		os.Exit(1) // error message written already
	}
	if cfg.Version {
		// show version & exit
		fmt.Printf("%s\n%s\n%s", Version, Build, Commit)
		os.Exit(0)
	}

	// use all CPU cores for maximum performance
	runtime.GOMAXPROCS(runtime.NumCPU())

	lg, err = NewLog(cfg.Log)
	panicIfError(nil, err, "Parse loglevel")
	return
}

// -----------------------------------------------------------------------------

func panicIfError(lg log.Logger, err error, msg string) {
	if err != nil {
		if lg != nil {
			lg.Printf("error: %s error: %s", msg, err.Error())
		} else {
			fmt.Printf("error: %s error: %s", msg, err.Error())
		}
		os.Exit(1)
	}
}
