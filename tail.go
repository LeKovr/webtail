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

	// Quit worker process
	Quit chan struct{}

	// Skip 1st line when read file not from start
	IsHeadTrimmed bool
}

// TailService holds Worker hub operations
type TailService struct {
	log     logr.Logger
	Config  *Config
	workers map[string]*TailAttr
	index   IndexItemAttrStore
}

// tailWorker holds tailer run arguments
type tailWorker struct {
	out     chan *TailerMessage
	quit    chan struct{}
	log     logr.Logger
	tf      *tail.Tail
	channel string
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

// WorkerStop stops worker (tailer or indexer)
func (ts *TailService) WorkerStop(channel string) {
	w := ts.workers[channel]
	w.Quit <- struct{}{}
	delete(ts.workers, channel)
}

// TailerBuffer returns worker buffer
func (ts *TailService) TailerBuffer(channel string) [][]byte {
	return ts.workers[channel].Buffer
}

// TailerAppend adds a line into worker buffer
func (ts *TailService) TailerAppend(channel string, data []byte) bool {
	if ts.workers[channel].IsHeadTrimmed {
		// Skip first trimmed (partial) line
		ts.workers[channel].IsHeadTrimmed = false
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
	headTrimmed := false

	if cfg.Bytes != 0 {
		fi, err := os.Stat(filename)
		if err != nil {
			return err
		}
		// get the file size
		size := fi.Size()
		if size > cfg.Bytes {
			config.Location = &tail.SeekInfo{Offset: -cfg.Bytes, Whence: os.SEEK_END}
			headTrimmed = true
		}
	}
	t, err := tail.TailFile(filename, config)
	if err != nil {
		return err
	}
	quit := make(chan struct{})
	ts.workers[channel] = &TailAttr{Buffer: [][]byte{}, Quit: quit, IsHeadTrimmed: headTrimmed}
	go tailWorker{
		tf:      t,
		channel: channel,
		out:     out,
		quit:    quit,
		log:     ts.log,
	}.run(readyChan, wg)
	return nil
}

// run runs tail worker
func (tw tailWorker) run(readyChan chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer func() {
		tw.log.Info("tailworker close")
		wg.Done()
	}()
	log := tw.log.WithValues("channel", tw.channel)
	log.Info("Tailer started")
	readyChan <- struct{}{}
	for {
		select {
		case line := <-tw.tf.Lines:
			tw.out <- &TailerMessage{Channel: tw.channel, Data: line.Text}
		case <-tw.quit:
			err := tw.tf.Stop() // Cleanup()
			if err != nil {
				log.Error(err, "Tailer stopped with error")
			} else {
				log.Info("Tailer stopped")
			}
			return
		}
	}
}
