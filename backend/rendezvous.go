package main

import (
	"fmt"
	"log"
	"net/http"
)

type registerRequest struct {
	ID         string   `json:"id"`
	Ufrag      string   `json:"ufrag"`
	Password   string   `json:"password"`
	Candidates []string `json:"candidates"`
	TTLSeconds int      `json:"ttl_seconds"`
}

type lookupRequest struct {
	ID string `json:"id"`
}

type unregisterRequest struct {
	ID string `json:"id"`
}

type connectIntentRequest struct {
	FromID     string `json:"from_id"`
	ToID       string `json:"to_id"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type pollIntentRequest struct {
	ID string `json:"id"`
}

type lookupResponse struct {
	ID         string   `json:"id"`
	Ufrag      string   `json:"ufrag"`
	Password   string   `json:"password"`
	Candidates []string `json:"candidates"`
}

type IceInfo struct {
	ID         string
	Ufrag      string
	Password   string
	Candidates []string
}

// ICE registration & lookup
func registerICE(serverAddr, clientID string, info IceInfo, ttlSeconds int) error {
	payload := registerRequest{
		ID:         clientID,
		Ufrag:      info.Ufrag,
		Password:   info.Password,
		Candidates: info.Candidates,
		TTLSeconds: ttlSeconds,
	}
	log.Printf("registering ICE info client_id=%s candidates=%d ttl=%ds", clientID, len(info.Candidates), ttlSeconds)
	return postJSON(serverAddr, "/register", payload, nil, http.StatusOK)
}

func lookupICE(serverAddr, targetID string) (IceInfo, bool, error) {
	payload := lookupRequest{ID: targetID}
	var peer lookupResponse
	status, err := postJSONWithStatus(serverAddr, "/lookup", payload, &peer)
	if err != nil {
		return IceInfo{}, false, err
	}
	if status == http.StatusTooManyRequests {
		return IceInfo{}, false, rateLimitError{}
	}
	if status == http.StatusNotFound {
		return IceInfo{}, false, nil
	}
	if status != http.StatusOK {
		return IceInfo{}, false, fmt.Errorf("unexpected status: %d", status)
	}
	return IceInfo{
		ID:         peer.ID,
		Ufrag:      peer.Ufrag,
		Password:   peer.Password,
		Candidates: peer.Candidates,
	}, true, nil
}

type rateLimitError struct{}

func (rateLimitError) Error() string {
	return "rate limited by rendezvous server"
}

// Intents
func sendConnectIntent(serverAddr, fromID, toID string, ttlSeconds int) error {
	payload := connectIntentRequest{
		FromID:     fromID,
		ToID:       toID,
		TTLSeconds: ttlSeconds,
	}
	log.Printf("intent sent from=%s to=%s", fromID, toID)
	return postJSON(serverAddr, "/intent", payload, nil, http.StatusOK)
}

func pollConnectIntent(serverAddr, clientID string) (IceInfo, bool, error) {
	payload := pollIntentRequest{ID: clientID}
	var peer lookupResponse
	status, err := postJSONWithStatus(serverAddr, "/poll", payload, &peer)
	if err != nil {
		return IceInfo{}, false, err
	}
	if status == http.StatusNotFound {
		return IceInfo{}, false, nil
	}
	if status != http.StatusOK {
		return IceInfo{}, false, fmt.Errorf("unexpected status: %d", status)
	}
	return IceInfo{
		ID:         peer.ID,
		Ufrag:      peer.Ufrag,
		Password:   peer.Password,
		Candidates: peer.Candidates,
	}, true, nil
}

// Unregister
func unregisterWithServer(serverAddr, clientID string) error {
	payload := unregisterRequest{ID: clientID}
	return postJSON(serverAddr, "/unregister", payload, nil, http.StatusOK, http.StatusNotFound)
}

// RegisterICE is a test-friendly wrapper around registerICE.
func RegisterICE(serverAddr, clientID string, info IceInfo, ttlSeconds int) error {
	return registerICE(serverAddr, clientID, info, ttlSeconds)
}

