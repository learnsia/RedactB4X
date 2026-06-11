package redactorpii

import "testing"

func TestListenAddr(t *testing.T) {
	tests := []struct {
		host, port, want string
	}{
		{"", "8090", ":8090"},
		{"0.0.0.0", "8090", ":8090"},
		{"127.0.0.1", "8090", "127.0.0.1:8090"},
		{"::1", "8091", "[::1]:8091"},
	}
	for _, tc := range tests {
		if got := ListenAddr(tc.host, tc.port); got != tc.want {
			t.Errorf("ListenAddr(%q, %q) = %q, want %q", tc.host, tc.port, got, tc.want)
		}
	}
}

func TestDisplayHost(t *testing.T) {
	if got := DisplayHost(""); got != "localhost" {
		t.Fatalf("DisplayHost empty = the %q", got)
	}
	if got := DisplayHost("192.168.1.5"); got != "192.168.1.5" {
		t.Fatalf("DisplayHost IP: got %q", got)
	}
}
