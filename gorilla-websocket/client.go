package main

import (
	"bytes"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

const (
	// peer 로 메세지 쓰는데 허용된 시간
	writeWait = 10 * time.Second
	// peer 로부터 다음 메세지를 읽는데 허용된 시간
	pongWait = 60 * time.Second
	// 이 간격으로 peer 에 ping 을 보낸다. pongWait 보다 작아야 함.
	pingPeriod = (pongWait * 9) / 10
	// peer 로부터 허용된 최대 메세지 사이즈
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
}

// Client 는 websocket 연결과 hub 사이의 중간 관리자이다.
type Client struct {
	hub *Hub
	conn *websocket.Conn
	// 나가는 메세지의 버퍼된 채널
	send chan []byte
}

// serveWs 는 peer 로부터 온 웹소켓 요청을 처리한다.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil) // Upgrade 는 HTTP 서버 연결을 웹소켓 프로토콜로 업그레이드한다.
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// 새 고루틴에서 모든 작업을 수행하여 호출자가 참조하는 메모리 수집을 허용한다.
	go client.writePump()
	go client.readPump()
}

// writePump 가 hub 에서 웹소켓으로 메세지를 펌프한다.
// 각 연결에 대해 writePump 를 실행하는 고루틴이 시작된다.
// app 은 이 고루틴에서 모든 작성을 실행함으로써 연결에 대해 최대 하나의 writer 가 있는 것을 보장한다.
func (c *Client)writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <- c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub 가 채널을 닫는다.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 대기 중인 대화 메세지를 현재 웹소켓 메세지에 추가한다.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}
			if err := w.Close(); err != nil {
				return
			}

		case <- ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}


// readPump 는 웹소켓에서 hub 로 메세지를 펌프한다.
// app 은 연결 당 고루틴에서 readPump 을 실행한다.
// app 은 이 고루틴에서 모든 읽음을 실행함으로써 연결에 대한 최대 하나의 reader 가 있는 것을 보장한다.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.hub.broadcast <- message
	}
}