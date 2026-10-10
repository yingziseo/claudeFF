package store

import (
	"github.com/yingziseo/claudeFF/internal/model"
	"testing"
)

type fakeVault struct{}

func (fakeVault) Protect(b []byte) ([]byte, error)   { return append([]byte("protected:"), b...), nil }
func (fakeVault) Unprotect(b []byte) ([]byte, error) { return b[10:], nil }
func TestPersistenceAndSingleProfile(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir, fakeVault{})
	if e != nil {
		t.Fatal(e)
	}
	p := model.Proxy{Protocol: "http", Host: "proxy.example.com", Port: 8080, Username: "test", Password: "not-a-real-secret"}
	rev, e := s.SaveProxy(p)
	if e != nil || rev != 1 {
		t.Fatal(rev, e)
	}
	a, e := s.EnsureProfile("code")
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.EnsureProfile("browser")
	if e != nil || a.ID != b.ID || b.Source != "code" {
		t.Fatal("duplicate profile", e)
	}
	s.Close()
	s, e = Open(dir, fakeVault{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	got, r, e := s.Proxy()
	if e != nil || got != p || r != 1 {
		t.Fatal(got, r, e)
	}
	var raw []byte
	s.db.QueryRow("SELECT proxy FROM settings").Scan(&raw)
	if string(raw) == p.Password {
		t.Fatal("raw secret")
	}
	_, e = s.SaveProxy(p)
	if e != nil {
		t.Fatal(e)
	}
	_, r, _ = s.Proxy()
	if r != 2 {
		t.Fatal("revision not incremented")
	}
}
