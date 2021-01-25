package webtail

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeKovr/go-base/log"
	"github.com/dc0d/dirwatch"
	"github.com/nxadm/tail"
)

type tailer struct {
	// Store for last Config.Lines lines
	Buffer [][]byte

	// Unregister requests from clients.
	Unregister chan bool

	// Skip 1st line when read file not from start
	Incomplete bool
}

// IndexItemAttr holds File (index item) Attrs
type IndexItemAttr struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
}

// IndexItemAttrStore holds all index items
type IndexItemAttrStore map[string]*IndexItemAttr

// TailHub holds Worker hub operations
type TailHub struct {
	Log     log.Logger
	Config  *Config
	workers map[string]*tailer
	index   IndexItemAttrStore
}

// NewTailHub creates tailer hub
func NewTailHub(logger log.Logger, cfg *Config) (*TailHub, error) {
	_, err := os.Stat(cfg.Root)
	if err != nil {
		return nil, err
	}
	aPath, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	if aPath != cfg.Root {
		cfg.Root = aPath
	}
	return &TailHub{
		Config: cfg,
		Log:    logger,

		workers: make(map[string]*tailer),
		index:   make(IndexItemAttrStore),
	}, nil
}

// LoadIndex fills index map on program start
func (wh *TailHub) LoadIndex(out chan *IndexItemEvent) {
	go wh.indexLoad(out, time.Time{})
}

// WorkerExists checks if worker already registered
func (wh *TailHub) WorkerExists(channel string) bool {
	_, ok := wh.workers[channel]
	return ok
}

// ChannelExists checks if channel allowed to attach
func (wh *TailHub) ChannelExists(channel string) bool {
	if channel == "" {
		return true
	}
	_, ok := wh.index[channel]
	return ok
}

// SetTrace turns on/off logging of incoming workers messages
func (wh *TailHub) SetTrace(on bool) {
	wh.Log.Printf("warn: tracing set to %t", on)
	wh.Config.Trace = on
}

// TraceEnabled returns trace state
func (wh *TailHub) TraceEnabled() bool {
	return wh.Config.Trace
}

// TailRun runs tail worker
func (wh *TailHub) TailRun(channel string, out chan *WorkerMessage) error {
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
func (wh *TailHub) IndexRun(out chan *IndexItemEvent) error {
	unregister := make(chan bool)
	wh.workers[""] = &tailer{Unregister: unregister}
	go wh.indexRun(out, unregister)
	return nil
}

// WorkerStop stops worker or indexer
func (wh *TailHub) WorkerStop(channel string) error {
	w := wh.workers[channel]
	w.Unregister <- true
	delete(wh.workers, channel)
	return nil
}

// Buffer returns worker buffer
func (wh *TailHub) Buffer(channel string) [][]byte {
	return wh.workers[channel].Buffer
}

// Append adds a line into worker buffer
func (wh *TailHub) Append(channel string, data []byte) bool {
	if wh.workers[channel].Incomplete {
		wh.workers[channel].Incomplete = false
		return false
	}
	buf := wh.workers[channel].Buffer

	if len(buf) == wh.Config.Lines {
		// drop oldest line if buffer is full
		buf = buf[1:]
	}
	buf = append(buf, data)
	wh.workers[channel].Buffer = buf
	return true
}

// Index returns index items
func (wh *TailHub) Index() *IndexItemAttrStore {
	return &wh.index
}

// Update updates item in index
func (wh *TailHub) Update(msg *IndexItemEvent) {
	if msg.Deleted {
		if _, ok := wh.index[msg.Name]; ok {
			wh.Log.Printf("debug: deleting file %s from index", msg.Name)
			delete(wh.index, msg.Name)
		}
		return
	}
	wh.index[msg.Name] = &IndexItemAttr{ModTime: msg.ModTime, Size: msg.Size}
}

func (wh *TailHub) worker(tf *tail.Tail, channel string, out chan *WorkerMessage, unregister chan bool) {
	wh.Log.Printf("debug: worker for channel %s started", channel)
	for {
		select {
		case line := <-tf.Lines:
			out <- &WorkerMessage{Channel: channel, Data: line.Text}
		case <-unregister:
			err := tf.Stop() // Cleanup()
			if err != nil {
				wh.Log.Printf("warn: worker for channel %s stopped with error %v", channel, err)
			} else {
				wh.Log.Printf("debug: worker for channel %s stopped", channel)
			}
			return
		}
	}
}

func (wh *TailHub) indexRun(out chan *IndexItemEvent, unregister chan bool) {
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
}

func (wh *TailHub) indexLoad(out chan *IndexItemEvent, lastmod time.Time) {
	wh.Log.Print("debug: ============== indexLoad start")

	dir := strings.TrimSuffix(wh.Config.Root, "/")
	err := filepath.Walk(wh.Config.Root, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if f.ModTime().After(lastmod) {
				p := strings.TrimPrefix(path, dir+"/")
				out <- &IndexItemEvent{Name: p, ModTime: f.ModTime(), Size: f.Size()}
			}
		}
		return nil
	})
	if err != nil {
		wh.Log.Printf("error: path walk %+v", err)
	}
	wh.Log.Print("debug: ============== indexLoad stop")
}

func (wh *TailHub) indexUpdateFile(out chan *IndexItemEvent, filePath string) {
	dir := strings.TrimSuffix(wh.Config.Root, "/")
	p := strings.TrimPrefix(filePath, dir+"/")

	f, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			out <- &IndexItemEvent{Name: p, Deleted: true}
		} else {
			wh.Log.Printf("error: cannot get stat for file %s with error %v", filePath, err)
		}
	}

	if !f.IsDir() {
		out <- &IndexItemEvent{Name: p, ModTime: f.ModTime(), Size: f.Size()}
	}
}
