package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/pion/ice/v2"
)

const (
	iceTTLSeconds         = 60
	intentTTLSeconds      = 20
	iceGatherTimeout      = 10 * time.Second
	iceConnectTimeout     = 2 * time.Minute
	iceLookupPollInterval = 1 * time.Second
	rateLimitBackoff      = 3 * time.Second
)

type ConnectionManager struct {
	localID    string
	serverAddr string

	sessionSetter func(*ChuteSession)

	iceMu    sync.Mutex
	iceAgent *ice.Agent

	healthMu           sync.RWMutex
	rendezvousHealthy  bool
	rendezvousChecked  bool
}

// Construction & wiring
func NewConnectionManager(localID, serverAddr string) *ConnectionManager {
	return &ConnectionManager{
		localID:    localID,
		serverAddr: serverAddr,
	}
}

func (m *ConnectionManager) SetSessionSetter(setter func(*ChuteSession)) {
	m.sessionSetter = setter
}

// Public entrypoints
func (m *ConnectionManager) Connect(targetID string) (*ChuteSession, error) {
	if targetID == "" {
		return nil, errors.New("missing target id")
	}

	agent, localInfo, err := m.createICEAgent()
	if err != nil {
		return nil, err
	}

	if err := registerICE(m.serverAddr, m.localID, localInfo, iceTTLSeconds); err != nil {
		_ = agent.Close()
		return nil, err
	}

	if err := sendConnectIntent(m.serverAddr, m.localID, targetID, intentTTLSeconds); err != nil {
		log.Printf("connect intent failed target=%s err=%v", targetID, err)
	}

	remoteInfo, err := waitForICEInfo(m.serverAddr, targetID, m.localID, iceConnectTimeout)
	if err != nil {
		_ = agent.Close()
		return nil, err
	}

	return m.startICE(agent, targetID, remoteInfo)
}

func (m *ConnectionManager) ConnectWithPeerInfo(info IceInfo) (*ChuteSession, error) {
	if info.ID == "" {
		return nil, errors.New("missing peer id")
	}

	agent, localInfo, err := m.createICEAgent()
	if err != nil {
		return nil, err
	}

	if err := registerICE(m.serverAddr, m.localID, localInfo, iceTTLSeconds); err != nil {
		_ = agent.Close()
		return nil, err
	}

	return m.startICE(agent, info.ID, info)
}

// ICE setup & gather
func (m *ConnectionManager) createICEAgent() (*ice.Agent, IceInfo, error) {
	stunServer := stunServerAddr()
	url, err := ice.ParseURL("stun:" + stunServer)
	if err != nil {
		return nil, IceInfo{}, err
	}
	agent, err := ice.NewAgent(&ice.AgentConfig{
		NetworkTypes:    []ice.NetworkType{ice.NetworkTypeUDP4},
		Urls:            []*ice.URL{url},
		IncludeLoopback: true,
	})
	if err != nil {
		return nil, IceInfo{}, err
	}

	ufrag, pwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		_ = agent.Close()
		return nil, IceInfo{}, err
	}

	candidates, err := gatherCandidates(agent)
	if err != nil {
		_ = agent.Close()
		return nil, IceInfo{}, err
	}

	return agent, IceInfo{
		ID:         m.localID,
		Ufrag:      ufrag,
		Password:   pwd,
		Candidates: candidates,
	}, nil
}

func gatherCandidates(agent *ice.Agent) ([]string, error) {
	var (
		mu         sync.Mutex
		candidates []string
		done       = make(chan struct{})
	)

	agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			close(done)
			return
		}
		log.Printf("ICE candidate gathered: %s", c.Marshal())
		mu.Lock()
		candidates = append(candidates, c.Marshal())
		mu.Unlock()
	})

	if err := agent.GatherCandidates(); err != nil {
		return nil, err
	}

	select {
	case <-done:
	case <-time.After(iceGatherTimeout):
		return nil, errors.New("ice candidate gathering timed out")
	}

	return candidates, nil
}

