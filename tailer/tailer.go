package tailer

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeKovr/go-base/log"
	"github.com/dc0d/dirwatch"
	"github.com/hpcloud/tail"

	"github.com/LeKovr/webtail/worker"
)

// Config defines local application flags
type Config struct {
	Root        string `long:"root"  default:"log/"  description:"Root directory for log files"`
	Bytes       int64  `long:"bytes" default:"5000"  description:"tail from the last Nth location"`
	Lines       int    `long:"lines" default:"100"   description:"keep N old lines for new consumers"`
	MaxLineSize int    `long:"split" default:"180"   description:"split line if longer"`
	Poll        bool   `long:"poll"  description:"use polling, instead of inotify"`
	Trace       bool   `long:"trace" description:"trace worker channels"`
	ListCache   int    `long:"cache"       default:"2"       description:"Time to cache file listing (sec)"`
}

type tailer struct {
	// Store for last Config.Lines lines
	Buffer [][]byte

	// Unregister requests from clients.
	Unregister chan bool

	// Skip 1st line when read file not from start
	Incomplete bool
}

// WorkerHub holds Worker hub operations
type WorkerHub struct {
	Log     log.Logger
	Config  Config
	workers map[string]*tailer
	index   worker.IndexStore
}

// New creates tailer
func New(logger log.Logger, cfg Config) (*WorkerHub, error) {
	_, err := os.Stat(cfg.Root)
	if err != nil {
		return nil, err
	}
	return &WorkerHub{
		Config: cfg,
		Log:    logger,

		workers: make(map[string]*tailer),
		index:   make(worker.IndexStore),
	}, nil
}

// LoadIndex fills index map on program start
func (wh *WorkerHub) LoadIndex(out chan *worker.Index) {
	go wh.indexLoad(out, time.Time{})
}

// WorkerExists checks if worker already registered
func (wh *WorkerHub) WorkerExists(channel string) bool {
	_, ok := wh.workers[channel]
	return ok
}

// ChannelExists checks if channel allowed to attach
func (wh *WorkerHub) ChannelExists(channel string) bool {
	if channel == "" {
		return true
	}
	_, ok := wh.index[channel]
	return ok
}

// SetTrace turns on/off logging of incoming workers messages
func (wh *WorkerHub) SetTrace(on bool) {
	wh.Log.Printf("warn: tracing set to %t", on)
	wh.Config.Trace = on
}

// TraceEnabled returns trace state
func (wh *WorkerHub) TraceEnabled() bool {
	return wh.Config.Trace
}

// WorkerRun runs worker
func (wh *WorkerHub) WorkerRun(channel string, out chan *worker.Message) error {

	config := tail.Config{
		Follow: true,
		ReOpen: true,
	}
	cfg := wh.Config
	filename := path.Join(cfg.Root, channel)
	config.MaxLineSize = cfg.MaxLineSize
	config.Poll = cfg.Poll
	lineIncomlete := false

	if cfg.Bytes != 0 {
		fi, err := os.Stat(filename)
		if err != nil {
			return err
		}
		// get the file size
		size := fi.Size()
		if size > cfg.Bytes {
			config.Location = &tail.SeekInfo{Offset: -cfg.Bytes, Whence: os.SEEK_END}
			lineIncomlete = true
		}
	}
	t, err := tail.TailFile(filename, config)
	if err != nil {
		return err
	}

	unregister := make(chan bool)
	wh.workers[channel] = &tailer{Buffer: [][]byte{}, Unregister: unregister, Incomplete: lineIncomlete}
	go wh.worker(t, channel, out, unregister)

	return nil
}

// IndexRun runs indexer
func (wh *WorkerHub) IndexRun(out chan *worker.Index) error {
	unregister := make(chan bool)
	wh.workers[""] = &tailer{Unregister: unregister}
	go wh.indexRun(out, unregister)
	return nil
}

// WorkerStop stops worker or indexer
func (wh *WorkerHub) WorkerStop(channel string) error {
	w := wh.workers[channel]
	w.Unregister <- true
	delete(wh.workers, channel)
	return nil
}

// Buffer returns worker buffer
func (wh *WorkerHub) Buffer(channel string) [][]byte {
	return wh.workers[channel].Buffer
}

// Append adds a line into worker buffer
func (wh *WorkerHub) Append(channel string, data []byte) bool {
	if wh.workers[channel].Incomplete {
		wh.workers[channel].Incomplete = false
		return false
	}
	buf := wh.workers[channel].Buffer

	if len(buf) == wh.Config.Lines {
		buf = buf[1:]
	}
	wh.workers[channel].Buffer = append(buf, data)
	return true
}

// Index returns index items
func (wh *WorkerHub) Index() *worker.IndexStore {
	return &wh.index
}

// Update updates item in index
func (wh *WorkerHub) Update(msg *worker.Index) {
	if _, ok := wh.index[msg.Name]; ok && msg.Deleted {
		delete(wh.index, msg.Name)
		return
	}
	wh.index[msg.Name] = &worker.IndexItem{ModTime: msg.ModTime, Size: msg.Size}
}

func (wh *WorkerHub) worker(tf *tail.Tail, channel string, out chan *worker.Message, unregister chan bool) {

	wh.Log.Printf("debug: worker for channel %s started", channel)
	for {
		select {
		case line := <-tf.Lines:
			//wh.Log.Printf("debug: got line for channel %s", channel)

			out <- &worker.Message{Channel: channel, Data: line.Text}
		case <-unregister:
			err := tf.Stop() //Cleanup()
			if err != nil {
				wh.Log.Printf("warn: worker for channel %s stopped with error %v", channel, err)
			} else {
				wh.Log.Printf("debug: worker for channel %s stopped", channel)
			}
			return
		}
	}
}

func (wh *WorkerHub) indexRun(out chan *worker.Index, unregister chan bool) {
	wh.indexLoad(out, time.Time{})
	wh.Log.Print("debug: indexer started")

	notify := func(ev dirwatch.Event) {
		wh.Log.Printf("debug: handling file event %v", ev)
		wh.indexUpdateFile(out, ev.Name)
	}

	watcher := dirwatch.New(dirwatch.Notify(notify), dirwatch.Logger(wh.Log.Println))
	defer watcher.Stop()

	watcher.Add(wh.Config.Root, true)

	<-unregister
	wh.Log.Print("debug: indexer stopped")
	return
}

func (wh *WorkerHub) indexLoad(out chan *worker.Index, lastmod time.Time) {

	dir := strings.TrimSuffix(wh.Config.Root, "/")
	err := filepath.Walk(wh.Config.Root, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if f.ModTime().After(lastmod) {
				p := strings.TrimPrefix(path, dir+"/")
				out <- &worker.Index{Name: p, ModTime: f.ModTime(), Size: f.Size()}
			}
		}
		return nil
	})
	if err != nil {
		wh.Log.Printf("error: path walk %+v", err)
	}
}

func (wh *WorkerHub) indexUpdateFile(out chan *worker.Index, path string) {
	dir := strings.TrimSuffix(wh.Config.Root, "/")
	p := strings.TrimPrefix(path, dir+"/")

	f, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if _, ok := wh.index[p]; ok {
				wh.Log.Printf("debug: deleting file %s from index", p)
				out <- &worker.Index{Name: p, Deleted: true}
			}
		} else {
			wh.Log.Printf("error: cannot get stat for file %s with error %v", path, err)
		}
	}

	if !f.IsDir() {
		out <- &worker.Index{Name: p, ModTime: f.ModTime(), Size: f.Size()}
	}
}
