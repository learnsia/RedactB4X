package redactorpii

import "strings"

// ListenAddr builds an http.Server Addr from host and port.
// Empty host or "0.0.0.0" listens on all interfaces (":port").
func ListenAddr(host, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if port == "" {
		port = "8090"
	}
	if host == "" || host == "0.0.0.0" {
		return ":" + port
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + port
}

// DisplayHost returns a browser-friendly host label for startup messages.
func DisplayHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" {
		return "localhost"
	}
	return strings.Trim(host, "[]")
}
