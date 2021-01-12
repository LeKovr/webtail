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
	} else {

		h.log.Printf("debug: Received from Client: (%+v)", in)

		switch in.Type {
		case "attach":
			if !h.wh.ChannelExists(in.Channel) {
				// проверить что путь зарегистрирован
				data, _ = json.Marshal(messageOut{Type: "error", Data: "unknown channel", Channel: in.Channel})
				break
			} else if !h.wh.WorkerExists(in.Channel) {
				// если нет продюсера - создать горутину
				if in.Channel == "" {
					err = h.wh.IndexRun(h.index)
				} else {
					err = h.wh.WorkerRun(in.Channel, h.receive)
				}
				if err != nil {
					h.log.Printf("warn: worker create error: %+v", err)
					data, _ = json.Marshal(messageOut{Type: "error", Data: "worker create error"})
					break
				}
				h.subscribers[in.Channel] = make(subscribers)
			} else if _, ok := h.subscribers[in.Channel][msg.Client]; ok {
				// клиент уже подписан - ответить "уже подписан" и выйти
				data, _ = json.Marshal(messageOut{Type: "error", Data: "attached already", Channel: in.Channel})
				break
			}

			// Confirm attach
			// not via data because have to be first in response
			datac, _ := json.Marshal(messageOut{Type: "attach", Channel: in.Channel})
			if !h.send(msg.Client, datac) {
				break
			}

			var aborted bool
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
					if !h.send(msg.Client, data) {
						aborted = true
						break
					}
				}

			} else {
				// отправить клиенту текущий буфер
				for _, item := range h.wh.Buffer(in.Channel) {
					if !h.send(msg.Client, item) {
						aborted = true
						break
					}
				}

			}
			if !aborted {
				// добавить клиента в подписчики
				h.subscribers[in.Channel][msg.Client] = true
				h.stats[in.Channel]++
			}

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

				delete(h.subscribers[in.Channel], msg.Client)
				h.stats[in.Channel]--
				if h.stats[in.Channel] == 0 {
					// если подписчиков не осталось - отправить true в unregister продюсера и  удалить из массива
					h.wh.WorkerStop(in.Channel)
				}
			}

		case "stats":
			// вернуть массив счетчиков подписок на каналы
			data, _ = json.Marshal(messageStats{Type: "stats", Data: h.stats})

		case "trace":
			// включить/выключить трассировку
			h.wh.SetTrace(in.Channel == "on")

		}
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

			delete(h.subscribers[k], client)
			h.stats[k]--
			if h.stats[k] == 0 {
				// если подписчиков не осталось - отправить true в unregister продюсера
				h.wh.WorkerStop(k)
			}
		}
	}
	close(client.send)
	delete(h.clients, client)

}
