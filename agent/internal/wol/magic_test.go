package wol

import (
	"bytes"
	"testing"
)

func TestMagicPacket(t *testing.T) {
	mac, err := ParseMAC("01:23:45:67:89:ab")
	if err != nil {
		t.Fatal(err)
	}
	packet, err := MagicPacket(mac)
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) != MagicPacketSize {
		t.Fatalf("len=%d want %d", len(packet), MagicPacketSize)
	}
	if !bytes.Equal(packet[:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) {
		t.Fatalf("bad sync stream: %x", packet[:6])
	}
	for i := 0; i < 16; i++ {
		got := packet[6+i*6 : 12+i*6]
		if !bytes.Equal(got, mac) {
			t.Fatalf("copy %d=%x want %x", i, got, mac)
		}
	}
}

func TestParseMACRejectsNonEthernetLength(t *testing.T) {
	if _, err := ParseMAC("01:23:45:67:89:ab:cd:ef"); err == nil {
		t.Fatal("expected 8-byte MAC to be rejected")
	}
}
