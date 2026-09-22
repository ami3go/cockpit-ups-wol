package config

import "testing"

func TestValidHostnameRejectsIPv4Typos(t *testing.T) {
	for _, value := range []string{"192.168.1.300", "192.168.1", "10.0.0.256"} {
		if validHostname(value) {
			t.Errorf("validHostname(%q)=true; malformed IPv4-like value must not pass as DNS", value)
		}
	}
}

func TestValidHostnameAllowsOrdinaryNames(t *testing.T) {
	for _, value := range []string{"nas", "nas.local", "ups-1.example.com", "node42.home.arpa"} {
		if !validHostname(value) {
			t.Errorf("validHostname(%q)=false", value)
		}
	}
}
