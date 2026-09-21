package wol

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

type Request struct {
	MAC       string
	Interface string
	Broadcast string
	Port      int
}

type Sender struct {
	Timeout time.Duration
}

func (s Sender) Wake(ctx context.Context, req Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mac, err := ParseMAC(req.MAC)
	if err != nil {
		return fmt.Errorf("parse MAC: %w", err)
	}
	packet, err := MagicPacket(mac)
	if err != nil {
		return err
	}
	if req.Port == 0 {
		req.Port = 9
	}
	if req.Port < 1 || req.Port > 65535 {
		return errors.New("invalid UDP port")
	}
	dstIP := net.ParseIP(req.Broadcast)
	if dstIP == nil || dstIP.To4() == nil {
		return errors.New("broadcast must be an IPv4 address")
	}

	srcIP := net.IPv4zero
	if req.Interface != "" {
		srcIP, err = interfaceIPv4(req.Interface)
		if err != nil {
			return err
		}
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: srcIP, Port: 0})
	if err != nil {
		return fmt.Errorf("open UDP socket: %w", err)
	}
	defer conn.Close()

	raw, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("get UDP raw connection: %w", err)
	}
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return fmt.Errorf("set UDP broadcast: %w", err)
	}
	if sockErr != nil {
		return fmt.Errorf("set UDP broadcast: %w", sockErr)
	}

	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	_, err = conn.WriteToUDP(packet, &net.UDPAddr{IP: dstIP, Port: req.Port})
	if err != nil {
		return fmt.Errorf("send magic packet: %w", err)
	}
	return nil
}

func interfaceIPv4(name string) (net.IP, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("interface %q: %w", name, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("interface %q addresses: %w", name, err)
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() {
			return ip4, nil
		}
	}
	return nil, fmt.Errorf("interface %q has no non-loopback IPv4 address", name)
}
