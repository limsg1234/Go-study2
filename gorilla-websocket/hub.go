package main

// Hub 는 활성화된 clients 의 집합을 유지하고 clients 에게 메세지를 알린다.
type Hub struct {
	// 등록된 clients
	clients map[*Client]bool
	// clients 로부터 들어오는 메세지
	broadcast chan []byte
	// clients 의 요청을 등록한다.
	register chan *Client
	// clients 의 요청 등록을 취소한다.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub {
		clients: make(map[*Client]bool),
		broadcast: make(chan []byte),
		register: make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <- h.register:
			h.clients[client] = true
		case client := <- h.unregister:
			if _, ok := h.clients[client]; ok { // 기존 요청 동록이 있으면
				delete(h.clients, client) // 삭제
				close(client.send) // 채널 닫음
			}
		case message := <- h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
