package downloader

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "public IPv4", ip: "8.8.8.8", want: false},
		{name: "loopback IPv4", ip: "127.0.0.1", want: true},
		{name: "private IPv4", ip: "192.168.1.1", want: true},
		{name: "CGNAT IPv4", ip: "100.64.0.1", want: true},
		{name: "loopback IPv6", ip: "::1", want: true},
		{name: "unique local IPv6", ip: "fc00::1", want: true},
		{name: "mapped loopback", ip: "::ffff:127.0.0.1", want: true},
		{name: "unspecified", ip: "0.0.0.0", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isPrivateIP(net.ParseIP(test.ip)); got != test.want {
				t.Fatalf("isPrivateIP(%s) = %t, want %t", test.ip, got, test.want)
			}
		})
	}
}
