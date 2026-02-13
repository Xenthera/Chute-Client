package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"
)

// ConnectionManager coordinates outbound connection attempts while maintaining
// a single active ChuteSession. It will eventually own retry strategy and
// deterministic winner logic when concurrent connect attempts occur.
type ConnectionManager struct {
	localID    string
	serverAddr string
	localPort  int
	listener   *quic.Listener
	session    *ChuteSession
	localIPs   []string
	publicIP   string
	publicPort int

	// mutex guards session state and in-flight attempts.
	mutex sync.Mutex

	// dialing prevents multiple concurrent outbound attempts.
	dialing bool

	// attemptID increments per connect attempt for deterministic winner logic.
	attemptID uint64

	// maxRetries and retryBackoff will control retry policy.
	maxRetries   int
	retryBackoff []time.Duration

	// winnerID will hold the attempt that "won" during simultaneous connects.
	winnerID uint64
}

const (
	lanDialTimeout    = 2 * time.Second
	publicDialTimeout = 3 * time.Second
)

// NewConnectionManager scaffolds a manager for connection attempts.
func NewConnectionManager(localID, serverAddr string, listener *quic.Listener, session *ChuteSession) *ConnectionManager {
	return &ConnectionManager{
		localID:    localID,
		serverAddr: serverAddr,
		localPort:  0,
		listener:   listener,
		session:    session,
	}
}

func NewConnectionManagerWithPort(localID, serverAddr string, listener *quic.Listener, session *ChuteSession, localPort int) *ConnectionManager {
	return &ConnectionManager{
		localID:    localID,
		serverAddr: serverAddr,
		localPort:  localPort,
		listener:   listener,
		session:    session,
	}
}

func (m *ConnectionManager) SetLocalEndpoints(localIPs []string, localPort int, publicIP string, publicPort int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.localIPs = append([]string(nil), localIPs...)
	m.localPort = localPort
	m.publicIP = publicIP
	m.publicPort = publicPort
}

// Connect starts a connection attempt to the target ID.
// TODO:
// - increment attemptID and record attempt state
// - run the connection attempt chain (dial, NAT hole punch, retry)
// - apply deterministic winner logic when simultaneous attempts succeed
// - update session state on success and return the winning session
// - surface appropriate errors on failure
func (m *ConnectionManager) Connect(targetID string) (*ChuteSession, error) {
	if err := m.announceIntent(targetID); err != nil {
		log.Printf("connect intent failed target=%s err=%v", targetID, err)
	}
	// Step 1: LAN direct attempt.
	if session, ok, err := m.attemptLANDirect(targetID); ok || err != nil {
		return session, err
	}

	// Step 2: Public IPv4 direct attempt (via STUN).
	if session, ok, err := m.attemptPublicDirect(targetID); ok || err != nil {
		return session, err
	}

	// Step 3: Coordinated simultaneous dial / hole punching.
	if session, ok, err := m.attemptHolePunch(targetID); ok || err != nil {
		return session, err
	}

	// Step 4: Any last-ditch optional methods.
	if session, ok, err := m.attemptFallbacks(targetID); ok || err != nil {
		return session, err
	}

	// Step 5: Return error if all fail.
	return nil, errors.New("all connection attempts failed")
}

func (m *ConnectionManager) announceIntent(targetID string) error {
	if targetID == "" {
		return errors.New("missing target id")
	}
	m.mutex.Lock()
	localIPs := append([]string(nil), m.localIPs...)
	localPort := m.localPort
	publicIP := m.publicIP
	publicPort := m.publicPort
	m.mutex.Unlock()

	if len(localIPs) == 0 || localPort == 0 || publicIP == "" || publicPort == 0 {
		return errors.New("client endpoints not ready")
	}
	return sendConnectIntent(m.serverAddr, m.localID, targetID, localIPs, localPort, publicIP, publicPort)
}

// determineWinner decides if this client should initiate the dial when both
// peers attempt to connect simultaneously. The placeholder rule is "lower
// client ID wins" (lexicographic). If true, we dial; if false, we accept.
func (m *ConnectionManager) determineWinner(peerID string) bool {
	return m.localID < peerID
}

// attemptLANDirect tries to connect using LAN discovery or local addressing.
func (m *ConnectionManager) attemptLANDirect(targetID string) (*ChuteSession, bool, error) {
	log.Printf("LAN attempt: looking for %s on the local network", targetID)

	info, err := lookupPeerInfo(m.serverAddr, targetID)
	if err != nil {
		log.Printf("LAN attempt failed: could not look up %s (%v)", targetID, err)
		return nil, false, err
	}

	return m.attemptLANDirectWithInfo(targetID, info)
}

func isOnLocalSubnet(targetIP net.IP) bool {
	target := targetIP.To4()
	if target == nil {
		return false
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil {
			continue
		}
		if ipnet.Contains(target) {
			return true
		}
	}
	return false
}

