package webtail

import (
	"encoding/json"
	"sort"

	"github.com/LeKovr/webtail/worker"
)

type messageIn struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
}
type messageOut struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
	Data    string `json:"data,omitempty"`
}
type messageStats struct {
	Type string            `json:"type"`
	Data map[string]uint64 `json:"data,omitempty"`
}
type messageIndex struct {
	Type  string       `json:"type"`
	Data  worker.Index `json:"data"`
	Error string       `json:"error,omitempty"`
}

func (h *Hub) fromClient(msg *Message) {
	var data []byte
	in := messageIn{}
	err := json.Unmarshal(msg.Message, &in)
	if err != nil {
		data, _ = json.Marshal(messageOut{Type: "error", Data: "parse error"})
		h.send(msg.Client, data)
		return
	}
	h.log.Printf("debug: Received from Client: (%+v)", in)
	switch in.Type {
	case "attach":
		h.attach(in.Channel, msg.Client, data)
	case "detach":
		// проверить, что клиент подписан
		if !h.wh.WorkerExists(in.Channel) {
			// unknown producer
			data, _ = json.Marshal(messageOut{Type: "error", Data: "unknown channel", Channel: in.Channel})
		} else if _, ok := h.subscribers[in.Channel][msg.Client]; !ok {
			// no subscriber
			data, _ = json.Marshal(messageOut{Type: "error", Data: "not subscribed", Channel: in.Channel})
		} else {
			// удалить подписку
			data, _ = json.Marshal(messageOut{Type: "detach", Channel: in.Channel})
			h.unsubscribe(in.Channel, msg.Client)
		}

	case "stats":
		// вернуть массив счетчиков подписок на каналы
		data, _ = json.Marshal(messageStats{Type: "stats", Data: h.stats})

	case "trace":
		// включить/выключить трассировку
		h.wh.SetTrace(in.Channel == "on")
	}
	if len(data) > 0 {
		h.send(msg.Client, data)
	}
}

// process message from worker
func (h *Hub) fromWorker(msg *worker.Message) bool {

	if h.wh.TraceEnabled() {
		h.log.Printf("debug: Trace from Worker: (%+v)", msg)
	}

	data, _ := json.Marshal(messageOut{Type: "log", Data: msg.Data})

	if !h.wh.Append(msg.Channel, data) {
		return true
	}

	clients := h.subscribers[msg.Channel]

	for client := range clients {
		h.send(client, data)
	}
	return true
}

// process message from worker
func (h *Hub) fromIndexer(msg *worker.Index) {

	if h.wh.TraceEnabled() {
		h.log.Printf("debug: Trace from Indexer: (%+v)", msg)
	}

	data, _ := json.Marshal(messageIndex{Type: "index", Data: *msg})

	h.wh.Update(msg)

	clients := h.subscribers[""]

	for client := range clients {
		h.send(client, data)
	}
}

func (h *Hub) attach(channel string, client *Client, data []byte) {
	var err error
	if !h.wh.ChannelExists(channel) {
		// проверить что путь зарегистрирован
		data, _ = json.Marshal(messageOut{Type: "error", Data: "unknown channel", Channel: channel})
		return
	} else if !h.wh.WorkerExists(channel) {
		// если нет продюсера - создать горутину
		if channel == "" {
			err = h.wh.IndexRun(h.index)
		} else {
			err = h.wh.WorkerRun(channel, h.receive)
		}
		if err != nil {
			h.log.Printf("warn: worker create error: %+v", err)
			data, _ = json.Marshal(messageOut{Type: "error", Data: "worker create error"})
			return
		}
		h.subscribers[channel] = make(subscribers)
	} else if _, ok := h.subscribers[channel][client]; ok {
		// клиент уже подписан - ответить "уже подписан" и выйти
		data, _ = json.Marshal(messageOut{Type: "error", Data: "attached already", Channel: channel})
		return
	}
	// Confirm attach
	// not via data because have to be first in response
	datac, _ := json.Marshal(messageOut{Type: "attach", Channel: channel})
	if !h.send(client, datac) {
		return
	}
	ok := h.sendReply(channel, client)
	if ok {
		// добавить клиента в подписчики
		h.subscribers[channel][client] = true
		h.stats[channel]++
	}
	return
}

func (h *Hub) sendReply(ch string, cl *Client) bool {

	if ch == "" {
		// отправить клиенту список каналов
		istore := h.wh.Index()
		// To store the keys in slice in sorted order
		var keys []string
		for k := range *istore {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, v := range keys {
			idx := &messageIndex{
				Type: "index",
				Data: worker.Index{
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

	} else {
		// отправить клиенту текущий буфер
		for _, item := range h.wh.Buffer(ch) {
			if !h.send(cl, item) {
				return false
			}
		}
	}
	return true
}

func (h *Hub) send(client *Client, data []byte) bool {

	h.log.Printf("debug: Send reply: %v", string(data))
	select {
	case client.send <- data:
	default:
		h.remove(client)
		return false
	}
	return true
}

// remove all client subscriptions
func (h *Hub) remove(client *Client) {
	for k := range h.subscribers {
		if _, ok := h.subscribers[k][client]; ok {
			h.log.Printf("debug: Remove subscriber from channel (%s)", k)
			h.unsubscribe(k, client)
		}
	}
	close(client.send)
	delete(h.clients, client)

}

func (h *Hub) unsubscribe(k string, client *Client) {
	delete(h.subscribers[k], client)
	h.stats[k]--
	if h.stats[k] == 0 {
		// если подписчиков не осталось - отправить true в unregister продюсера
		err := h.wh.WorkerStop(k)
		if err != nil {
			h.log.Printf("warn: worker stop error: %+v", err)
		}
	}
}
