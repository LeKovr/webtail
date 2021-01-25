// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright (c) 2017 Alexey Kovrizhkin <lekovr+webtail@gmail.com>
// Minor changes

package webtail

import (
	"bytes"
	"log"
	"net/http"
	"time"

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
	hub *ClientHub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

const (
	newline = "\n"
	space   = " "
)

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))

	if err != nil {
		log.Printf("warn: SetReadDeadline: %v", err)
		return
	}

	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.ReplaceAll(message, []byte(newline), []byte(space)))
		c.hub.broadcast <- &Message{Client: c, Message: message}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.Printf("warn: SetWriteDeadline: %v", err)
				return
			}

			if !ok {
				// The hub closed the channel.
				err := c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					log.Printf("warn: CloseMessage: %v", err)
				}
				return
			}
			c.sendMesage(message)
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
		log.Printf("warn: NextWriter: %v", err)
		return
	}
	_, err = w.Write(message)
	if err != nil {
		log.Printf("warn: Write: %v", err)
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

func upgrader(rbs, wbs int) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  rbs,
		WriteBufferSize: wbs,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
}
