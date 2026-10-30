// Package control defines the closed gateway management protocol.
package control

import (
	"net/netip"
	"os"
	"strings"
)

type Grant struct {
	IP   string `json:"ip"`
	Role string `json:"role"`
	TTL  int    `json:"ttl"`
}

var Ports = map[string][]int{
	"student": {80},
	"teacher": {80, 8081},
	"admin":   {80, 8081, 8082},
}

// Only the two simulated guest devices can acquire grants in this lab.
func ClientIP(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	return err == nil && (addr.String() == "172.28.10.10" || addr.String() == "172.28.10.11")
}

func Secret(path string) (string, error) {
	data, err := os.ReadFile(path)
	return strings.TrimSpace(string(data)), err
}
