package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ilyadh/go-chat/chat"
)

type Server struct {
	Port int
	WS   struct {
		ReadBufferSize  int
		WriteBufferSize int
	}
}

func (cmd *Server) Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		log.Printf("[WARN] Graceful shutdown")
		cancel()
	}()

	server := cmd.newServer()
	server.run(ctx)
}

func (cmd *Server) newServer() *server {
	s := &server{}

	s.chatService = chat.NewService()
	s.upgrader = &websocket.Upgrader{
		ReadBufferSize:  cmd.WS.ReadBufferSize,
		WriteBufferSize: cmd.WS.WriteBufferSize,
	}
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cmd.Port),
		Handler: s.newHandler(),
	}

	return s
}

type server struct {
	chatService *chat.Service
	upgrader    *websocket.Upgrader
	httpServer  *http.Server
}

func (s *server) run(ctx context.Context) {
	log.Printf("[INFO] Starting server on port %d", 8080)

	go func() {
		err := s.httpServer.ListenAndServe()
		log.Printf("[INFO] Server terminated, %v", err)
	}()

	s.chatService.Run(ctx)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("[INFO] Server graceful shutdown complete %v", err)
	}
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

	client := chat.NewWSClient(conn)
	s.chatService.Register(client)

	go client.Serve()
}
