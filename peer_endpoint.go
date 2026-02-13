package main

import "net"

// PeerEndpoint describes a UDP host:port endpoint for QUIC.
type PeerEndpoint struct {
	IP   string
	Port int
}

// Helpers
func endpointFromNetAddr(addr net.Addr) (PeerEndpoint, error) {
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return PeerEndpoint{}, err
	}
	port, err := net.LookupPort("udp", portStr)
	if err != nil {
		return PeerEndpoint{}, err
	}
	return PeerEndpoint{IP: host, Port: port}, nil
}

