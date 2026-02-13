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
	serverAddr := flag.String("server", "localhost:8080", "rendezvous server address (host:port)")
	flag.Parse()

	clientID, err := generateClientID()
	if err != nil {
		panic(err)
	}

	fmt.Println("chute client starting")
	fmt.Printf("client id: %s\n", clientID)
	fmt.Printf("server: %s\n", *serverAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient(clientID, *serverAddr)
	manager := NewConnectionManager(clientID, *serverAddr)
	manager.SetSessionSetter(client.SetSession)
	go handleSignals(client, cancel)
	go client.StartPolling(ctx, manager)

	runCLI(ctx, cancel, client, manager, clientID, *serverAddr)
}

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
