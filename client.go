// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright (c) 2017 Alexey Kovrizhkin <lekovr+webtail@gmail.com>
// Minor changes

package webtail

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	log logr.Logger
}

const (
	newline = "\n"
	space   = " "
)

// runReadPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) runReadPump(wg *sync.WaitGroup) {
	wg.Add(1)
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
		wg.Done()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		c.log.Error(err, "SetReadDeadline")
		return
	}

	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.log.Error(err, "UnexpectedCloseError")
			}
			return
		}
		message = bytes.TrimSpace(bytes.ReplaceAll(message, []byte(newline), []byte(space)))
		c.hub.broadcast <- &Message{Client: c, Message: message}
	}
}

// runWritePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) runWritePump(wg *sync.WaitGroup) {
	wg.Add(1)
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		defer wg.Done()
	}()
	for {
		select {
		case message, ok := <-c.send:
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				c.log.Error(err, "SetWriteDeadline")
				return
			}
			if ok {
				c.sendMesage(message)
				continue
			}
			// The hub closed the channel. Send Bye and exit
			err = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			if err != nil && err != websocket.ErrCloseSent {
				c.log.Error(err, "Close socket")
			}
			return
		case <-ticker.C:
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				err = c.conn.WriteMessage(websocket.PingMessage, nil)
			}
			if err != nil {
				return
			}
		}
	}
}

func (c *Client) sendMesage(message []byte) {
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		c.log.Error(err, "NextWriter")
		return
	}
	_, err = w.Write(message)
	if err != nil {
		c.log.Error(err, "Write")
		return
	}
	// Add queued chat messages to the current websocket message.
	n := len(c.send)
	for i := 0; i < n; i++ {
		_, err = w.Write([]byte(newline))
		if err == nil {
			_, err = w.Write(<-c.send)
		}
		if err != nil {
			return
		}
	}
	if err := w.Close(); err != nil {
		return
	}
}

func upgrader(readBufferSize, writeBufferSize int) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
}
