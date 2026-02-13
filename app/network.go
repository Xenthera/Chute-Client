package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

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

func postJSONWithStatus(serverAddr, path string, payload any, response any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	url := "http://" + serverAddr + path
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if response != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func sendUDP(conn *net.UDPConn, peerIP string, peerPort int, payload []byte) error {
	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(peerIP, fmt.Sprintf("%d", peerPort)))
	if err != nil {
		return fmt.Errorf("resolve udp addr failed: %w", err)
	}

	if _, err := conn.WriteToUDP(payload, remoteAddr); err != nil {
		return fmt.Errorf("udp send failed: %w", err)
	}
	return nil
}
