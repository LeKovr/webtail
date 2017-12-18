package tailer

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hpcloud/tail"

	"github.com/LeKovr/go-base/log"
	"github.com/LeKovr/webtail/worker"
)

// Config defines local application flags
type Config struct {
	Root        string `long:"root"  default:"log/"  description:"Root directory for log files"`
	Bytes       int64  `long:"bytes" default:"5000"  description:"tail from the last Nth location"`
	Lines       int    `long:"lines" default:"100"   description:"keep N old lines for new consumers"`
	MaxLineSize int    `long:"split" default:"180"   description:"min line size for split"`
	Poll        bool   `long:"poll"  description:"use polling, instead of inotify"`
	ListCache   int    `long:"cache"       default:"2"       description:"Time to cache file listing (sec)"`
}

type tailer struct {
	Buffer [][]byte

	// Unregister requests from clients.
	Unregister chan bool

	Incomplete bool
}

type WorkerHub struct {
	Log     log.Logger
	Config  Config
	workers map[string]*tailer
	index   worker.IndexStore
}

// New creates tailer
func New(logger log.Logger, cfg Config) (*WorkerHub, error) {
	return &WorkerHub{
		Config: cfg,
		Log:    logger,

		workers: make(map[string]*tailer),
		index:   make(worker.IndexStore),
	}, nil
}

func (wh *WorkerHub) WorkerExists(channel string) bool {
	_, ok := wh.workers[channel]
	return ok
}

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

func (wh *WorkerHub) IndexRun(out chan *worker.Index) error {
	unregister := make(chan bool)
	wh.workers[""] = &tailer{Unregister: unregister}
	go wh.indexRun(out, unregister)
	return nil
}

func (wh *WorkerHub) WorkerStop(channel string) error {
	w := wh.workers[channel]
	w.Unregister <- true
	delete(wh.workers, channel)
	return nil
}

func (wh *WorkerHub) Buffer(channel string) [][]byte {
	return wh.workers[channel].Buffer
}

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

func (wh *WorkerHub) Index() *worker.IndexStore {
	return &wh.index // TODO: sort?
}
func (wh *WorkerHub) Update(msg *worker.Index) {
	wh.index[msg.Name] = &worker.IndexItem{ModTime: msg.ModTime, Size: msg.Size}
}

func (wh *WorkerHub) worker(tf *tail.Tail, channel string, out chan *worker.Message, unregister chan bool) {

	for {
		select {
		case line := <-tf.Lines:
			out <- &worker.Message{Channel: channel, Data: line.Text}
		//	wh.Log.Printf("debug: got log (%s)", line.Text)
		case <-unregister:
			tf.Cleanup()
			return
		}
	}
}

func (wh *WorkerHub) indexRun(out chan *worker.Index, unregister chan bool) {

	ticker := time.NewTicker(time.Duration(wh.Config.ListCache) * time.Second)
	defer func() {
		ticker.Stop()
	}()
	last := time.Now()
	wh.indexLoad(out, time.Time{})
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			wh.indexLoad(out, last)
			last = now

		case <-unregister:
			return
		}
	}
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
