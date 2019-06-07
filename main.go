package main

import (
	"flag"
	"os"

	"github.com/ilyadh/go-chat/cmd"
)

func main() {
	switch os.Args[1] {
	case "server":
		serverCmd := &cmd.Server{
			Port: 8080,
			WS: struct {
				ReadBufferSize  int
				WriteBufferSize int
			}{1024, 1024},
		}

		serverCmd.Execute()
	case "client":
		var name string

		clientFlagSet := flag.NewFlagSet("client", flag.ExitOnError)
		clientFlagSet.StringVar(&name, "name", "", "name")

		clientFlagSet.Parse(os.Args[2:])

		clientCmd := &cmd.Client{
			URL:  "ws://localhost:8080/ws",
			Name: name,
		}

		clientCmd.Execute()
	}
}
