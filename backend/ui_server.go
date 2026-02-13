package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type uiServer struct {
	client      *Client
	manager     *ConnectionManager
	serverAddr  string
	clientID    string
	httpServer  *http.Server
}

type uiStatusResponse struct {
	ClientID              string `json:"client_id"`
	ServerAddr            string `json:"server_addr"`
	Connected             bool   `json:"connected"`
	PeerID                string `json:"peer_id"`
	RendezvousRegistered  bool   `json:"rendezvous_registered"`
}

type uiConnectRequest struct {
	TargetID string `json:"target_id"`
}

type uiSendRequest struct {
	Message string `json:"message"`
}

type uiMessageResponse struct {
	Message string `json:"message"`
}

func startUIServer(ctx context.Context, addr string, client *Client, manager *ConnectionManager, serverAddr, clientID string) error {
	server := &uiServer{
		client:      client,
		manager:     manager,
		serverAddr:  serverAddr,
		clientID:    clientID,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", server.withCORS(server.handleStatus))
	mux.HandleFunc("/connect", server.withCORS(server.handleConnect))
	mux.HandleFunc("/send", server.withCORS(server.handleSend))
	mux.HandleFunc("/messages", server.withCORS(server.handleMessages))

	httpServer := &http.Server{
		Handler: mux,
	}
	server.httpServer = httpServer

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	actualAddr := listener.Addr().String()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	go func() {
		log.Printf("ui server listening on %s", actualAddr)
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("ui server error: %v", err)
		}
	}()
	return nil
}

func (s *uiServer) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *uiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp := uiStatusResponse{
		ClientID:             s.clientID,
		ServerAddr:           s.serverAddr,
		Connected:            s.client.IsConnected(),
		PeerID:               s.client.CurrentPeerID(),
		RendezvousRegistered: s.manager.IsRegistered(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *uiServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var payload uiConnectRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	targetID := strings.ReplaceAll(strings.TrimSpace(payload.TargetID), " ", "")
	if targetID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if targetID == s.clientID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot connect to your own id"})
		return
	}
	if _, err := s.manager.Connect(targetID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

func (s *uiServer) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var payload uiSendRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := s.client.SendMessage("", []byte(message)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *uiServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	select {
	case msg := <-s.client.ReceiveChan():
		writeJSON(w, http.StatusOK, uiMessageResponse{Message: string(msg)})
		return
	default:
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

// no config writer needed for fixed UI port

