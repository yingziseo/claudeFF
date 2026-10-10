package licensing

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestActivationRevocationAndPrivacy(t *testing.T) {
	admin := strings.Repeat("test-token-", 4)
	s, e := Open(filepath.Join(t.TempDir(), "license.db"), admin)
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	now := time.Date(2026, 10, 10, 1, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	call := func(method, path, token string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		q := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if token != "" {
			q.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, q)
		return w
	}
	if call("GET", "/v1/admin/cards", "", nil).Code != 401 {
		t.Fatal("admin unprotected")
	}
	created := call("POST", "/v1/admin/cards", admin, map[string]string{"Label": "test"})
	if created.Code != 201 {
		t.Fatal(created.Body.String())
	}
	var card struct {
		ID   int
		Card string
	}
	json.Unmarshal(created.Body.Bytes(), &card)
	register := map[string]string{"Username": "用户01", "Password": "good-password", "Card": card.Card}
	w := call("POST", "/v1/register", "", register)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var session struct{ Token string }
	json.Unmarshal(w.Body.Bytes(), &session)
	if call("POST", "/v1/register", "", map[string]string{"Username": "用户02", "Password": "good-password", "Card": card.Card}).Code == 200 {
		t.Fatal("card activated twice")
	}
	heartbeat := map[string]any{"version": "0.2.0", "browser": true, "code": false}
	if call("POST", "/v1/heartbeat", session.Token, heartbeat).Code != 200 {
		t.Fatal("valid session rejected")
	}
	users := call("GET", "/v1/admin/users", admin, nil)
	if !strings.Contains(users.Body.String(), `"online":true`) {
		t.Fatal(users.Body.String())
	}
	now = now.Add(4 * time.Minute)
	users = call("GET", "/v1/admin/users", admin, nil)
	if strings.Contains(users.Body.String(), `"online":true`) {
		t.Fatal("online stale")
	}
	list := call("GET", "/v1/admin/cards", admin, nil)
	if strings.Contains(list.Body.String(), card.Card) || strings.Contains(users.Body.String(), "good-password") {
		t.Fatal("secret leaked")
	}
	if call("POST", "/v1/admin/revoke", admin, map[string]int{"ID": card.ID}).Code != 200 {
		t.Fatal("revoke failed")
	}
	if call("POST", "/v1/heartbeat", session.Token, heartbeat).Code != 403 {
		t.Fatal("revoked card still active")
	}
	if call("POST", "/v1/login", "", map[string]string{"Username": "用户01", "Password": "good-password"}).Code != 403 {
		t.Fatal("revoked login allowed")
	}
}
func TestClientRejectsInsecureEndpoint(t *testing.T) {
	if _, e := NewClient("http://example.com", ""); e == nil {
		t.Fatal("insecure server accepted")
	}
	if _, e := NewClient("https://user:password@example.com", ""); e == nil {
		t.Fatal("credentials embedded in URL")
	}
}
