package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"
)

const (
	nextProto     = "chute-quic"
	identityLimit = 64
	sessionIdle   = 5 * time.Minute
	keepAlive     = 20 * time.Second
	handshakeIdle = 10 * time.Second
)

type ChuteSession struct {
	LocalID     string
	PeerID      string
	Connected   bool
	ReceiveChan chan []byte
	Mutex       sync.Mutex

	transport  *quic.Transport
	listener   *quic.Listener
	conn       quic.Connection
	acceptOnce sync.Once
}

func NewChuteSession(conn *net.UDPConn, localID string) *ChuteSession {
	transport := &quic.Transport{Conn: conn}
	return &ChuteSession{
		LocalID:     localID,
		ReceiveChan: make(chan []byte, 16),
		transport:   transport,
	}
}

func (s *ChuteSession) Start() {
	s.acceptOnce.Do(func() {
		listener, err := s.transport.Listen(serverTLSConfig(), quicConfig())
		if err != nil {
			log.Printf("quic listen failed: %v", err)
			return
		}
		s.listener = listener
		go s.acceptLoop()
	})
}

func (s *ChuteSession) Connect(peer PeerEndpoint, id string) error {
	return s.connectWithContext(context.Background(), peer, id)
}

func (s *ChuteSession) ConnectWithContext(ctx context.Context, peer PeerEndpoint, id string) error {
	return s.connectWithContext(ctx, peer, id)
}

func (s *ChuteSession) connectWithContext(ctx context.Context, peer PeerEndpoint, id string) error {
	s.Mutex.Lock()
	if s.Connected {
		s.Mutex.Unlock()
		log.Printf("session busy peer_id=%s", s.PeerID)
		return errors.New("busy")
	}
	s.Mutex.Unlock()

	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(peer.IP),
		Port: peer.Port,
	}
	conn, err := s.transport.Dial(ctx, remoteAddr, clientTLSConfig(), quicConfig())
	if err != nil {
		return err
	}

	if err := s.handshakeDial(conn); err != nil {
		_ = conn.CloseWithError(0, "handshake failed")
		return err
	}

	s.Mutex.Lock()
	s.PeerID = id
	s.Connected = true
	s.conn = conn
	s.Mutex.Unlock()

	log.Printf("session started peer_id=%s remote=%s", s.PeerID, conn.RemoteAddr().String())
	go s.monitorConnection(conn)
	go s.readLoop(conn)
	return nil
}

func (s *ChuteSession) Close() error {
	s.Mutex.Lock()
	if !s.Connected {
		s.Mutex.Unlock()
		return nil
	}
	conn := s.conn
	s.conn = nil
	s.Connected = false
	s.PeerID = ""
	s.Mutex.Unlock()

	if conn != nil {
		_ = conn.CloseWithError(0, "session closed")
	}
	log.Printf("session closed")
	return nil
}

func (s *ChuteSession) acceptLoop() {
	for {
		conn, err := s.listener.Accept(context.Background())
		if err != nil {
			log.Printf("quic accept failed: %v", err)
			continue
		}
		go s.handleIncoming(conn)
	}
}

func (s *ChuteSession) handleIncoming(conn quic.Connection) {
	s.Mutex.Lock()
	if s.Connected {
		s.Mutex.Unlock()
		_ = conn.CloseWithError(0, "busy")
		return
	}
	s.Connected = true
	s.conn = conn
	s.Mutex.Unlock()

	peerID, err := s.handshakeAccept(conn)
	if err != nil {
		_ = conn.CloseWithError(0, "handshake failed")
		s.Mutex.Lock()
		s.Connected = false
		s.conn = nil
		s.Mutex.Unlock()
		return
	}

	s.Mutex.Lock()
	s.PeerID = peerID
	s.Mutex.Unlock()

	log.Printf("session accepted peer_id=%s remote=%s", s.PeerID, conn.RemoteAddr().String())
	go s.monitorConnection(conn)
	go s.readLoop(conn)
}

