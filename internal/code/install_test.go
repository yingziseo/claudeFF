package code

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOfficialManifestIntegrity(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			body := []byte("official fixture binary")
			h := sha256.Sum256(body)
			digest := hex.EncodeToString(h[:])
			if bad {
				digest = strings.Repeat("0", 64)
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/latest":
					fmt.Fprint(w, "2.1.1")
				case "/2.1.1/manifest.json":
					fmt.Fprintf(w, `{"platforms":{"win32-x64":{"checksum":"%s"}}}`, digest)
				case "/2.1.1/win32-x64/claude.exe":
					w.Write(body)
				default:
					w.WriteHeader(404)
				}
			}))
			defer s.Close()
			p, e := fetchFrom(context.Background(), s.Client(), t.TempDir(), s.URL, "windows", "amd64")
			if bad {
				if e == nil {
					t.Fatal("corrupt binary accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			b, e := os.ReadFile(p)
			if e != nil || string(b) != string(body) {
				t.Fatal("verified binary missing")
			}
		})
	}
}
func TestManagedEnvironment(t *testing.T) {
	t.Setenv("https_proxy", "http://old-proxy.invalid")
	t.Setenv("NO_PROXY", "*")
	v := strings.Join(managedEnv("http://127.0.0.1:12345"), "\n")
	if strings.Contains(v, "old-proxy.invalid") || strings.Contains(v, "NO_PROXY=*\n") {
		t.Fatal("conflicting variables leaked")
	}
	if !strings.Contains(v, "HTTPS_PROXY=http://127.0.0.1:12345") {
		t.Fatal("proxy not provided")
	}
}
