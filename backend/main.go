package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	serverAddr := flag.String("server", "chute-rendezvous-server.fly.dev", "rendezvous server address (host:port)")
	uiAddr := flag.String("ui", "127.0.0.1:8787", "ui api address (host:port)")
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
	go checkRendezvousHealth(*serverAddr, manager)
	if err := startUIServer(ctx, *uiAddr, client, manager, *serverAddr, clientID); err != nil {
		log.Printf("ui server failed: %v", err)
	}

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

func checkRendezvousHealth(serverAddr string, manager *ConnectionManager) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + serverAddr + "/health")
	if err != nil {
		manager.SetRendezvousHealth(false)
		log.Printf("rendezvous health failed: %v", err)
		return
	}
	defer resp.Body.Close()
	manager.SetRendezvousHealth(resp.StatusCode == http.StatusOK)
}