func (s *ChuteSession) Send(msg []byte) error {
	s.Mutex.Lock()
	if !s.Connected || s.conn == nil {
		s.Mutex.Unlock()
		return errors.New("no active session")
	}
	conn := s.conn
	peerID := s.PeerID
	s.Mutex.Unlock()

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}
	if _, err := stream.Write(msg); err != nil {
		_ = stream.Close()
		log.Printf("quic send failed peer_id=%s err=%v", peerID, err)
		return err
	}
	if err := stream.Close(); err != nil {
		log.Printf("quic send close failed peer_id=%s err=%v", peerID, err)
	}
	log.Printf("quic sent peer_id=%s bytes=%d", peerID, len(msg))
	return nil
}

func (s *ChuteSession) IsConnectedTo(targetID string) bool {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	return s.Connected && s.PeerID == targetID
}

func (s *ChuteSession) IsConnected() bool {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	return s.Connected
}

func (s *ChuteSession) CurrentPeerID() string {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	return s.PeerID
}

func (s *ChuteSession) Listener() *quic.Listener {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	return s.listener
}

func (s *ChuteSession) readLoop(conn quic.Connection) {
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			s.handleDisconnect(err)
			return
		}

		payload, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			log.Printf("quic stream read failed: %v", err)
			continue
		}

		s.Mutex.Lock()
		receiveChan := s.ReceiveChan
		peerID := s.PeerID
		s.Mutex.Unlock()

		log.Printf("quic received peer_id=%s bytes=%d", peerID, len(payload))
		if receiveChan != nil {
			select {
			case receiveChan <- append([]byte(nil), payload...):
			default:
			}
		}
	}
}

func (s *ChuteSession) handshakeDial(conn quic.Connection) error {
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}

	if err := writeLine(stream, s.LocalID); err != nil {
		_ = stream.Close()
		return err
	}

	response, err := readLine(stream)
	_ = stream.Close()
	if err != nil {
		return err
	}
	if response == "busy" {
		return errors.New("busy")
	}
	if response != "accept" {
		return errors.New("handshake failed")
	}
	return nil
}

func (s *ChuteSession) handshakeAccept(conn quic.Connection) (string, error) {
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		return "", err
	}
	defer stream.Close()

	peerID, err := readLine(stream)
	if err != nil {
		return "", err
	}
	if peerID == "" {
		if err := writeLine(stream, "busy"); err != nil {
			return "", err
		}
		return "", errors.New("missing identity")
	}

	if err := writeLine(stream, "accept"); err != nil {
		return "", err
	}
	return peerID, nil
}

func writeLine(stream quic.Stream, value string) error {
	if len(value) > identityLimit {
		return errors.New("identity too long")
	}
	_, err := stream.Write([]byte(value + "\n"))
	return err
}

func readLine(stream quic.Stream) (string, error) {
	limited := &io.LimitedReader{R: stream, N: identityLimit + 2}
	reader := bufio.NewReader(limited)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if len(line) > identityLimit {
		return "", errors.New("identity too long")
	}
	return line, nil
}

func (s *ChuteSession) monitorConnection(conn quic.Connection) {
	<-conn.Context().Done()
	s.handleDisconnect(conn.Context().Err())
}

func (s *ChuteSession) handleDisconnect(err error) {
	s.Mutex.Lock()
	if !s.Connected {
		s.Mutex.Unlock()
		return
	}
	s.conn = nil
	s.Connected = false
	s.PeerID = ""
	s.Mutex.Unlock()

	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		log.Printf("session disconnected")
		return
	}
	log.Printf("session disconnected err=%v", err)
}

func quicConfig() *quic.Config {
	return &quic.Config{
		MaxIdleTimeout:       sessionIdle,
		KeepAlivePeriod:      keepAlive,
		HandshakeIdleTimeout: handshakeIdle,
	}
}

func serverTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{nextProto},
	}
}

func clientTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{nextProto},
	}
}
