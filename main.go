package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type registerRequest struct {
	ID   string `json:"id"`
	Port int    `json:"port"`
}

type lookupRequest struct {
	ID string `json:"id"`
}

type unregisterRequest struct {
	ID string `json:"id"`
}

type lookupResponse struct {
	ID         string `json:"id"`
	PublicIP   string `json:"public_ip"`
	PublicPort int    `json:"public_port"`
}

func main() {
	serverAddr := flag.String("server", "localhost:8080", "rendezvous server address (host:port)")
	listenPort := flag.Int("port", 0, "listening port (0 = auto)")
	flag.Parse()

	clientID, err := generateClientID()
	if err != nil {
		panic(err)
	}

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(*listenPort))
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	resolvedPort := listener.Addr().(*net.TCPAddr).Port

	fmt.Println("chute client starting")
	fmt.Printf("client id: %s\n", clientID)
	fmt.Printf("server: %s\n", *serverAddr)
	fmt.Printf("listen port: %d\n", resolvedPort)

	if err := registerWithServer(*serverAddr, clientID, resolvedPort); err != nil {
		log.Fatalf("registration failed: %v", err)
	}
	log.Println("registered with rendezvous server")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go acceptConnections(ctx, listener, clientID)
	go handleSignals(cancel)

	runCLI(ctx, cancel, clientID, *serverAddr)
}

func generateClientID() (string, error) {
	const digits = 8
	const maxDigit = 10

	var result [digits]byte
	for i := 0; i < digits; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(maxDigit))
		if err != nil {
			return "", err
		}
		result[i] = byte('0' + n.Int64())
	}

	return string(result[:]), nil
}

func acceptConnections(ctx context.Context, listener net.Listener, clientID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept failed: %v", err)
			continue
		}
		go handleConn(conn, clientID)
	}
}

func handleConn(conn net.Conn, clientID string) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("incoming connection client_id=%s remote=%s", clientID, remoteAddr)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("read failed: %v", err)
		return
	}

	message := strings.TrimSpace(line)
	log.Printf("received message client_id=%s remote=%s message=%s", clientID, remoteAddr, message)
}

func runCLI(ctx context.Context, cancel context.CancelFunc, clientID, serverAddr string) {
	scanner := bufio.NewScanner(os.Stdin)
	printHelp()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case line == "exit":
			unregisterAndExit(serverAddr, clientID)
			cancel()
			return
		case strings.HasPrefix(line, "connect "):
			id, ok := parseConnectID(line)
			if !ok {
				fmt.Println("usage: connect <id>")
				continue
			}
			if err := connectToPeer(ctx, serverAddr, clientID, id); err != nil {
				log.Printf("connect failed client_id=%s target=%s err=%v", clientID, id, err)
				continue
			}
			log.Printf("connect ok client_id=%s target=%s", clientID, id)
		default:
			printHelp()
		}
	}
}

func printHelp() {
	fmt.Println("commands:")
	fmt.Println("  connect <id>")
	fmt.Println("  exit")
}

func registerWithServer(serverAddr, clientID string, port int) error {
	payload := registerRequest{
		ID:   clientID,
		Port: port,
	}
	return postJSON(serverAddr, "/register", payload, nil, http.StatusOK)
}

func connectToPeer(ctx context.Context, serverAddr, selfID, targetID string) error {
	peer, err := lookupPeer(serverAddr, targetID)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(peer.PublicIP, strconv.Itoa(peer.PublicPort))
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("dial %s failed: %w", address, err)
	}
	defer conn.Close()

	message := fmt.Sprintf("hello from %s\n", selfID)
	if _, err := conn.Write([]byte(message)); err != nil {
		return fmt.Errorf("send failed: %w", err)
	}
	return nil
}

func lookupPeer(serverAddr, targetID string) (lookupResponse, error) {
	payload := lookupRequest{ID: targetID}
	var peer lookupResponse
	if err := postJSON(serverAddr, "/lookup", payload, &peer, http.StatusOK); err != nil {
		return lookupResponse{}, err
	}
	return peer, nil
}

func unregisterAndExit(serverAddr, clientID string) {
	if err := unregisterWithServer(serverAddr, clientID); err != nil {
		log.Printf("unregister failed: %v", err)
	}
}

func unregisterWithServer(serverAddr, clientID string) error {
	payload := unregisterRequest{ID: clientID}
	return postJSON(serverAddr, "/unregister", payload, nil, http.StatusOK, http.StatusNotFound)
}

func handleSignals(cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	cancel()
}

func postJSON(serverAddr, path string, payload any, response any, okStatuses ...int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := "http://" + serverAddr + path
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, status := range okStatuses {
		if resp.StatusCode == status {
			if response != nil {
				if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
					return err
				}
			}
			return nil
		}
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}

func parseConnectID(line string) (string, bool) {
	id := strings.TrimSpace(strings.TrimPrefix(line, "connect "))
	if id == "" {
		return "", false
	}
	return id, true
}
