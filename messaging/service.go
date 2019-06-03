package messaging

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Client represents a messaging Service client
type Client interface {
	ID() string

	Serve()
	Send([]byte)

	SetReceiveHandler(func(Client, []byte))
	SetShutdownHandler(func(Client))

	Shutdown(context.Context) error
}

// Service manages Clients
type Service struct {
	clients    map[Client]bool
	register   chan Client
	unregister chan Client
	broadcast  chan *payload
}

// NewService creates a new messaging Service
func NewService() *Service {
	return &Service{
		clients:    make(map[Client]bool),
		register:   make(chan Client),
		unregister: make(chan Client),
		broadcast:  make(chan *payload),
	}
}

// Run starts the messaging service
func (s *Service) Run(ctx context.Context) {
	for {
		select {
		case c := <-s.register:
			s.clients[c] = true
			log.Printf("[INFO] Register Client: %s", c.ID())
		case c := <-s.unregister:
			delete(s.clients, c)
			log.Printf("[INFO] Unregister Client: %s", c.ID())
		case payload := <-s.broadcast:
			for c := range s.clients {
				if c != payload.client {
					c.Send(payload.message)
				}
			}
		case <-ctx.Done():
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			done := make(chan error, len(s.clients))

			for c := range s.clients {
				go func(c Client) { done <- wrap(c.Shutdown(ctx), "[%s]", c.ID()) }(c)
			}

			for i := 0; i < cap(done); i++ {
				if err := <-done; err != nil {
					log.Printf("[ERROR] %v", err)
				}
			}
			log.Printf("[WARN] Messaging service shutdown")
			return
		}
	}
}

// Register adds Client to list of Clients
func (s *Service) Register(client Client) {
	client.SetReceiveHandler(func(c Client, message []byte) {
		s.broadcast <- &payload{message: message, client: c}
	})
	client.SetShutdownHandler(func(c Client) {
		s.unregister <- c
	})

	s.register <- client
}

type payload struct {
	message []byte
	client  Client
}

func wrap(err error, format string, a ...interface{}) error {
	if err == nil {
		return err
	}

	a = append(a, err)

	return fmt.Errorf(format+" %v", a...)
}
