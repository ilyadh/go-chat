package chat

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
)

// WSClient is a WebSocket implementation of Client
type WSClient struct {
	id uuid.UUID

	conn *websocket.Conn

	send chan []byte

	handleReceive  func(Client, []byte)
	handleShutdown func(Client)

	shutdown chan struct{}
	ok       chan struct{}
}

// NewWSClient returns a new WSClient to be used to send and receive messages from websocket.Conn
func NewWSClient(conn *websocket.Conn) *WSClient {
	return &WSClient{
		id:             uuid.New(),
		conn:           conn,
		send:           make(chan []byte),
		handleReceive:  func(Client, []byte) {},
		handleShutdown: func(Client) {},

		shutdown: make(chan struct{}),
		ok:       make(chan struct{}),
	}
}

// ID returns unique identifier of Client
func (c *WSClient) ID() string {
	return c.id.String()
}

// Serve starts serving the lifecycle of a WebSocket connection
func (c *WSClient) Serve() {
	errChan := make(chan error, 2)

	go func() { errChan <- c.writeMessages() }()
	go func() { errChan <- c.readMessages() }()

	done := false
	for {
		select {
		case err := <-errChan:
			log.Printf("[WARN] [%s] %v", c.id, err)
			if done {
				// Cleanup already done
				return
			}

			go func() {
				// Notify the Service so it stops sending messages to us and we can close the channel
				c.handleShutdown(c)
				close(c.send)
			}()
			// Drain send channel
			for range c.send {
			}
			c.conn.Close()
			done = true
		case <-c.shutdown:
			// Perform graceful shutdown
			if done {
				// Write or read failed; no need to continue
				close(c.ok)
				return
			}

			err := c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, ""),
				time.Now().Add(writeWait),
			)
			if err != nil {
				c.conn.Close()
			}

			close(c.send)

			for i := 0; i < cap(errChan); i++ {
				// Wait for all goroutines to shutdown
				err = <-errChan
				log.Printf("[WARN] [%s] %v", c.id, err)
			}

			c.conn.Close()
			close(c.ok)
			return
		}
	}
}

// Send sends a text message to the WebSocket connection
func (c *WSClient) Send(message []byte) {
	c.send <- message
}

// SetReceiveHandler sets callback to be called when client reads a text message from the connection
func (c *WSClient) SetReceiveHandler(handler func(Client, []byte)) {
	c.handleReceive = handler
}

// SetShutdownHandler sets callback to be called after client performs a shutdown
func (c *WSClient) SetShutdownHandler(handler func(Client)) {
	c.handleShutdown = handler
}

// Shutdown writes close message to WebSocket connection, which should trigger graceful shutdown.
// As soon as shutdown's complete, ShutdownHandler will be called
func (c *WSClient) Shutdown(ctx context.Context) error {
	close(c.shutdown)

	select {
	case <-ctx.Done():
		c.conn.Close()
		return errors.New("Failed to shutdown gracefully")
	case <-c.ok:
		return nil
	}
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

	newline byte = '\n'
)

func (c *WSClient) writeMessages() error {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return errors.New("Send closed")
			}

			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return err
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return err
			}

			for i := 0; i < len(c.send); i++ {
				message = append(message, newline)
				message = append(message, <-c.send...)
			}

			if _, err = w.Write(message); err != nil {
				return err
			}
			if err = w.Close(); err != nil {
				return err
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return err
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return err
			}

			log.Printf("[DEBUG] [%s] PING", c.id)
		}
	}
}

func (c *WSClient) readMessages() error {
	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}
	c.conn.SetPongHandler(func(string) error {
		log.Printf("[DEBUG] [%s] PONG", c.id)
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			return err
		}
		if messageType != websocket.TextMessage {
			log.Printf("[WARN] [%s] Skip message type %d", c.id, messageType)
			continue
		}

		c.handleReceive(c, message)
	}
}
