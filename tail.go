package webtail

// This file holds directory file tail methods

import (
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/go-logr/logr"
	"github.com/nxadm/tail"
)

// TailAttr holds tail worker attributes
type TailAttr struct {
	// Store for last Config.Lines lines
	Buffer [][]byte

	// Unregister requests from clients.
	Unregister chan bool

	// Skip 1st line when read file not from start
	Incomplete bool
}

// TailService holds Worker hub operations
type TailService struct {
	log     logr.Logger
	Config  *Config
	workers map[string]*TailAttr
	index   IndexItemAttrStore
}

// tailRun holds tailer run arguments
type tailRun struct {
	tf         *tail.Tail
	channel    string
	out        chan *TailerMessage
	unregister chan bool
	readyChan  chan struct{}
	log        logr.Logger
}

// NewTailService creates tailer service
func NewTailService(logger logr.Logger, cfg *Config) (*TailService, error) {
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
	return &TailService{
		Config:  cfg,
		log:     logger,
		workers: make(map[string]*TailAttr),
		index:   make(IndexItemAttrStore),
	}, nil
}

// WorkerExists checks if worker already registered
func (ts *TailService) WorkerExists(channel string) bool {
	_, ok := ts.workers[channel]
	return ok
}

// ChannelExists checks if channel allowed to attach
func (ts *TailService) ChannelExists(channel string) bool {
	if channel == "" {
		return true
	}
	_, ok := ts.index[channel]
	return ok
}

// SetTrace turns on/off logging of incoming workers messages
func (ts *TailService) SetTrace(mode string) {
	if mode == "on" {
		ts.Config.Trace = true
	} else if mode == "off" {
		ts.Config.Trace = false
	}
	ts.log.Info("Tracing", "trace", ts.Config.Trace)
}

// TraceEnabled returns trace state
func (ts *TailService) TraceEnabled() bool {
	return ts.Config.Trace
}

// TailerRun creates and runs tail worker
func (ts *TailService) TailerRun(channel string, out chan *TailerMessage, readyChan chan struct{}, wg *sync.WaitGroup) error {
	config := tail.Config{
		Follow: true,
		ReOpen: true,
	}
	cfg := ts.Config
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
	ts.workers[channel] = &TailAttr{Buffer: [][]byte{}, Unregister: unregister, Incomplete: lineIncomlete}
	go tailRun{
		tf:         t,
		channel:    channel,
		out:        out,
		unregister: unregister,
		readyChan:  readyChan,
		log:        ts.log,
	}.run(wg)
	return nil
}

// WorkerStop stops worker or indexer
func (ts *TailService) WorkerStop(channel string) {
	w := ts.workers[channel]
	w.Unregister <- true
	delete(ts.workers, channel)
}

// TailerBuffer returns worker buffer
func (ts *TailService) TailerBuffer(channel string) [][]byte {
	return ts.workers[channel].Buffer
}

// TailerAppend adds a line into worker buffer
func (ts *TailService) TailerAppend(channel string, data []byte) bool {
	if ts.workers[channel].Incomplete {
		ts.workers[channel].Incomplete = false
		return false
	}
	buf := ts.workers[channel].Buffer

	if len(buf) == ts.Config.Lines {
		// drop oldest line if buffer is full
		buf = buf[1:]
	}
	buf = append(buf, data)
	ts.workers[channel].Buffer = buf
	return true
}

func (tailer tailRun) run(wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	log := tailer.log.WithValues("channel", tailer.channel)
	log.Info("Tailer started")
	tailer.readyChan <- struct{}{}
	for {
		select {
		case line := <-tailer.tf.Lines:
			tailer.out <- &TailerMessage{Channel: tailer.channel, Data: line.Text}
		case <-tailer.unregister:
			err := tailer.tf.Stop() // Cleanup()
			if err != nil {
				log.Error(err, "Tailer stopped with error")
			} else {
				log.Info("Tailer stopped")
			}
			return
		}
	}
}
