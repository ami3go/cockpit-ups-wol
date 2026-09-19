package wol

import (
	"errors"
	"net"
)

const MagicPacketSize = 102

func ParseMAC(s string) (net.HardwareAddr, error) {
	mac, err := net.ParseMAC(s)
	if err != nil {
		return nil, err
	}
	if len(mac) != 6 {
		return nil, errors.New("Wake-on-LAN requires a 6-byte MAC address")
	}
	return mac, nil
}

func MagicPacket(mac net.HardwareAddr) ([]byte, error) {
	if len(mac) != 6 {
		return nil, errors.New("Wake-on-LAN requires a 6-byte MAC address")
	}
	packet := make([]byte, MagicPacketSize)
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	off := 6
	for i := 0; i < 16; i++ {
		copy(packet[off:off+6], mac)
		off += 6
	}
	return packet, nil
}
