package wol

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestSenderWakeDeliversMagicPacket(t *testing.T) {
	ln, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.LocalAddr().(*net.UDPAddr).Port

	const macText = "AA:BB:CC:DD:EE:FF"
	if err := (Sender{}).Wake(context.Background(), Request{
		MAC:       macText,
		Broadcast: "127.0.0.1",
		Port:      port,
	}); err != nil {
		t.Fatalf("Wake: %v", err)
	}

	buf := make([]byte, 256)
	_ = ln.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := ln.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("no packet received: %v", err)
	}
	mac, err := ParseMAC(macText)
	if err != nil {
		t.Fatal(err)
	}
	want, err := MagicPacket(mac)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("payload mismatch: got %d bytes want %d", n, len(want))
	}
}

func TestSenderWakeRejectsIPv6Destination(t *testing.T) {
	err := (Sender{}).Wake(context.Background(), Request{
		MAC:       "AA:BB:CC:DD:EE:FF",
		Broadcast: "::1",
		Port:      9,
	})
	if err == nil {
		t.Fatal("expected IPv6 destination rejection")
	}
}

func TestSenderWakeHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (Sender{}).Wake(ctx, Request{
		MAC:       "AA:BB:CC:DD:EE:FF",
		Broadcast: "127.0.0.1",
		Port:      9,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
}
