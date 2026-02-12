package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

func runCLI(ctx context.Context, cancel context.CancelFunc, client *Client, clientID, serverAddr string) {
	scanner := bufio.NewScanner(os.Stdin)
	printHelp()
	go printReceived(ctx, client)

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
			_ = client.Disconnect()
			if err := client.Unregister(); err != nil {
				log.Printf("unregister failed: %v", err)
			}
			cancel()
			return
		case strings.HasPrefix(line, "connect "):
			id, ok := parseConnectID(line)
			if !ok {
				fmt.Println("usage: connect <id>")
				continue
			}
			if err := client.Connect(id); err != nil {
				log.Printf("connect failed client_id=%s target=%s err=%v", clientID, id, err)
				continue
			}
			message := fmt.Sprintf("hello from %s\n", clientID)
			if err := client.SendMessage(id, []byte(message)); err != nil {
				log.Printf("connect hello failed client_id=%s target=%s err=%v", clientID, id, err)
				continue
			}
			log.Printf("connect ok client_id=%s target=%s", clientID, id)
		case strings.HasPrefix(line, "udp "):
			message, ok := parseUDPCommand(line)
			if !ok {
				fmt.Println("usage: udp <message>")
				continue
			}
			if !client.IsConnected() {
				log.Printf("udp denied client_id=%s err=%v", clientID, errors.New("no active session"))
				continue
			}
			if err := client.SendMessage("", []byte(message)); err != nil {
				log.Printf("udp failed client_id=%s err=%v", clientID, err)
				continue
			}
			log.Printf("udp ok client_id=%s", clientID)
		default:
			printHelp()
		}
	}
}

func printHelp() {
	fmt.Println("commands:")
	fmt.Println("  connect <id>")
	fmt.Println("  udp <message>")
	fmt.Println("  exit")
}

func parseConnectID(line string) (string, bool) {
	id := strings.TrimSpace(strings.TrimPrefix(line, "connect "))
	if id == "" {
		return "", false
	}
	return id, true
}

func parseUDPCommand(line string) (string, bool) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return "", false
	}
	message := strings.TrimSpace(strings.TrimPrefix(line, "udp "))
	if message == "" {
		return "", false
	}
	return message, true
}

func printReceived(ctx context.Context, client *Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-client.ReceiveChan():
			if !ok {
				return
			}
			fmt.Printf("\nreceived: %s\n> ", strings.TrimSpace(string(msg)))
		}
	}
}
