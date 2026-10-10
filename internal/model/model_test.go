package model

import "testing"

func TestProxyValidation(t *testing.T) {
	for _, p := range []Proxy{{Protocol: "http", Host: "proxy.example.com", Port: 8080}, {Protocol: "socks5", Host: "::1", Port: 1080}} {
		if e := p.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range []Proxy{{Protocol: "file", Host: "a", Port: 1}, {Protocol: "http", Host: "user@host", Port: 80}, {Protocol: "http", Host: "host", Port: 0}} {
		if p.Validate() == nil {
			t.Fatal("invalid proxy accepted")
		}
	}
}
func TestStableNewProfile(t *testing.T) {
	a, b := NewProfile("browser"), NewProfile("code")
	if a.ID == b.ID || a.Seed == b.Seed {
		t.Fatal("profiles not independent")
	}
	if a.Validate() != nil {
		t.Fatal("default profile invalid")
	}
}