// ICE connect & QUIC bootstrap
func (m *ConnectionManager) startICE(agent *ice.Agent, targetID string, remote IceInfo) (*ChuteSession, error) {
	m.setICEAgent(agent)
	agent.OnConnectionStateChange(func(state ice.ConnectionState) {
		log.Printf("ICE state for %s: %s", targetID, state.String())
	})
	if err := agent.SetRemoteCredentials(remote.Ufrag, remote.Password); err != nil {
		_ = agent.Close()
		return nil, err
	}
	for _, c := range remote.Candidates {
		cand, err := ice.UnmarshalCandidate(c)
		if err != nil {
			_ = agent.Close()
			return nil, err
		}
		if err := agent.AddRemoteCandidate(cand); err != nil {
			_ = agent.Close()
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), iceConnectTimeout)
	defer cancel()

	var conn *ice.Conn
	var err error
	if m.localID < targetID {
		conn, err = agent.Dial(ctx, remote.Ufrag, remote.Password)
	} else {
		conn, err = agent.Accept(ctx, remote.Ufrag, remote.Password)
	}
	if err != nil {
		_ = agent.Close()
		return nil, err
	}

	packetConn := newICEPacketConn(conn)
	session := NewChuteSession(packetConn, m.localID)
	session.SetOnClose(func() {
		m.closeICE()
		_ = unregisterWithServer(m.serverAddr, m.localID)
	})

	isInitiator := m.localID < targetID
	if isInitiator {
		remoteEndpoint, err := endpointFromNetAddr(conn.RemoteAddr())
		if err != nil {
			_ = agent.Close()
			return nil, err
		}
		if err := session.ConnectWithContext(ctx, remoteEndpoint, targetID); err != nil {
			_ = agent.Close()
			return nil, err
		}
		if m.sessionSetter != nil {
			m.sessionSetter(session)
		}
		return session, nil
	}

	session.Start()
	if err := waitForSession(session, iceConnectTimeout); err != nil {
		_ = agent.Close()
		return nil, err
	}
	if m.sessionSetter != nil {
		m.sessionSetter(session)
	}
	return session, nil
}

// ICE lifecycle
func (m *ConnectionManager) setICEAgent(agent *ice.Agent) {
	m.iceMu.Lock()
	m.iceAgent = agent
	m.iceMu.Unlock()
}

func (m *ConnectionManager) closeICE() {
	m.iceMu.Lock()
	agent := m.iceAgent
	m.iceAgent = nil
	m.iceMu.Unlock()
	if agent != nil {
		_ = agent.Close()
	}
}

func (m *ConnectionManager) SetRendezvousHealth(ok bool) {
	m.healthMu.Lock()
	m.rendezvousHealthy = ok
	m.rendezvousChecked = true
	m.healthMu.Unlock()
}

func (m *ConnectionManager) RendezvousHealth() (bool, bool) {
	m.healthMu.RLock()
	ok := m.rendezvousHealthy
	checked := m.rendezvousChecked
	m.healthMu.RUnlock()
	return ok, checked
}

// Signaling helpers
func waitForICEInfo(serverAddr, targetID, fromID string, timeout time.Duration) (IceInfo, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info, ok, err := lookupICE(serverAddr, targetID, fromID)
		if err != nil {
			if _, limited := err.(rateLimitError); limited {
				time.Sleep(rateLimitBackoff)
				continue
			}
			if _, declined := err.(declineError); declined {
				return IceInfo{}, err
			}
			return IceInfo{}, err
		}
		if ok {
			return info, nil
		}
		time.Sleep(iceLookupPollInterval)
	}
	return IceInfo{}, fmt.Errorf("timed out waiting for ICE info for %s", targetID)
}

func stunServerAddr() string {
	if v := os.Getenv("CHUTE_STUN_SERVER"); v != "" {
		return v
	}
	return "stun.l.google.com:19302"
}

// ICE -> net.PacketConn adapter
type icePacketConn struct {
	conn *ice.Conn
}

func newICEPacketConn(conn *ice.Conn) net.PacketConn {
	return &icePacketConn{conn: conn}
}

func (c *icePacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.conn.Read(p)
	return n, c.conn.RemoteAddr(), err
}

func (c *icePacketConn) WriteTo(p []byte, _ net.Addr) (n int, err error) {
	return c.conn.Write(p)
}

func (c *icePacketConn) Close() error {
	return c.conn.Close()
}

func (c *icePacketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *icePacketConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *icePacketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *icePacketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func waitForSession(session *ChuteSession, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if session.IsConnected() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("timeout waiting for QUIC connection")
}
