package webtail

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

// Returned Messages
const (
	MsgSubscribed        = "success"
	MsgUnSubscribed      = "success"
	MsgUnknownChannel    = "unknown channel"
	MsgNotSubscribed     = "not subscribed"
	MsgWorkerError       = "worker create error"
	MsgSubscribedAlready = "attached already"
	MsgNone              = ""
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
	// Logger
	log logr.Logger

	// Tail Service workers
	workers *TailService

	// wg used by Close for wh.WorkerStop ending
	wg *sync.WaitGroup

	// Registered clients.
	clients map[*Client]bool

	// Channel subscribers
	subscribers map[string]subscribers

	// Channel subscriber counts
	stats map[string]uint64

	// Inbound messages from the clients.
	broadcast chan *Message

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Inbound messages from the tailers.
	receive chan *TailMessage

	// Inbound messages from the channel indexer.
	index chan *IndexItemEvent

	// Quit channel
	quit chan struct{}
}

// codebeat:enable[TOO_MANY_IVARS]

// NewHub creates hub for client services
func NewHub(logger logr.Logger, ts *TailService, wg *sync.WaitGroup) *Hub {
	return &Hub{
		log:         logger,
		workers:     ts,
		wg:          wg,
		clients:     make(map[*Client]bool),
		subscribers: make(map[string]subscribers),
		stats:       make(map[string]uint64),
		broadcast:   make(chan *Message),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		receive:     make(chan *TailMessage),
		index:       make(chan *IndexItemEvent),
		quit:        make(chan struct{}),
	}
}

// Run processes hub messages
func (h *Hub) Run() {
	h.subscribers[""] = make(subscribers)
	h.workers.IndexerRun(h.index, h.wg)
	defer h.workers.WorkerStop("")
	onAir := true
	for {
		select {
		case client := <-h.register:
			if onAir {
				h.clients[client] = true
			}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.unsubscribeClient(client, onAir)
			}
			if !onAir && len(h.clients) == 0 {
				return
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
			onAir = false
			if len(h.clients) == 0 {
				return
			}
			for client := range h.clients {
				close(client.send)
			}
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
		msgData, ok := h.subscribe(in.Channel, msg.Client)
		data = formatTailMessage(in.Channel, "attach", msgData, ok)
	case "detach":
		msgData, ok := h.unsubscribe(in.Channel, msg.Client)
		data = formatTailMessage(in.Channel, "detach", msgData, ok)
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
func (h *Hub) fromTailer(msg *TailMessage) {
	if h.workers.TraceEnabled() {
		h.log.Info("Trace from tailer", "channel", msg.Channel, "data", msg.Data, "type", msg.Type)
	}
	data, _ := json.Marshal(msg)
	if msg.Type == "log" && !h.workers.TailerAppend(msg.Channel, data) {
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

func (h *Hub) subscribe(channel string, client *Client) (string, bool) {
	var err error
	if !h.workers.ChannelExists(channel) {
		return MsgUnknownChannel, false
	}
	if !h.workers.WorkerExists(channel) {
		readyChan := make(chan struct{})
		// no producer => create
		err = h.workers.TailerRun(channel, h.receive, readyChan, h.wg)
		if err != nil {
			h.log.Error(err, "Worker create error")
			return MsgWorkerError, false
		}
		h.subscribers[channel] = make(subscribers)
		<-readyChan
	} else if _, ok := h.subscribers[channel][client]; ok {
		return MsgSubscribedAlready, false
	}
	// Confirm attach
	// not via data because have to be first in response
	if h.send(client, formatTailMessage(channel, "attach", MsgSubscribed, true)) {
		if h.sendReply(channel, client) {
			// subscribe client
			h.subscribers[channel][client] = true
			h.stats[channel]++
		}
	}
	return MsgNone, true
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
	for _, v := range h.workers.IndexKeys() {
		file := h.workers.IndexItem(v)
		idx := &IndexMessage{
			Type: "index",
			Data: IndexItemEvent{
				Name:    v,
				ModTime: file.ModTime,
				Size:    file.Size,
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
		h.unsubscribeClient(client, true)
		return false
	}
	return true
}

// unsubscribeClient removes all client subscriptions
func (h *Hub) unsubscribeClient(client *Client, needsClose bool) {
	for k := range h.subscribers {
		if _, ok := h.subscribers[k][client]; ok {
			h.log.Info("Remove subscriber from channel", "channel", k)
			h.unsubscribe(k, client)
		}
	}
	if needsClose {
		close(client.send)
	}
	delete(h.clients, client)
}

func (h *Hub) unsubscribe(channel string, client *Client) (string, bool) {
	subscribers, ok := h.subscribers[channel]
	if !ok {
		return MsgUnknownChannel, false
	}
	if _, ok = subscribers[client]; !ok {
		return MsgNotSubscribed, false
	}
	delete(h.subscribers[channel], client)
	h.stats[channel]--
	if channel != "" && h.stats[channel] == 0 {
		// tailer has no subscribers => stop it
		h.workers.WorkerStop(channel)
	}
	return MsgUnSubscribed, true
}

// formatTailMessage packs data to json
func formatTailMessage(channel, cmd, msgData string, ok bool) []byte {
	if msgData == MsgNone {
		return []byte{}
	}
	msg := TailMessage{Data: msgData, Channel: channel}
	if ok {
		msg.Type = cmd
	} else {
		msg.Type = "error"
	}
	data, _ := json.Marshal(msg)
	return data
}
