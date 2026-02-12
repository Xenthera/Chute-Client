package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	serverAddr := flag.String("server", "localhost:8080", "rendezvous server address (host:port)")
	listenPort := flag.Int("port", 0, "listening port (0 = auto)")
	flag.Parse()

	clientID, err := generateClientID()
	if err != nil {
		panic(err)
	}

	udpAddr := &net.UDPAddr{Port: *listenPort}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("udp listen failed: %v", err)
	}
	defer conn.Close()

	resolvedPort := conn.LocalAddr().(*net.UDPAddr).Port

	fmt.Println("chute client starting")
	fmt.Printf("client id: %s\n", clientID)
	fmt.Printf("server: %s\n", *serverAddr)
	fmt.Printf("listen port: %d\n", resolvedPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := NewChuteSession(conn, clientID)
	session.Start()
	client := NewClient(clientID, *serverAddr, session)
	if err := client.Register(resolvedPort); err != nil {
		log.Fatalf("registration failed: %v", err)
	}
	log.Println("registered with rendezvous server")

	go handleSignals(client, cancel)

	runCLI(ctx, cancel, client, clientID, *serverAddr)
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
