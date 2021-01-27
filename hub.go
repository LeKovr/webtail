package webtail

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

// InMessage holds incoming client request
type InMessage struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
}

// TailMessage holds outgoing file tail row
type TailMessage struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
	Data    string `json:"data,omitempty"`
}

// TraceMessage holds outgoing trace state
type TraceMessage struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// StatsMessage holds outgoing app stats
type StatsMessage struct {
	Type string            `json:"type"`
	Data map[string]uint64 `json:"data,omitempty"`
}

// IndexItemEvent holds messages from indexer
type IndexItemEvent struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
	Name    string    `json:"name"`
	Deleted bool      `json:"deleted,omitempty"`
}

// IndexMessage holds outgoing message item for file index
type IndexMessage struct {
	Type  string         `json:"type"`
	Data  IndexItemEvent `json:"data"`
	Error string         `json:"error,omitempty"`
}

// TailerMessage holds message from tailers
type TailerMessage struct {
	Channel string
	Data    string // []byte
}

// Message holds received message and sender
type Message struct {
	Client  *Client
	Message []byte
}

// subscribers holds clients subscribed on channel
type subscribers map[*Client]bool

// codebeat:disable[TOO_MANY_IVARS]

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	log logr.Logger

	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan *Message

	// Inbound messages from the tailers.
	receive chan *TailerMessage

	// Inbound messages from the channel indexer.
	index chan *IndexItemEvent

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Quit channel
	quit chan struct{}

	// Tail Service workers
	workers *TailService

	// Channel subscribers
	subscribers map[string]subscribers

	// Channel subscriber counts
	stats map[string]uint64

	// wg used by Close for wh.WorkerStop ending
	wg *sync.WaitGroup
}

// codebeat:enable[TOO_MANY_IVARS]

// NewHub creates hub for client services
func NewHub(logger logr.Logger, ts *TailService, wg *sync.WaitGroup) *Hub {
	return &Hub{
		log:         logger,
		broadcast:   make(chan *Message),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		receive:     make(chan *TailerMessage),
		index:       make(chan *IndexItemEvent),
		quit:        make(chan struct{}),
		workers:     ts,
		wg:          wg,
		subscribers: make(map[string]subscribers),
		stats:       make(map[string]uint64),
	}
}

// Run processes hub messages
func (h *Hub) Run() {
	h.subscribers[""] = make(subscribers)
	h.workers.IndexerRun(h.index, h.wg)
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.unsubscribeAll(client)
			}
		case cmessage := <-h.broadcast:
			// client sends attach/detach/?list
			h.fromClient(cmessage)
		case wmessage := <-h.receive:
			// tailer sends file line
			h.fromTailer(wmessage)
		case imessage := <-h.index:
			// worker sends index update
			h.fromIndexer(imessage)
		case <-h.quit:
			h.workers.WorkerStop("")
			return
		}
	}
}

// Close closes message processing
func (h *Hub) Close() {
	h.quit <- struct{}{}
}

func (h *Hub) fromClient(msg *Message) {
	var data []byte
	in := InMessage{}
	err := json.Unmarshal(msg.Message, &in)
	if err != nil {
		data, _ = json.Marshal(TailMessage{Type: "error", Data: "parse error"})
		h.send(msg.Client, data)
		return
	}
	h.log.Info("Received from Client", "message", in)
	switch in.Type {
	case "attach":
		data = h.attach(in.Channel, msg.Client)
	case "detach":
		// check subscription
		if !h.workers.WorkerExists(in.Channel) {
			// unknown producer
			data, _ = json.Marshal(TailMessage{Type: "error", Data: "unknown channel", Channel: in.Channel})
		} else if _, ok := h.subscribers[in.Channel][msg.Client]; !ok {
			// no subscriber
			data, _ = json.Marshal(TailMessage{Type: "error", Data: "not subscribed", Channel: in.Channel})
		} else {
			h.unsubscribe(in.Channel, msg.Client)
			data, _ = json.Marshal(TailMessage{Type: "detach", Channel: in.Channel})
		}
	case "stats":
		// send index counters
		data, _ = json.Marshal(StatsMessage{Type: "stats", Data: h.stats})
	case "trace":
		// on/off tracing
		h.workers.SetTrace(in.Channel)
		data, _ = json.Marshal(TraceMessage{Type: "trace", Enabled: h.workers.TraceEnabled()})
	}
	if len(data) > 0 {
		h.send(msg.Client, data)
	}
}

