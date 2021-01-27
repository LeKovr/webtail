package webtail

import (
	"os"
	"path"
	"path/filepath"

	"github.com/go-logr/logr"
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

// TailHub holds Worker hub operations
type TailHub struct {
	log     logr.Logger
	Config  *Config
	workers map[string]*tailer
	index   IndexItemAttrStore
}

// Tailer holds tailer worker data
type Tailer struct {
	tf         *tail.Tail
	channel    string
	out        chan *TailerMessage
	unregister chan bool
	readyChan  chan struct{}
	log        logr.Logger
}

// NewTailHub creates tailer hub
func NewTailHub(logger logr.Logger, cfg *Config) (*TailHub, error) {
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
		Config:  cfg,
		log:     logger,
		workers: make(map[string]*tailer),
		index:   make(IndexItemAttrStore),
	}, nil
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
func (wh *TailHub) SetTrace(mode string) {
	if mode == "on" {
		wh.Config.Trace = true
	} else if mode == "off" {
		wh.Config.Trace = false
	}
	wh.log.Info("Tracing", "trace", wh.Config.Trace)
}

// TraceEnabled returns trace state
func (wh *TailHub) TraceEnabled() bool {
	return wh.Config.Trace
}

// TailerRun creates and runs tail worker
func (wh *TailHub) TailerRun(channel string, out chan *TailerMessage, readyChan chan struct{}) error {
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
	tailer := &Tailer{
		tf:         t,
		channel:    channel,
		out:        out,
		unregister: unregister,
		readyChan:  readyChan,
		log:        wh.log,
	}
	go tailer.run()
	return nil
}

// WorkerStop stops worker or indexer
func (wh *TailHub) WorkerStop(channel string) {
	w := wh.workers[channel]
	w.Unregister <- true
	delete(wh.workers, channel)
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

func (tailer *Tailer) run() {
	tailer.log.Info("Tailer started", "channel", tailer.channel)
	tailer.readyChan <- struct{}{}
	for {
		select {
		case line := <-tailer.tf.Lines:
			tailer.out <- &TailerMessage{Channel: tailer.channel, Data: line.Text}
		case <-tailer.unregister:
			err := tailer.tf.Stop() // Cleanup()
			if err != nil {
				tailer.log.Error(err, "Tailer stopped with error", "channel", tailer.channel)
			} else {
				tailer.log.Info("Tailer stopped", "channel", tailer.channel)
			}
			return
		}
	}
}
