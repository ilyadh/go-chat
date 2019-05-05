package messaging

import (
	"context"
	"log"

	"github.com/gorilla/websocket"
)

// Service is a concurrent messaging service
type Service struct {
	clients    map[*client]bool
	register   chan *client
	unregister chan *client
	messageBus chan *message
}

// NewService creates new messaging services
func NewService() *Service {
	return &Service{
		clients:    make(map[*client]bool, 10),
		register:   make(chan *client, 5),
		unregister: make(chan *client, 5),
		messageBus: make(chan *message, 5),
	}
}

// Run runs the messaging service
func (s *Service) Run(ctx context.Context) {
	log.Print("[DEBUG] Messaging service start")

	for {
		select {
		case client := <-s.register:
			log.Printf("[DEBUG] register client")
			s.clients[client] = true
		case client := <-s.unregister:
			delete(s.clients, client)
		case message := <-s.messageBus:
			log.Printf("[DEBUG] Sending message %s to %d clients", string(message.msg), len(s.clients))
			for client := range s.clients {
				if client != message.client {
					client.send(message.msg)
				}
			}
		case <-ctx.Done():
			for client := range s.clients {
				client.close()
			}
			return
		}
	}
}

// RegisterAndServe creates a Client and registers it
func (s *Service) RegisterAndServe(conn *websocket.Conn) {
	client := newClient(s, conn)

	s.register <- client

	client.serve()
}

func (s *Service) broadcast(msg []byte, client *client) {
	log.Printf("[DEBUG] broadcast before %s", string(msg))
	s.messageBus <- &message{msg: msg, client: client}
	log.Printf("[DEBUG] broadcast after %s", string(msg))
}

func (s *Service) remove(client *client) {
	s.unregister <- client
}
