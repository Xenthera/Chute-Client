package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

type Client struct {
	clientID   string
	serverAddr string
	session    *ChuteSession
	localIPs   []string
	localPort  int
	publicIP   string
	publicPort int
}

func NewClient(clientID, serverAddr string, session *ChuteSession) *Client {
	return &Client{
		clientID:   clientID,
		serverAddr: serverAddr,
		session:    session,
	}
}

func (c *Client) Register(conn *net.UDPConn) error {
	localIPs, err := detectLocalIPs()
	if err != nil {
		return err
	}
	localPort := conn.LocalAddr().(*net.UDPAddr).Port
	ip, port, err := discoverPublicEndpoint(conn)
	if err != nil {
		return err
	}

	publicIPv6 := ""
	log.Printf("client endpoints local_ips=%v local_port=%d public=%s:%d", localIPs, localPort, ip, port)

	c.localIPs = localIPs
	c.localPort = localPort
	c.publicIP = ip
	c.publicPort = port

	return registerWithServer(c.serverAddr, c.clientID, localIPs, localPort, ip, port, publicIPv6)
}

func (c *Client) Unregister() error {
	return unregisterWithServer(c.serverAddr, c.clientID)
}

func (c *Client) SendMessage(targetID string, data []byte) error {
	if !c.session.IsConnected() {
		return errors.New("no active session")
	}
	activePeer := c.session.CurrentPeerID()
	if targetID == "" {
		targetID = activePeer
	}
	if targetID == "" {
		return errors.New("no active peer")
	}
	if activePeer != "" && activePeer != targetID {
		return fmt.Errorf("connected to %s", activePeer)
	}
	return c.session.Send(data)
}

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
			log.Printf("poll tick (idle=%t)", !c.IsConnected())

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

func (c *Client) Disconnect() error {
	return c.session.Close()
}

func (c *Client) IsConnected() bool {
	return c.session.IsConnected()
}

func (c *Client) ReceiveChan() <-chan []byte {
	return c.session.ReceiveChan
}
