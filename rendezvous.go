package main

import (
	"log"
	"net/http"
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

type PeerEndpoint struct {
	IP   string
	Port int
}

func registerWithServer(serverAddr, clientID string, port int) error {
	payload := registerRequest{
		ID:   clientID,
		Port: port,
	}
	log.Printf("registering client_id=%s udp_port=%d", clientID, port)
	return postJSON(serverAddr, "/register", payload, nil, http.StatusOK)
}

func lookupPeer(serverAddr, targetID string) (PeerEndpoint, error) {
	payload := lookupRequest{ID: targetID}
	var peer lookupResponse
	if err := postJSON(serverAddr, "/lookup", payload, &peer, http.StatusOK); err != nil {
		return PeerEndpoint{}, err
	}
	endpoint := PeerEndpoint{
		IP:   peer.PublicIP,
		Port: peer.PublicPort,
	}
	log.Printf("lookup ok target=%s udp_endpoint=%s:%d", targetID, endpoint.IP, endpoint.Port)
	return endpoint, nil
}

func unregisterWithServer(serverAddr, clientID string) error {
	payload := unregisterRequest{ID: clientID}
	return postJSON(serverAddr, "/unregister", payload, nil, http.StatusOK, http.StatusNotFound)
}

func unregisterAndExit(serverAddr, clientID string) {
	if err := unregisterWithServer(serverAddr, clientID); err != nil {
		log.Printf("unregister failed: %v", err)
	}
}

// RegisterWithServer is a test-friendly wrapper around registerWithServer.
func RegisterWithServer(serverAddr, clientID string, port int) error {
	return registerWithServer(serverAddr, clientID, port)
}

// LookupPeer is a test-friendly wrapper around lookupPeer.
func LookupPeer(serverAddr, targetID string) (PeerEndpoint, error) {
	return lookupPeer(serverAddr, targetID)
}

// UnregisterWithServer is a test-friendly wrapper around unregisterWithServer.
func UnregisterWithServer(serverAddr, clientID string) error {
	return unregisterWithServer(serverAddr, clientID)
}
