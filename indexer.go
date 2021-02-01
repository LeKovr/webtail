package webtail

// This file holds directory tree indexer methods

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dc0d/dirwatch"
	"github.com/go-logr/logr"
)

// IndexItemAttr holds File (index item) Attrs
type IndexItemAttr struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
}

// IndexItemAttrStore holds all index items
type IndexItemAttrStore map[string]*IndexItemAttr

type indexWorker struct {
	out  chan *IndexItemEvent
	quit chan struct{}
	log  logr.Logger
	root string
}

// IndexerRun runs indexer
func (ts *TailService) IndexerRun(out chan *IndexItemEvent, wg *sync.WaitGroup) {
	quit := make(chan struct{})
	ts.workers[""] = &TailAttr{Quit: quit}
	readyChan := make(chan struct{})
	go indexWorker{
		out:  out,
		quit: quit,
		log:  ts.log,
		root: ts.Config.Root,
	}.run(readyChan, wg)
	<-readyChan
	err := loadIndex(ts.index, ts.Config.Root, time.Now())
	if err != nil {
		ts.log.Error(err, "Path walk")
	}
	ts.log.V(1).Info("Indexer started")
}

// IndexKeys returns sorted index keys
func (ts *TailService) IndexKeys() []string {
	items := ts.index
	// To store the keys in slice in sorted order
	keys := make([]string, len(items))
	i := 0
	for k := range items {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

// IndexItem returns index item
func (ts *TailService) IndexItem(key string) *IndexItemAttr {
	return ts.index[key]
}

// IndexUpdate updates TailService index item
func (ts *TailService) IndexUpdate(msg *IndexItemEvent) {
	if !msg.Deleted {
		ts.index[msg.Name] = &IndexItemAttr{ModTime: msg.ModTime, Size: msg.Size}
		return
	}
	if _, ok := ts.index[msg.Name]; ok {
		ts.log.Info("Deleting path from index", "path", msg.Name)
		items := ts.index
		for k := range items {
			if strings.HasPrefix(k, msg.Name) {
				delete(ts.index, k)
			}
		}
	}
}

// run runs indexer worker
func (iw indexWorker) run(readyChan chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	defer func() {
		wg.Done()
		iw.log.V(1).Info("Indexer stopped")
	}()
	notify := func(ev dirwatch.Event) {
		iw.log.Info("Handling file event", "event", ev)
		if err := sendUpdate(iw.out, iw.root, ev.Name); err != nil {
			iw.log.Error(err, "Cannot get stat for file", "filepath", ev.Name)
		}
	}
	logger := func(args ...interface{}) {} // Is it called ever?
	watcher := dirwatch.New(dirwatch.Notify(notify), dirwatch.Logger(logger))
	defer watcher.Stop()
	watcher.Add(iw.root, true)
	readyChan <- struct{}{}
	<-iw.quit
}

// sendUpdate sends index update to out channel
func sendUpdate(out chan *IndexItemEvent, root, filePath string) error {
	dir := strings.TrimSuffix(root, "/")
	p := strings.TrimPrefix(filePath, dir+"/")

	f, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			out <- &IndexItemEvent{Name: p, Deleted: true}
		} else {
			return err
		}
	} else if !f.IsDir() {
		out <- &IndexItemEvent{Name: p, ModTime: f.ModTime(), Size: f.Size()}
	}
	return nil
}

// loadIndex loads index items for the first time
func loadIndex(index IndexItemAttrStore, root string, lastmod time.Time) error {
	dir := strings.TrimSuffix(root, "/")
	err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			if f.ModTime().Before(lastmod) {
				p := strings.TrimPrefix(path, dir+"/")
				index[p] = &IndexItemAttr{ModTime: f.ModTime(), Size: f.Size()}
			}
		}
		return nil
	})
	return err
}
