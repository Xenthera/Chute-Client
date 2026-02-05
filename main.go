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

type peerEndpoint struct {
	IP   string
	Port int
}

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

	if err := registerWithServer(*serverAddr, clientID, resolvedPort); err != nil {
		log.Fatalf("registration failed: %v", err)
	}
	log.Println("registered with rendezvous server")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go serveUDP(ctx, conn, clientID)
	go handleSignals(*serverAddr, clientID, cancel)

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

func serveUDP(ctx context.Context, conn *net.UDPConn, clientID string) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			log.Printf("udp read failed: %v", err)
			continue
		}

		message := strings.TrimSpace(string(buf[:n]))
		log.Printf("udp received client_id=%s remote=%s message=%s", clientID, remoteAddr.String(), message)

		if _, err := conn.WriteToUDP([]byte(message), remoteAddr); err != nil {
			log.Printf("udp echo failed: %v", err)
		} else {
			log.Printf("udp echo sent client_id=%s remote=%s", clientID, remoteAddr.String())
		}
	}
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
		case strings.HasPrefix(line, "udp "):
			targetID, message, ok := parseUDPCommand(line)
			if !ok {
				fmt.Println("usage: udp <id> <message>")
				continue
			}
			if err := sendMessage(ctx, serverAddr, clientID, targetID, []byte(message)); err != nil {
				log.Printf("udp failed client_id=%s target=%s err=%v", clientID, targetID, err)
				continue
			}
			log.Printf("udp ok client_id=%s target=%s", clientID, targetID)
		default:
			printHelp()
		}
	}
}

func printHelp() {
	fmt.Println("commands:")
	fmt.Println("  connect <id>")
	fmt.Println("  udp <id> <message>")
	fmt.Println("  exit")
}

func registerWithServer(serverAddr, clientID string, port int) error {
	payload := registerRequest{
		ID:   clientID,
		Port: port,
	}
	log.Printf("registering client_id=%s udp_port=%d", clientID, port)
	return postJSON(serverAddr, "/register", payload, nil, http.StatusOK)
}

func connectToPeer(ctx context.Context, serverAddr, selfID, targetID string) error {
	message := fmt.Sprintf("hello from %s\n", selfID)
	return sendMessage(ctx, serverAddr, selfID, targetID, []byte(message))
}

func sendMessage(ctx context.Context, serverAddr, selfID, targetID string, payload []byte) error {
	peer, err := lookupPeer(serverAddr, targetID)
	if err != nil {
		return err
	}

	return sendMessageToPeer(ctx, selfID, targetID, peer, payload)
}

func sendMessageToPeer(ctx context.Context, selfID, targetID string, peer peerEndpoint, payload []byte) error {
	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(peer.IP, strconv.Itoa(peer.Port)))
	if err != nil {
		return fmt.Errorf("resolve udp addr failed: %w", err)
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", remoteAddr.String())
	if err != nil {
		return fmt.Errorf("udp dial %s failed: %w", remoteAddr.String(), err)
	}
	defer conn.Close()

	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("udp send failed: %w", err)
	}
	log.Printf("udp sent client_id=%s target=%s remote=%s bytes=%d", selfID, targetID, remoteAddr.String(), len(payload))

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("udp read failed: %w", err)
	}

	echo := strings.TrimSpace(string(buf[:n]))
	log.Printf("udp echo received client_id=%s target=%s message=%s", selfID, targetID, echo)
	return nil
}

func lookupPeer(serverAddr, targetID string) (peerEndpoint, error) {
	payload := lookupRequest{ID: targetID}
	var peer lookupResponse
	if err := postJSON(serverAddr, "/lookup", payload, &peer, http.StatusOK); err != nil {
		return peerEndpoint{}, err
	}
	endpoint := peerEndpoint{
		IP:   peer.PublicIP,
		Port: peer.PublicPort,
	}
	log.Printf("lookup ok target=%s udp_endpoint=%s:%d", targetID, endpoint.IP, endpoint.Port)
	return endpoint, nil
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

func handleSignals(serverAddr, clientID string, cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	unregisterAndExit(serverAddr, clientID)
	cancel()
	os.Exit(0)
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

func parseUDPCommand(line string) (string, string, bool) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return "", "", false
	}
	id := strings.TrimSpace(parts[1])
	message := strings.TrimSpace(parts[2])
	if id == "" || message == "" {
		return "", "", false
	}
	return id, message, true
}
