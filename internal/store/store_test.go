package store

import (
	"bytes"
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
	if bytes.Contains(raw, []byte(p.Password)) {
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

func TestFutureSchemaIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir, fakeVault{})
	if e != nil {
		t.Fatal(e)
	}
	s.db.Exec("PRAGMA user_version=99")
	s.Close()
	if _, e = Open(dir, fakeVault{}); e == nil {
		t.Fatal("newer schema accepted")
	}
}

func TestLegacyProviderHostnameIsRepairedWithoutLeakingSecret(t *testing.T) {
	s, e := Open(t.TempDir(), fakeVault{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.db.Exec(`INSERT INTO settings(id,proxy,revision) VALUES(1,?,1)`, []byte(`{"protocol":"http","host":"198.51.100.8:7085:legacy-user:legacy-test-secret","port":8080,"username":""}`)); e != nil {
		t.Fatal(e)
	}
	p, rev, e := s.Proxy()
	if e != nil || p.Host != "198.51.100.8" || p.Port != 7085 || p.Username != "legacy-user" || p.Password != "legacy-test-secret" || rev != 2 {
		t.Fatal("legacy provider format was not repaired")
	}
	var raw, secret []byte
	if e = s.db.QueryRow("SELECT proxy,secret FROM settings WHERE id=1").Scan(&raw, &secret); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("legacy-test-secret")) || !bytes.HasPrefix(secret, []byte("protected:")) {
		t.Fatal("legacy authentication text exposed")
	}
	_, rev, e = s.Proxy()
	if e != nil || rev != 2 {
		t.Fatal("migration repeated")
	}
}
