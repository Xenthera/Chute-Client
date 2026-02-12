package main

import (
	"errors"
	"fmt"
)

type Client struct {
	clientID   string
	serverAddr string
	session    *ChuteSession
}

func NewClient(clientID, serverAddr string, session *ChuteSession) *Client {
	return &Client{
		clientID:   clientID,
		serverAddr: serverAddr,
		session:    session,
	}
}

func (c *Client) Register(port int) error {
	return registerWithServer(c.serverAddr, c.clientID, port)
}

func (c *Client) Unregister() error {
	return unregisterWithServer(c.serverAddr, c.clientID)
}

func (c *Client) Connect(targetID string) error {
	if c.session.IsConnected() {
		return errors.New("already connected")
	}
	peer, err := lookupPeer(c.serverAddr, targetID)
	if err != nil {
		return err
	}
	return c.session.Connect(peer, targetID)
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

func (c *Client) Disconnect() error {
	return c.session.Close()
}

func (c *Client) IsConnected() bool {
	return c.session.IsConnected()
}

func (c *Client) ReceiveChan() <-chan []byte {
	return c.session.ReceiveChan
}

