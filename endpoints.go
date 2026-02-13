package main

import (
	"bytes"
	"errors"
	"net"
	"os"
	"sort"
	"time"

	"github.com/pion/stun"
)

const (
	defaultSTUNServer   = "stun.l.google.com:19302"
	defaultSTUNServerV6 = "stun.l.google.com:19302"
	stunTimeout         = 3 * time.Second
)

func detectLocalIPs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var ips []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
				continue
			}
			if ip.To4() == nil && ip.To16() == nil {
				continue
			}
			ipStr := ip.String()
			if _, ok := seen[ipStr]; ok {
				continue
			}
			seen[ipStr] = struct{}{}
			ips = append(ips, ipStr)
		}
	}

	if len(ips) == 0 {
		return nil, errors.New("no local IPv4 addresses found")
	}
	sort.Strings(ips)
	return ips, nil
}

func discoverPublicEndpoint(conn *net.UDPConn) (string, int, error) {
	stunServer := os.Getenv("CHUTE_STUN_SERVER")
	if stunServer == "" {
		stunServer = defaultSTUNServer
	}
	return stunBinding(conn, stunServer)
}

func discoverPublicEndpointIPv6() (string, error) {
	stunServer := os.Getenv("CHUTE_STUN_SERVER_V6")
	if stunServer == "" {
		stunServer = defaultSTUNServerV6
	}

	conn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.ParseIP("::"), Port: 0})
	if err != nil {
		return "", err
	}
	defer conn.Close()

	ip, _, err := stunBinding(conn, stunServer)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func stunBinding(conn *net.UDPConn, stunServer string) (string, int, error) {
	stunAddr, err := net.ResolveUDPAddr(conn.LocalAddr().Network(), stunServer)
	if err != nil {
		return "", 0, err
	}

	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if _, err := conn.WriteToUDP(msg.Raw, stunAddr); err != nil {
		return "", 0, err
	}

	buf := make([]byte, 1500)
	deadline := time.Now().Add(stunTimeout)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return "", 0, err
		}
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return "", 0, err
		}

		var res stun.Message
		res.Raw = append([]byte(nil), buf[:n]...)
		if err := res.Decode(); err != nil {
			continue
		}
		if !bytes.Equal(res.TransactionID[:], msg.TransactionID[:]) {
			continue
		}

		var xor stun.XORMappedAddress
		if err := xor.GetFrom(&res); err != nil {
			return "", 0, err
		}
		if xor.IP == nil || xor.Port == 0 {
			return "", 0, errors.New("invalid STUN response")
		}
		_ = conn.SetReadDeadline(time.Time{})
		return xor.IP.String(), xor.Port, nil
	}
}
