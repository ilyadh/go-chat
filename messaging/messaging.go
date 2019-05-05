package messaging

import (
	"bytes"
	"context"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// conn represents a client connection
type conn interface {
	close() error
}

type message struct {
	msg    []byte
	client *client
}

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

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Client represents a messaging service client
type client struct {
	service     *Service
	conn        *websocket.Conn
	ctx         context.Context
	ctxCancel   context.CancelFunc
	sendChan    chan []byte
	receiveChan chan []byte
}

func newClient(service *Service, conn *websocket.Conn) *client {
	ctx, cancel := context.WithCancel(context.Background())
	return &client{
		service:     service,
		conn:        conn,
		ctx:         ctx,
		ctxCancel:   cancel,
		sendChan:    make(chan []byte, 5),
		receiveChan: make(chan []byte, 5),
	}
}

// Serve handles incoming and outgoing messages for Client
func (c *client) serve() {
	ticker := time.NewTicker(pingPeriod)

	go c.readFromConn()

	for {
		select {
		case msg := <-c.sendChan:
			log.Printf("[DEBUG] Writing message %s to conn", string(msg))
			c.writeToConn(msg)
		case <-ticker.C:
			c.writePingToConn()
		case msg := <-c.receiveChan:
			log.Printf("[DEBUG] Message received from chan %s", string(msg))
			c.service.broadcast(msg, c)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *client) send(msg []byte) {
	c.sendChan <- msg
}

func (c *client) readFromConn() {
	defer c.close()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[ERROR] Reading from socket: %v", err)
			break
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		c.receiveChan <- message

		log.Printf("[INFO] Message received %s", string(message))
	}
}

func (c *client) writeToConn(msg []byte) {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))

	err := c.conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Printf("[ERROR] Writing to socket: %v", err)
	}

	log.Printf("[INFO] Message written %s", string(msg))
}

func (c *client) writePingToConn() {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		log.Printf("[ERROR] Writing Ping to socket: %v", err)
	}
}

func (c *client) writeCloseToConn() {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteMessage(websocket.CloseMessage, nil); err != nil {
		log.Printf("[ERROR] Writing Close to socket: %v", err)
	}
}

func (c *client) close() {
	// perform cleanup
	c.service.remove(c)
	c.writeCloseToConn()
	c.conn.Close()
	c.ctxCancel()
}