// fromTailer processes message from worker
func (h *Hub) fromTailer(msg *TailerMessage) {
	if h.workers.TraceEnabled() {
		h.log.Info("Trace from tailer", "channel", msg.Channel, "data", msg.Data)
	}
	data, _ := json.Marshal(TailMessage{Type: "log", Data: msg.Data})
	if !h.workers.TailerAppend(msg.Channel, data) {
		h.log.Info("Incomplete line skipped")
		return
	}
	clients := h.subscribers[msg.Channel]
	for client := range clients {
		h.send(client, data)
	}
}

// process message from indexer
func (h *Hub) fromIndexer(msg *IndexItemEvent) {
	if h.workers.TraceEnabled() {
		h.log.Info("Trace from indexer", "message", msg)
	}
	data, _ := json.Marshal(IndexMessage{Type: "index", Data: *msg})
	h.workers.IndexUpdate(msg)
	clients := h.subscribers[""]
	for client := range clients {
		h.send(client, data)
	}
}

func (h *Hub) attach(channel string, client *Client) (data []byte) {
	var err error
	if !h.workers.ChannelExists(channel) {
		data, _ = json.Marshal(TailMessage{Type: "error", Data: "unknown channel", Channel: channel})
		return
	}
	if !h.workers.WorkerExists(channel) {
		readyChan := make(chan struct{})
		// no producer => create
		err = h.workers.TailerRun(channel, h.receive, readyChan, h.wg)
		if err != nil {
			h.log.Error(err, "Worker create error")
			data, _ = json.Marshal(TailMessage{Type: "error", Data: "worker create error"})
			return
		}
		h.subscribers[channel] = make(subscribers)
		<-readyChan
	} else if _, ok := h.subscribers[channel][client]; ok {
		data, _ = json.Marshal(TailMessage{Type: "error", Data: "attached already", Channel: channel})
		return
	}
	// Confirm attach
	// not via data because have to be first in response
	datac, _ := json.Marshal(TailMessage{Type: "attach", Channel: channel})
	if h.send(client, datac) {
		if h.sendReply(channel, client) {
			// subscribe client
			h.subscribers[channel][client] = true
			h.stats[channel]++
		}
	}
	// 'data' added for linter which says:
	// "naked return in func `attach` with 36 lines of code (nakedret)""
	return data
}

func (h *Hub) sendReply(ch string, cl *Client) bool {
	if ch != "" {
		// send actual buffer
		for _, item := range h.workers.TailerBuffer(ch) {
			if !h.send(cl, item) {
				return false
			}
		}
		return true
	}
	// send channel index
	istore := h.workers.Index()
	// To store the keys in slice in sorted order
	keys := make([]string, len(*istore))
	i := 0
	for k := range *istore {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	for _, v := range keys {
		idx := &IndexMessage{
			Type: "index",
			Data: IndexItemEvent{
				Name:    v,
				ModTime: (*istore)[v].ModTime,
				Size:    (*istore)[v].Size,
			},
		}
		data, _ := json.Marshal(idx)
		if !h.send(cl, data) {
			return false
		}
	}
	return true
}

func (h *Hub) send(client *Client, data []byte) bool {
	h.log.Info("Send reply", "message", string(data))
	select {
	case client.send <- data:
	default:
		h.unsubscribeAll(client)
		return false
	}
	return true
}

// unsubscribeAll removes all client subscriptions
func (h *Hub) unsubscribeAll(client *Client) {
	for k := range h.subscribers {
		if _, ok := h.subscribers[k][client]; ok {
			h.log.Info("Remove subscriber from channel", "channel", k)
			h.unsubscribe(k, client)
		}
	}
	close(client.send)
	delete(h.clients, client)
}

func (h *Hub) unsubscribe(k string, client *Client) {
	delete(h.subscribers[k], client)
	h.stats[k]--
	if k != "" && h.stats[k] == 0 {
		// tailer has no subscribers => stop it
		h.workers.WorkerStop(k)
	}
}
