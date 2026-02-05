package main

import (
	"bufio"
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

	go acceptConnections(listener)

	go handleSignals(*serverAddr, clientID)
	runCLI(clientID, *serverAddr)
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

func acceptConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept failed: %v", err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	log.Printf("incoming connection from %s", conn.RemoteAddr().String())

	buf := make([]byte, 256)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("read failed: %v", err)
		return
	}

	message := strings.TrimSpace(string(buf[:n]))
	log.Printf("received message: %s", message)
}

func runCLI(clientID, serverAddr string) {
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
			return
		case strings.HasPrefix(line, "connect "):
			id := strings.TrimSpace(strings.TrimPrefix(line, "connect "))
			if id == "" {
				fmt.Println("usage: connect <id>")
				continue
			}
			if err := connectToPeer(serverAddr, clientID, id); err != nil {
				log.Printf("connect failed: %v", err)
				continue
			}
			log.Printf("connect ok: %s", id)
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
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := "http://" + serverAddr + "/register"
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func connectToPeer(serverAddr, selfID, targetID string) error {
	peer, err := lookupPeer(serverAddr, targetID)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(peer.PublicIP, strconv.Itoa(peer.PublicPort))
	conn, err := net.Dial("tcp", address)
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

type lookupResponse struct {
	ID         string `json:"id"`
	PublicIP   string `json:"public_ip"`
	PublicPort int    `json:"public_port"`
}

func lookupPeer(serverAddr, targetID string) (lookupResponse, error) {
	payload := lookupRequest{ID: targetID}
	body, err := json.Marshal(payload)
	if err != nil {
		return lookupResponse{}, err
	}

	url := "http://" + serverAddr + "/lookup"
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return lookupResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return lookupResponse{}, fmt.Errorf("lookup unexpected status: %d", resp.StatusCode)
	}

	var peer lookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&peer); err != nil {
		return lookupResponse{}, err
	}
	return peer, nil
}

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

func unregisterAndExit(serverAddr, clientID string) {
	if err := unregisterWithServer(serverAddr, clientID); err != nil {
		log.Printf("unregister failed: %v", err)
	}
}

func unregisterWithServer(serverAddr, clientID string) error {
	payload := unregisterRequest{ID: clientID}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := "http://" + serverAddr + "/unregister"
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

func handleSignals(serverAddr, clientID string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	unregisterAndExit(serverAddr, clientID)
	os.Exit(0)
}
