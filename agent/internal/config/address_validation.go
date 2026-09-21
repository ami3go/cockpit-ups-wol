package config

import (
	"net"
	"regexp"
	"strings"
)

var hostnameLabelPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func validateAddress(problems *[]string, path string, addr *string) {
	if nilOrEmpty(addr) {
		return
	}
	v := strings.TrimSpace(*addr)
	if net.ParseIP(v) != nil || validHostname(v) {
		return
	}
	*problems = append(*problems, path+" must be an IP address or DNS hostname")
}

func validateIPv4(problems *[]string, path string, value *string) {
	if nilOrEmpty(value) {
		*problems = append(*problems, path+" required when wake is enabled")
		return
	}
	ip := net.ParseIP(strings.TrimSpace(*value))
	if ip == nil || ip.To4() == nil {
		*problems = append(*problems, path+" must be an IPv4 address")
	}
}

func validHostname(v string) bool {
	v = strings.TrimSuffix(v, ".")
	if v == "" || len(v) > 253 {
		return false
	}
	for _, label := range strings.Split(v, ".") {
		if !hostnameLabelPattern.MatchString(label) {
			return false
		}
	}
	return true
}
