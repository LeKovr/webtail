// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package webtail

import (
	"github.com/LeKovr/go-base/log"
	"github.com/LeKovr/webtail/worker"
)

// Message holds data received from client
type Message struct {
	Client  *Client
	Message []byte
}

// subscribers holds clients subscribed on channel
type subscribers map[*Client]bool

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	log log.Logger

	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan *Message

	// Inbound messages from the workers.
	receive chan *worker.Message

	// Inbound messages from the channel indexer.
	index chan *worker.Index

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Worker Hub
	wh worker.Hub

	// Channel subscribers
	subscribers map[string]subscribers

	// Channel subscriber counts
	stats map[string]uint64
}

func newHub(logger log.Logger, wh worker.Hub) *Hub {
	return &Hub{
		log:         logger,
		broadcast:   make(chan *Message),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		receive:     make(chan *worker.Message), // 1),
		index:       make(chan *worker.Index),
		wh:          wh,
		subscribers: make(map[string]subscribers),
		stats:       make(map[string]uint64),
	}
}

func (h *Hub) run() {
	h.wh.LoadIndex(h.index)
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.remove(client)
			}
		case cmessage := <-h.broadcast:
			// client sends attach/detach/?list
			h.fromClient(cmessage)
		case wmessage := <-h.receive:
			// worker sends file line
			h.fromWorker(wmessage)
		case imessage := <-h.index:
			// worker sends index update
			h.fromIndexer(imessage)
		}
	}
}
