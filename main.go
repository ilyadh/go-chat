package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/ilyadh/go-chat/messaging"
)

type config struct{}

type server struct {
	config *config

	messageService *messaging.Service

	upgrader   *websocket.Upgrader
	httpServer *http.Server
}

func (s *server) run() {
	log.Printf("[INFO] Starting server on port %d", 8080)

	s.upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", 8080),
		Handler: s.newHandler(),
	}

	err := s.httpServer.ListenAndServe()
	log.Printf("[INFO] Server terminated, %v", err)
}

func (s *server) newHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", s.handleWS)

	return mux
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to upgrade request, %v", err)
		return
	}

	client := messaging.NewWSClient(conn)
	s.messageService.Register(client)

	go client.Serve()
}

func main() {
	var name string

	clientFlagSet := flag.NewFlagSet("client", flag.ExitOnError)
	clientFlagSet.StringVar(&name, "name", "", "name")

	switch os.Args[1] {
	case "server":
		messageService := messaging.NewService()

		go messageService.Run(context.Background())

		server := &server{
			messageService: messageService,
		}

		server.run()
	case "client":
		clientFlagSet.Parse(os.Args[2:])

		if len(name) == 0 {
			name = "client-" + strconv.Itoa(rand.Int())
		}

		u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
		log.Printf("[INFO] Connecting to %s", u.String())

		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Fatalf("[FATAL] Dialing: %v", err)
		}
		defer c.Close()

		go func() {
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					return
				}

				fmt.Println(string(message))
			}
		}()

		for {
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			text = name + ": " + text
			c.WriteMessage(websocket.TextMessage, []byte(text))
		}
	}
}
