package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	serverAddr := flag.String("server", "chute-rendezvous-server.fly.dev", "rendezvous server address (host:port)")
	flag.Parse()

	// Startup
	clientID, err := generateClientID()
	if err != nil {
		panic(err)
	}

	fmt.Println("chute client starting")
	fmt.Printf("client id: %s\n", formatClientID(clientID))
	fmt.Printf("server: %s\n", *serverAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient(clientID, *serverAddr)
	manager := NewConnectionManager(clientID, *serverAddr)
	manager.SetSessionSetter(client.SetSession)
	go handleSignals(client, cancel)
	go client.StartPolling(ctx, manager)

	// GUI-first: keep backend running without the CLI loop.
	<-ctx.Done()
}

// Shutdown
func handleSignals(client *Client, cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	_ = client.Disconnect()
	cancel()
	if err := client.Unregister(); err != nil {
		log.Printf("unregister failed: %v", err)
	}
	os.Exit(0)
}
