//go:build darwin

package secret

import (
	"bytes"
	"os"
	"testing"
)

func TestMacKeychainProtectedRoundTrip(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("system Keychain validation is cloud-only")
	}
	v := Vault{}
	plain := []byte("cloud-test-credential")
	a, e := v.Protect(plain)
	if e != nil {
		t.Fatal(e)
	}
	b, e := v.Protect(plain)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(a, plain) || bytes.Equal(a, b) {
		t.Fatal("credential protection failed")
	}
	restored, e := v.Unprotect(a)
	if e != nil || !bytes.Equal(restored, plain) {
		t.Fatal("Keychain round trip failed")
	}
	a[len(a)-1] ^= 1
	if _, e = v.Unprotect(a); e == nil {
		t.Fatal("altered ciphertext accepted")
	}
}
