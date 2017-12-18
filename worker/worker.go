/*
Package worker defines worker interface.

*/
package worker

import (
	"time"
)

// Worker is an interface which allows mocks
type WorkerHub interface {
	WorkerExists(channel string) bool
	WorkerRun(channel string, out chan *Message) error
	IndexRun(out chan *Index) error
	WorkerStop(channel string) error
	Buffer(channel string) [][]byte
	Append(channel string, data []byte) bool
	Index() *IndexStore
	Update(msg *Index)
}

type Message struct {
	Channel string
	Data    string //[]byte
}

// FileAttr holds File Attrs
type IndexItem struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
}
type Index struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
	//	IndexItem
	Name string `json:"name"`
}

// FileStore holds all log files attrs
type IndexStore map[string]*IndexItem
