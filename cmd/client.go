package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ilyadh/go-chat/chat"
)

type Client struct {
	URL  string
	Name string
}

func (cmd *Client) Execute() {
	conn, _, err := websocket.DefaultDialer.Dial(cmd.URL, nil)
	if err != nil {
		log.Fatalf("[FATAL] Dialing: %v", err)
	}

	client := chat.NewWSClient(conn)

	send := make(chan []byte)
	shutdown := make(chan struct{})

	client.SetReceiveHandler(func(message []byte) {
		fmt.Println(string(message))
	})
	client.SetShutdownHandler(func() {
		close(shutdown)
	})

	go client.Serve()

	go func() {
		name := cmd.Name
		if name == "" {
			name = client.ID()
		}

		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			text := scanner.Text()

			text = name + ": " + text
			send <- []byte(text)
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[WARN] [%s] %v", client.ID(), err)
		}

		close(send)
	}()

	func() {
		done := false
		for {
			select {
			case <-shutdown:
				done = true
			case message, ok := <-send:
				if done {
					return
				}
				if ok {
					client.Send(message)
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				client.Shutdown(ctx)

				return
			}
		}
	}()
}
