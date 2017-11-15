// Package manager opens tails and manages them to clients
package manager

import (
	"os"
	"path"
	"sync"

	"github.com/hpcloud/tail"

	"github.com/LeKovr/go-base/log"
)

// -----------------------------------------------------------------------------

// Config defines local application flags
type Config struct {
	Root        string `long:"root"  default:"log/"  description:"Root directory for log files"`
	Bytes       int64  `long:"bytes" default:"5000"  description:"tail from the last Nth location"`
	Lines       int    `long:"lines" default:"100"   description:"keep N old lines for new consumers"`
	MaxLineSize int    `long:"split" default:"180"   description:"min line size for split"`
	Poll        bool   `long:"poll"  description:"use polling, instead of inotify"`
	Keep        bool   `long:"keep"  description:"keep watches when all file clients leave"`
}

// Sender holds consumer
type Sender interface {
	Send(line string) error
}

// Tail holds file tail attributes
type Tail struct {

	// Register requests from the clients.
	register chan Sender

	// Unregister requests from clients.
	unregister chan Sender
}

// Manager struct holds tail manager attributes
type Manager struct {
	Config         Config
	Log            log.Logger
	lock           *sync.RWMutex
	Producers      map[string]*Tail
	ConsumerCounts map[string]uint64
}

// New creates tail manager
func New(logger log.Logger, cfg Config) (*Manager, error) {
	return &Manager{
		Config:         cfg,
		Log:            logger,
		lock:           &sync.RWMutex{},
		Producers:      make(map[string]*Tail),
		ConsumerCounts: make(map[string]uint64),
	}, nil
}

// Attach creates tail worker if none and registers new tail Consumer
func (tm *Manager) Attach(filename string, s Sender) (err error) {
	// зарегать канал
	tm.lock.RLock()
	t, ok := tm.Producers[filename]
	tm.lock.RUnlock()
	// create producer if not exists
	if !ok {
		t, err = tm.newProducer(filename)
		if err != nil {
			return
		}
	}

	tm.lock.Lock()
	defer tm.lock.Unlock()
	if !ok {
		tm.Producers[filename] = t
	}
	t.register <- s
	tm.ConsumerCounts[filename]++
	return
}

// Detach deregisters tail Consumer
func (tm *Manager) Detach(filename string, s Sender) error {
	tm.lock.RLock()
	t, ok := tm.Producers[filename]
	tm.lock.RUnlock()

	tm.lock.Lock()
	defer tm.lock.Unlock()
	if ok {
		t.unregister <- s
		tm.ConsumerCounts[filename]--
	}
	return nil
}

// Stats returns Consumers count per tailed file
func (tm *Manager) Stats() map[string]uint64 {
	tm.lock.RLock()
	defer tm.lock.RUnlock()
	return tm.ConsumerCounts
}

// newProducer method starts the run loop for the worker, listening for a quit channel in
// case we need to stop it
func (tm Manager) newProducer(key string) (*Tail, error) {
	tf, lineIncomlete, err := tm.newTail(key)
	if err != nil {
		return nil, err
	}

	t := Tail{
		register:   make(chan Sender),
		unregister: make(chan Sender),
	}

	go func() {
		tm.Log.Printf("debug: Worker %s started", key)
		buf := []string{}
		clients := make(map[Sender]bool)

		for {
			select {
			case line := <-tf.Lines:
				// we have received a text line
				if lineIncomlete {
					// 1st line after offset might be incomplete - so skip it
					lineIncomlete = false
					continue
				}
				if len(buf) == tm.Config.Lines {
					buf = buf[1:]
				}
				buf = append(buf, line.Text)
				for client := range clients {
					if err := client.Send(line.Text); err != nil {
						// TODO: print if not "write: broken pipe" error - lg.Println("info: Can't send:", err)
					}
				}

			case client := <-t.register:
				for _, item := range buf {
					if err := client.Send(item); err != nil {
						// TODO: print if not "write: broken pipe" error - lg.Println("info: Can't send:", err)
					}
				}
				clients[client] = true
				tm.Log.Printf("debug: Worker %s attached", key)
			case client := <-t.unregister:
				if _, ok := clients[client]; ok {
					delete(clients, client)
					tm.Log.Printf("debug: Worker %s detached", key)
				}
				// TODO: close worker if !tm.Cfg.Keep && !tm.ConsumersCount[id]
			}
		}
	}()
	return &t, nil
}

// newTail creates new tailfile channel
func (tm Manager) newTail(name string) (*tail.Tail, bool, error) {
	config := tail.Config{
		Follow: true,
		ReOpen: true,
	}
	filename := path.Join(tm.Config.Root, name)
	cfg := tm.Config
	config.MaxLineSize = cfg.MaxLineSize
	config.Poll = cfg.Poll
	lineIncomlete := false

	if cfg.Bytes != 0 {
		fi, err := os.Stat(filename)
		if err != nil {
			return nil, false, err
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
		return nil, false, err
	}
	return t, lineIncomlete, nil
}
