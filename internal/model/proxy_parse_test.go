package model

import (
	"strings"
	"testing"
)

func TestProviderClipboardFormats(t *testing.T) {
	for _, tc := range []struct{ input, protocol, host, username, password string }{
		{"proxy.example.com:7085:fixture-user:fixture-password", "http", "proxy.example.com", "fixture-user", "fixture-password"},
		{"http://fixture-user:p%40ss%3Aword@proxy.example.com:7085", "http", "proxy.example.com", "fixture-user", "p@ss:word"},
		{"socks5h://fixture-user:fixture-password@proxy.example.com:7085", "socks5", "proxy.example.com", "fixture-user", "fixture-password"},
		{"fixture-user:fixture-password@proxy.example.com:7085", "http", "proxy.example.com", "fixture-user", "fixture-password"},
		{"[2001:db8::1]:7085:fixture-user:p:a:ss", "http", "2001:db8::1", "fixture-user", "p:a:ss"},
		{" proxy.example.com:7085 ", "http", "proxy.example.com", "", ""},
	} {
		p, err := ParseProxyText(tc.input, "http")
		if err != nil || p.Host != tc.host || p.Port != 7085 || p.Protocol != tc.protocol || p.Username != tc.username || p.Password != tc.password {
			t.Fatalf("clipboard format did not populate the fields correctly: %v", err)
		}
	}
}
func TestMalformedImportsNeverEchoCredentials(t *testing.T) {
	for _, input := range []string{"http://fixture-user:private-fixture@bad%host:7085", "proxy.example.com:invalid:fixture-user:private-fixture", "proxy.example.com:7085:private-fixture"} {
		_, err := ParseProxyText(input, "http")
		if err == nil || strings.Contains(err.Error(), "private-fixture") {
			t.Fatal("invalid import accepted or input leaked")
		}
	}
	if (Proxy{Protocol: "http", Host: "proxy.example.com:7085:fixture-user:private-fixture", Port: 8080}).Validate() == nil {
		t.Fatal("provider credentials must never be stored in the host field")
	}
}
