package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type Client struct {
	clientID   string
	serverAddr string
	receive    chan []byte

	sessionMu sync.RWMutex
	session   *ChuteSession
}

// Construction
func NewClient(clientID, serverAddr string) *Client {
	return &Client{
		clientID:   clientID,
		serverAddr: serverAddr,
		receive:    make(chan []byte, 16),
	}
}

// Connection lifecycle
func (c *Client) Unregister() error {
	return unregisterWithServer(c.serverAddr, c.clientID)
}

func (c *Client) SendMessage(targetID string, data []byte) error {
	session := c.getSession()
	if session == nil || !session.IsConnected() {
		return errors.New("no active session")
	}
	activePeer := session.CurrentPeerID()
	if targetID == "" {
		targetID = activePeer
	}
	if targetID == "" {
		return errors.New("no active peer")
	}
	if activePeer != "" && activePeer != targetID {
		return fmt.Errorf("connected to %s", activePeer)
	}
	return session.Send(data)
}

// Polling
func (c *Client) StartPolling(ctx context.Context, manager *ConnectionManager) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.IsConnected() {
				continue
			}
			intent, ok, err := pollConnectIntent(c.serverAddr, c.clientID)
			if err != nil {
				log.Printf("poll failed: %v", err)
				continue
			}
			if !ok {
				continue
			}
			log.Printf("incoming connection request from %s", intent.ID)
			if _, err := manager.ConnectWithPeerInfo(intent); err != nil {
				log.Printf("connect back failed: %v", err)
			}
		}
	}
}

// Session state
func (c *Client) Disconnect() error {
	session := c.getSession()
	if session == nil {
		return nil
	}
	return session.Close()
}

func (c *Client) IsConnected() bool {
	session := c.getSession()
	if session == nil {
		return false
	}
	return session.IsConnected()
}

func (c *Client) ReceiveChan() <-chan []byte {
	return c.receive
}

// Session wiring
func (c *Client) SetSession(session *ChuteSession) {
	c.sessionMu.Lock()
	c.session = session
	c.sessionMu.Unlock()

	if session == nil {
		return
	}
	go func() {
		for msg := range session.ReceiveChan {
			c.receive <- msg
		}
	}()
}

// Internal helpers
func (c *Client) getSession() *ChuteSession {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.session
}
