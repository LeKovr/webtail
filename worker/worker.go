/*
Package worker defines worker interface.

*/
package worker

import (
	"time"
)

// Hub is an interface which allows mocks and holds Worker hub operations
type Hub interface {
	ChannelExists(channel string) bool
	WorkerExists(channel string) bool
	WorkerRun(channel string, out chan *Message) error
	IndexRun(out chan *Index) error
	WorkerStop(channel string) error
	Buffer(channel string) [][]byte
	Append(channel string, data []byte) bool
	LoadIndex(out chan *Index)
	Index() *IndexStore
	Update(msg *Index)
	SetTrace(bool)
	TraceEnabled() bool
}

// Message holds messages from workers
type Message struct {
	Channel string
	Data    string //[]byte
}

// IndexItem holds File (index item) Attrs
type IndexItem struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
}

// Index holds messages from indexer
type Index struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
	//	IndexItem
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

// IndexStore holds all index items
type IndexStore map[string]*IndexItem
