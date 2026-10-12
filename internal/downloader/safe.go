package downloader

import (
	"net"
)

var privateCIDRs []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"169.254.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
		"100.64.0.0/10",
	}
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			privateCIDRs = append(privateCIDRs, n)
		}
	}
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsUnspecified() {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		return true
	}
	for _, n := range privateCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
