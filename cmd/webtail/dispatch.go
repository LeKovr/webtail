package main

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

func (h *Hub) fromClient(msg *Message) bool {

	var data []byte

	in := messageIn{}
	err := json.Unmarshal(msg.Message, &in)
	if err != nil {
		h.log.Printf("warn: parse error: %+v", err)
		return true
	}
	//	return h.out("error")
	switch in.Type {
	case "attach":
		// проверить что путь зарегистрирован
		// если нет продюсера - создать горутину
		if !h.wh.WorkerExists(in.Channel) {
			if in.Channel == "" {
				err = h.wh.IndexRun(h.index)
			} else {
				err = h.wh.WorkerRun(in.Channel, h.receive)
			}
			if err != nil {
				h.log.Printf("warn: wcer %+v", err)
				data, _ = json.Marshal(messageOut{Type: "error", Data: "worker create error"})
				return false
			}
			h.subscribers[in.Channel] = make(subscribers)
		} else if _, ok := h.subscribers[in.Channel][msg.Client]; ok {
			// клиент уже подписан - ответить "уже подписан" и выйти
			data, _ = json.Marshal(messageOut{Type: "error", Data: "worker create error"})

		}
		if in.Channel == "" {
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
				h.send(msg.Client, data)
			}

		} else {
			// отправить клиенту текущий буфер
			for _, item := range h.wh.Buffer(in.Channel) {
				h.send(msg.Client, item)
			}

		}
		// добавить клиента в подписчики
		h.subscribers[in.Channel][msg.Client] = true
		h.stats[in.Channel]++
		return true

	case "detach":
		// проверить, что клиент подписан
		if !h.wh.WorkerExists(in.Channel) {
			// unknown producer
			data, _ = json.Marshal(messageOut{Type: "error", Data: "unknown channel"})
		} else if _, ok := h.subscribers[in.Channel][msg.Client]; !ok {
			// no subscriber
			data, _ = json.Marshal(messageOut{Type: "error", Data: "not subscribed"})
		} else {
			// удалить подписку
			h.log.Printf("debug: unsubscribe from channel (%s)", in.Channel)

			delete(h.subscribers[in.Channel], msg.Client)
			h.stats[in.Channel]--
			if h.stats[in.Channel] == 0 {
				// если подписчиков не осталось - отправить true в unregister продюсера и  удалить из массива
				h.wh.WorkerStop(in.Channel)
			}
		}

	case "stats":
		//вернуть массив счетчиков подписок на каналы
		data, _ = json.Marshal(messageStats{Type: "stats", Data: h.stats})

	}
	if len(data) > 0 {
		h.send(msg.Client, data)
	}

	return true
}

// process message from worker
func (h *Hub) fromWorker(msg *worker.Message) bool {

	data, _ := json.Marshal(messageOut{Type: "log", Data: msg.Data})
	//h.log.Printf("debug: got2 log (%v)", msg.Data)

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

	data, _ := json.Marshal(messageIndex{Type: "index", Data: *msg})

	h.wh.Update(msg)

	clients := h.subscribers[""]

	for client := range clients {
		h.send(client, data)
	}
}

func (h *Hub) send(client *Client, data []byte) {

	h.log.Printf("debug:Send reply: %v", string(data))
	select {
	case client.send <- data:
	default:
		close(client.send)
		delete(h.clients, client)
	}
}

// remove all client subscriptions
func (h *Hub) remove(client *Client) {
	for k := range h.subscribers {
		if _, ok := h.subscribers[k][client]; ok {
			delete(h.subscribers[k], client)
			h.stats[k]--
			if h.stats[k] == 0 {
				// если подписчиков не осталось - отправить true в unregister продюсера и  удалить из массива
				h.wh.WorkerStop(k)
			}
		}
	}
}
