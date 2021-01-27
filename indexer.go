package webtail

// This file holds directory tree indexer methods

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dc0d/dirwatch"
)

// IndexItemAttr holds File (index item) Attrs
type IndexItemAttr struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
}

// IndexItemAttrStore holds all index items
type IndexItemAttrStore map[string]*IndexItemAttr

// IndexerRun runs indexer
func (ts *TailService) IndexerRun(out chan *IndexItemEvent, wg *sync.WaitGroup) {
	unregister := make(chan bool)
	ts.workers[""] = &TailAttr{Unregister: unregister}
	readyChan := make(chan struct{})
	go ts.runIndexWorker(out, unregister, readyChan, wg)
	<-readyChan
	ts.indexLoad(time.Now())
	ts.log.Info("Indexer started")
}

// Index returns index items
func (ts *TailService) Index() *IndexItemAttrStore {
	return &ts.index
}

// IndexUpdate updates item in index
func (ts *TailService) IndexUpdate(msg *IndexItemEvent) {
	if !msg.Deleted {
		ts.index[msg.Name] = &IndexItemAttr{ModTime: msg.ModTime, Size: msg.Size}
		return
	}

	if _, ok := ts.index[msg.Name]; ok {
		ts.log.Info("Deleting file from index", "filename", msg.Name)
		delete(ts.index, msg.Name)
	}
}

// runIndexWorker runs dirwatch
func (ts *TailService) runIndexWorker(out chan *IndexItemEvent, unregister chan bool, readyChan chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	notify := func(ev dirwatch.Event) {
		ts.log.Info("Handling file event", "event", ev)
		ts.indexUpdateFile(out, ev.Name)
	}
	logger := func(args ...interface{}) {} // Is it called ever?
	watcher := dirwatch.New(dirwatch.Notify(notify), dirwatch.Logger(logger))
	defer watcher.Stop()
	watcher.Add(ts.Config.Root, true)
	readyChan <- struct{}{}
	<-unregister
	ts.log.Info("Indexer stopped")
}

func (ts *TailService) indexLoad(lastmod time.Time) {
	dir := strings.TrimSuffix(ts.Config.Root, "/")
	err := filepath.Walk(ts.Config.Root, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if f.ModTime().Before(lastmod) {
				p := strings.TrimPrefix(path, dir+"/")
				ts.index[p] = &IndexItemAttr{ModTime: f.ModTime(), Size: f.Size()}
			}
		}
		return nil
	})
	if err != nil {
		ts.log.Error(err, "Path walk")
	}
}

func (ts *TailService) indexUpdateFile(out chan *IndexItemEvent, filePath string) {
	dir := strings.TrimSuffix(ts.Config.Root, "/")
	p := strings.TrimPrefix(filePath, dir+"/")

	f, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			out <- &IndexItemEvent{Name: p, Deleted: true}
		} else {
			ts.log.Error(err, "Cannot get stat for file", "filepath", filePath)
		}
	}
	if !f.IsDir() {
		out <- &IndexItemEvent{Name: p, ModTime: f.ModTime(), Size: f.Size()}
	}
}
