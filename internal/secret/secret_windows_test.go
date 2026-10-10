//go:build windows

package secret

import (
	"bytes"
	"testing"
)

func TestUserDPAPI(t *testing.T) {
	vault := Vault{}
	original := []byte("test-fixture-no-production-secret")
	sealed, e := vault.Protect(original)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Equal(sealed, original) {
		t.Fatal("secret remained plaintext")
	}
	plain, e := vault.Unprotect(sealed)
	if e != nil || !bytes.Equal(plain, original) {
		t.Fatal("roundtrip failed", e)
	}
	sealed[0] ^= 0xff
	if _, e = vault.Unprotect(sealed); e == nil {
		t.Fatal("tampering accepted")
	}
}