// attemptPublicDirect tries a public IP direct connection using STUN.
func (m *ConnectionManager) attemptPublicDirect(targetID string) (*ChuteSession, bool, error) {
	log.Printf("Public IPv4 attempt: looking up %s via rendezvous", targetID)

	info, err := lookupPeerInfo(m.serverAddr, targetID)
	if err != nil {
		log.Printf("Public IPv4 attempt failed: could not look up %s (%v)", targetID, err)
		return nil, false, err
	}
	return m.attemptPublicDirectWithInfo(targetID, info)
}

func (m *ConnectionManager) attemptLANDirectWithInfo(targetID string, info PeerInfo) (*ChuteSession, bool, error) {
	candidateIP := selectLANIP(info.LocalIPs)
	if candidateIP == nil {
		log.Printf("LAN attempt skipped: %s is not on the same subnet", targetID)
		return nil, false, nil
	}
	if info.LocalPort <= 0 {
		err := fmt.Errorf("invalid local port %d", info.LocalPort)
		log.Printf("LAN attempt failed: invalid local port for %s (%v)", targetID, err)
		return nil, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), lanDialTimeout)
	defer cancel()
	endpoint := PeerEndpoint{IP: candidateIP.String(), Port: info.LocalPort}
	if err := m.session.ConnectWithContext(ctx, endpoint, targetID); err != nil {
		log.Printf("LAN attempt failed: could not connect to %s at %s (%v)", targetID, candidateIP.String(), err)
		return nil, false, err
	}

	log.Printf("LAN attempt succeeded: connected to %s at %s", targetID, candidateIP.String())
	return m.session, true, nil
}

func (m *ConnectionManager) attemptPublicDirectWithInfo(targetID string, info PeerInfo) (*ChuteSession, bool, error) {
	endpoint, err := publicEndpointFromInfo(info)
	if err != nil {
		log.Printf("Public IPv4 attempt skipped: missing endpoint for %s (%v)", targetID, err)
		return nil, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), publicDialTimeout)
	defer cancel()
	if err := m.session.ConnectWithContext(ctx, endpoint, targetID); err != nil {
		log.Printf("Public IPv4 attempt failed: could not connect to %s at %s (%v)", targetID, endpoint.IP, err)
		return nil, false, nil
	}

	log.Printf("Public IPv4 attempt succeeded: connected to %s at %s", targetID, endpoint.IP)
	return m.session, true, nil
}

func (m *ConnectionManager) ConnectWithPeerInfo(info PeerInfo) (*ChuteSession, error) {
	targetID := info.ID
	if targetID == "" {
		return nil, errors.New("missing peer id")
	}
	log.Printf("Incoming connect: attempting to reach %s", targetID)

	if session, ok, err := m.attemptHolePunchWithInfo(targetID, info); ok || err != nil {
		return session, err
	}
	return nil, errors.New("all connection attempts failed")
}

func selectLANIP(localIPs []string) net.IP {
	for _, ip := range localIPs {
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			continue
		}
		if isOnLocalSubnet(parsed) {
			return parsed
		}
	}
	return nil
}

func publicEndpointFromInfo(info PeerInfo) (PeerEndpoint, error) {
	if info.PublicIP == "" || info.PublicPort <= 0 {
		return PeerEndpoint{}, fmt.Errorf("missing public endpoint ip=%q port=%d", info.PublicIP, info.PublicPort)
	}
	publicIP := net.ParseIP(info.PublicIP)
	if publicIP == nil || publicIP.To4() == nil {
		return PeerEndpoint{}, fmt.Errorf("invalid public ipv4 %q", info.PublicIP)
	}
	return PeerEndpoint{IP: info.PublicIP, Port: info.PublicPort}, nil
}

// attemptHolePunch coordinates simultaneous dialing / hole punching.
func (m *ConnectionManager) attemptHolePunch(targetID string) (*ChuteSession, bool, error) {
	info, err := lookupPeerInfo(m.serverAddr, targetID)
	if err != nil {
		log.Printf("Hole punching skipped: could not look up %s (%v)", targetID, err)
		return nil, false, err
	}
	return m.attemptHolePunchWithInfo(targetID, info)
}

// attemptFallbacks runs any last-ditch optional connection methods.
func (m *ConnectionManager) attemptFallbacks(targetID string) (*ChuteSession, bool, error) {
	log.Printf("Fallback attempt skipped for %s (not implemented yet)", targetID)
	return nil, false, nil
}

func (m *ConnectionManager) attemptHolePunchWithInfo(targetID string, info PeerInfo) (*ChuteSession, bool, error) {
	if selectLANIP(info.LocalIPs) != nil {
		log.Printf("Hole punching skipped: %s is on the local network", targetID)
		return nil, false, nil
	}
	endpoint, err := publicEndpointFromInfo(info)
	if err != nil {
		log.Printf("Hole punching skipped: missing endpoint for %s (%v)", targetID, err)
		return nil, false, nil
	}

	log.Printf("Hole punching: sending repeated dials to %s at %s", targetID, endpoint.IP)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err := m.session.ConnectWithContext(ctx, endpoint, targetID)
		cancel()
		if err == nil {
			log.Printf("Hole punching succeeded: connected to %s at %s", targetID, endpoint.IP)
			return m.session, true, nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("Hole punching failed: no connection to %s after retries", targetID)
	return nil, false, nil
}
