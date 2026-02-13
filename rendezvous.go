package main

import (
	"fmt"
	"log"
	"net/http"
)

type registerRequest struct {
	ID         string   `json:"id"`
	LocalIPs   []string `json:"local_ips"`
	LocalPort  int      `json:"local_port"`
	PublicIP   string   `json:"public_ip"`
	PublicPort int      `json:"public_port"`
	PublicIPv6 string   `json:"public_ipv6,omitempty"`
}

type lookupRequest struct {
	ID string `json:"id"`
}

type connectIntentRequest struct {
	FromID     string   `json:"from_id"`
	ToID       string   `json:"to_id"`
	LocalIPs   []string `json:"local_ips"`
	LocalPort  int      `json:"local_port"`
	PublicIP   string   `json:"public_ip"`
	PublicPort int      `json:"public_port"`
}

type pollIntentRequest struct {
	ID string `json:"id"`
}

type unregisterRequest struct {
	ID string `json:"id"`
}

type lookupResponse struct {
	ID         string   `json:"id"`
	LocalIPs   []string `json:"local_ips"`
	LocalPort  int      `json:"local_port"`
	PublicIP   string   `json:"public_ip"`
	PublicPort int      `json:"public_port"`
	PublicIPv6 string   `json:"public_ipv6,omitempty"`
}

type PeerInfo struct {
	ID         string
	LocalIPs   []string
	LocalPort  int
	PublicIP   string
	PublicPort int
	PublicIPv6 string
}

type PeerEndpoint struct {
	IP   string
	Port int
}

func registerWithServer(serverAddr, clientID string, localIPs []string, localPort int, publicIP string, publicPort int, publicIPv6 string) error {
	payload := registerRequest{
		ID:         clientID,
		LocalIPs:   localIPs,
		LocalPort:  localPort,
		PublicIP:   publicIP,
		PublicPort: publicPort,
		PublicIPv6: publicIPv6,
	}
	log.Printf("registering client_id=%s local_port=%d public=%s:%d public_ipv6=%s local_ips=%v", clientID, localPort, publicIP, publicPort, publicIPv6, localIPs)
	return postJSON(serverAddr, "/register", payload, nil, http.StatusOK)
}

func lookupPeer(serverAddr, targetID string) (PeerEndpoint, error) {
	info, err := lookupPeerInfo(serverAddr, targetID)
	if err != nil {
		return PeerEndpoint{}, err
	}
	endpoint := PeerEndpoint{
		IP:   info.PublicIP,
		Port: info.PublicPort,
	}
	log.Printf("lookup ok target=%s udp_endpoint=%s:%d", targetID, endpoint.IP, endpoint.Port)
	return endpoint, nil
}

func lookupPeerInfo(serverAddr, targetID string) (PeerInfo, error) {
	payload := lookupRequest{ID: targetID}
	var peer lookupResponse
	if err := postJSON(serverAddr, "/lookup", payload, &peer, http.StatusOK); err != nil {
		return PeerInfo{}, err
	}
	return PeerInfo{
		ID:         peer.ID,
		LocalIPs:   peer.LocalIPs,
		LocalPort:  peer.LocalPort,
		PublicIP:   peer.PublicIP,
		PublicPort: peer.PublicPort,
		PublicIPv6: peer.PublicIPv6,
	}, nil
}

func unregisterWithServer(serverAddr, clientID string) error {
	payload := unregisterRequest{ID: clientID}
	return postJSON(serverAddr, "/unregister", payload, nil, http.StatusOK, http.StatusNotFound)
}

func sendConnectIntent(serverAddr, fromID, toID string, localIPs []string, localPort int, publicIP string, publicPort int) error {
	payload := connectIntentRequest{
		FromID:     fromID,
		ToID:       toID,
		LocalIPs:   localIPs,
		LocalPort:  localPort,
		PublicIP:   publicIP,
		PublicPort: publicPort,
	}
	log.Printf("intent sent from=%s to=%s public=%s:%d local_port=%d", fromID, toID, publicIP, publicPort, localPort)
	return postJSON(serverAddr, "/intent", payload, nil, http.StatusOK)
}

func pollConnectIntent(serverAddr, clientID string) (PeerInfo, bool, error) {
	payload := pollIntentRequest{ID: clientID}
	var peer lookupResponse
	status, err := postJSONWithStatus(serverAddr, "/poll", payload, &peer)
	if err != nil {
		return PeerInfo{}, false, err
	}
	if status == http.StatusNotFound {
		return PeerInfo{}, false, nil
	}
	if status != http.StatusOK {
		return PeerInfo{}, false, fmt.Errorf("unexpected status: %d", status)
	}
	return PeerInfo{
		ID:         peer.ID,
		LocalIPs:   peer.LocalIPs,
		LocalPort:  peer.LocalPort,
		PublicIP:   peer.PublicIP,
		PublicPort: peer.PublicPort,
		PublicIPv6: peer.PublicIPv6,
	}, true, nil
}

func unregisterAndExit(serverAddr, clientID string) {
	if err := unregisterWithServer(serverAddr, clientID); err != nil {
		log.Printf("unregister failed: %v", err)
	}
}

// RegisterWithServer is a test-friendly wrapper around registerWithServer.
func RegisterWithServer(serverAddr, clientID string, localIPs []string, localPort int, publicIP string, publicPort int, publicIPv6 string) error {
	return registerWithServer(serverAddr, clientID, localIPs, localPort, publicIP, publicPort, publicIPv6)
}

// LookupPeer is a test-friendly wrapper around lookupPeer.
func LookupPeer(serverAddr, targetID string) (PeerEndpoint, error) {
	return lookupPeer(serverAddr, targetID)
}

// UnregisterWithServer is a test-friendly wrapper around unregisterWithServer.
func UnregisterWithServer(serverAddr, clientID string) error {
	return unregisterWithServer(serverAddr, clientID)
}
