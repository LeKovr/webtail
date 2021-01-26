package webtail

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"strings"

	"github.com/dc0d/dirwatch"
	//	"github.com/go-logr/logr"
)

// IndexItemAttr holds File (index item) Attrs
type IndexItemAttr struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
}

// IndexItemAttrStore holds all index items
type IndexItemAttrStore map[string]*IndexItemAttr

// IndexerRun runs indexer
func (wh *TailHub) IndexerRun(out chan *IndexItemEvent, wg *sync.WaitGroup) {
	unregister := make(chan bool)
	wh.workers[""] = &tailer{Unregister: unregister}

	readyChan := make(chan struct{})
	go wh.bgIndexer(out, unregister, readyChan, wg)
	<-readyChan
	wh.indexLoad(time.Now())
	wh.log.Info("Indexer started")
}

// Index returns index items
func (wh *TailHub) Index() *IndexItemAttrStore {
	return &wh.index
}

// Update updates item in index
func (wh *TailHub) Update(msg *IndexItemEvent) {
	if !msg.Deleted {
		wh.index[msg.Name] = &IndexItemAttr{ModTime: msg.ModTime, Size: msg.Size}
		return
	}

	if _, ok := wh.index[msg.Name]; ok {
		wh.log.Info("Deleting file from index", "filename", msg.Name)
		delete(wh.index, msg.Name)
	}
}

// bgIndexer runs dirwatch
func (wh *TailHub) bgIndexer(out chan *IndexItemEvent, unregister chan bool, readyChan chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	notify := func(ev dirwatch.Event) {
		wh.log.Info("Handling file event", "event", ev)
		wh.indexUpdateFile(out, ev.Name)
	}
	logger := func(args ...interface{}) {
		// data := args[1:]
		wh.log.Info("Dirwatch log") // args[0].(string)) //, "data", &data)
	}
	watcher := dirwatch.New(dirwatch.Notify(notify), dirwatch.Logger(logger))
	defer watcher.Stop()
	watcher.Add(wh.Config.Root, true)

	readyChan <- struct{}{}
	<-unregister
	wh.log.Info("Indexer stopped")
}

func (wh *TailHub) indexLoad(lastmod time.Time) {
	dir := strings.TrimSuffix(wh.Config.Root, "/")
	err := filepath.Walk(wh.Config.Root, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if f.ModTime().Before(lastmod) {
				p := strings.TrimPrefix(path, dir+"/")
				wh.index[p] = &IndexItemAttr{ModTime: f.ModTime(), Size: f.Size()}
			}
		}
		return nil
	})
	if err != nil {
		wh.log.Error(err, "Path walk")
	}
}

func (wh *TailHub) indexUpdateFile(out chan *IndexItemEvent, filePath string) {
	dir := strings.TrimSuffix(wh.Config.Root, "/")
	p := strings.TrimPrefix(filePath, dir+"/")

	f, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			out <- &IndexItemEvent{Name: p, Deleted: true}
		} else {
			wh.log.Error(err, "Cannot get stat for file", "filepath", filePath)
		}
	}
	if !f.IsDir() {
		out <- &IndexItemEvent{Name: p, ModTime: f.ModTime(), Size: f.Size()}
	}
}
