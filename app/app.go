package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type App struct {
	ctx        context.Context
	serverAddr string

	mu      sync.Mutex
	client  *Client
	manager *ConnectionManager
}

type StatusResponse struct {
	ClientID          string `json:"client_id"`
	ServerAddr        string `json:"server_addr"`
	Connected         bool   `json:"connected"`
	PeerID            string `json:"peer_id"`
	RendezvousHealthy bool   `json:"rendezvous_healthy"`
	RendezvousChecked bool   `json:"rendezvous_checked"`
}

func NewApp(serverAddr string) *App {
	return &App{serverAddr: serverAddr}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	clientID, err := generateClientID()
	if err != nil {
		log.Printf("client id error: %v", err)
		return
	}

	log.Printf("chute client starting")
	log.Printf("client id: %s", formatClientID(clientID))
	log.Printf("server: %s", a.serverAddr)

	client := NewClient(clientID, a.serverAddr)
	manager := NewConnectionManager(clientID, a.serverAddr)
	manager.SetSessionSetter(client.SetSession)

	a.mu.Lock()
	a.client = client
	a.manager = manager
	a.mu.Unlock()

	go client.StartPolling(ctx, manager)
	go checkRendezvousHealth(a.serverAddr, manager)
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil {
		return
	}
	_ = client.Disconnect()
	if err := client.Unregister(); err != nil {
		log.Printf("unregister failed: %v", err)
	}
}

func (a *App) Status() StatusResponse {
	a.mu.Lock()
	client := a.client
	manager := a.manager
	a.mu.Unlock()

	if client == nil || manager == nil {
		return StatusResponse{}
	}

	ok, checked := manager.RendezvousHealth()
	return StatusResponse{
		ClientID:          client.clientID,
		ServerAddr:        a.serverAddr,
		Connected:         client.IsConnected(),
		PeerID:            client.CurrentPeerID(),
		RendezvousHealthy: ok,
		RendezvousChecked: checked,
	}
}

func (a *App) Connect(targetID string) error {
	a.mu.Lock()
	manager := a.manager
	a.mu.Unlock()
	if manager == nil {
		return fmt.Errorf("client not ready")
	}
	targetID = strings.ReplaceAll(strings.TrimSpace(targetID), " ", "")
	if targetID == "" {
		return fmt.Errorf("missing target id")
	}
	_, err := manager.Connect(targetID)
	if err == nil {
		return nil
	}
	if _, declined := err.(declineError); declined {
		return fmt.Errorf("connection declined")
	}
	return fmt.Errorf("%v", err)
}

func (a *App) Disconnect() error {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil {
		return fmt.Errorf("client not ready")
	}
	return client.Disconnect()
}

func (a *App) Send(message string) error {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil {
		return fmt.Errorf("client not ready")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("message required")
	}
	return client.SendMessage("", []byte(message))
}

func (a *App) Messages() []string {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil {
		return nil
	}
	return drainMessages(client.ReceiveChan(), 50)
}

func (a *App) Pending() string {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil {
		return ""
	}
	intent, ok := client.getPendingIntent()
	if !ok {
		return ""
	}
	return intent.ID
}

func (a *App) Accept() error {
	a.mu.Lock()
	client := a.client
	manager := a.manager
	a.mu.Unlock()
	if client == nil || manager == nil {
		return fmt.Errorf("client not ready")
	}
	intent, ok := client.getPendingIntent()
	if !ok {
		return fmt.Errorf("no pending request")
	}
	client.clearPendingIntent()
	_, err := manager.ConnectWithPeerInfo(intent)
	return err
}

func (a *App) Decline() error {
	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	if client == nil {
		return fmt.Errorf("client not ready")
	}
	intent, ok := client.getPendingIntent()
	if !ok {
		return fmt.Errorf("no pending request")
	}
	client.clearPendingIntent()
	if err := sendDecline(a.serverAddr, intent.ID, client.clientID, 20); err != nil {
		return err
	}
	return nil
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

func drainMessages(ch <-chan []byte, max int) []string {
	if max <= 0 {
		max = 1
	}
	out := make([]string, 0, max)
	for i := 0; i < max; i++ {
		select {
		case msg := <-ch:
			out = append(out, string(msg))
		default:
			return out
		}
	}
	return out
}
