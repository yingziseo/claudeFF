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
func TestSessionExpiryAndLogout(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "license.db"), strings.Repeat("admin-fixture", 3))
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	now := time.Now().UTC()
	s.Now = func() time.Time { return now }
	s.DB.Exec("INSERT INTO cards(hash,label,created) VALUES(?,?,?)", hash("test-card"), "test", now.Format(time.RFC3339))
	request := func(path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	w := request("/v1/register", "", `{"Username":"testuser","Password":"test-password","Card":"test-card"}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var session struct{ Token string }
	json.Unmarshal(w.Body.Bytes(), &session)
	if request("/v1/login", "", `{"Username":"testuser","Password":"wrong"}`).Code != 401 {
		t.Fatal("bad password accepted")
	}
	request("/v1/logout", session.Token, `{}`)
	if request("/v1/heartbeat", session.Token, `{}`).Code != 403 {
		t.Fatal("logout ignored")
	}
	w = request("/v1/login", "", `{"Username":"testuser","Password":"test-password"}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &session)
	now = now.Add(15 * 24 * time.Hour)
	if request("/v1/heartbeat", session.Token, `{}`).Code != 403 {
		t.Fatal("expired session accepted")
	}
}
func TestAdminPaginationAndSearch(t *testing.T) {
	admin := strings.Repeat("fixture-admin", 3)
	s, e := Open(filepath.Join(t.TempDir(), "license.db"), admin)
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	tx, _ := s.DB.Begin()
	for i := 0; i < 105; i++ {
		_, e = tx.Exec("INSERT INTO cards(hash,label,created) VALUES(?,?,?)", random(16), "批量测试", time.Now().UTC().Format(time.RFC3339))
		if e != nil {
			t.Fatal(e)
		}
	}
	tx.Commit()
	query := func(path string) []map[string]any {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+admin)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var items []map[string]any
		json.Unmarshal(w.Body.Bytes(), &items)
		return items
	}
	if len(query("/v1/admin/cards")) != 100 || len(query("/v1/admin/cards?offset=100")) != 5 {
		t.Fatal("pagination lost data")
	}
	if len(query("/v1/admin/cards?q=absent")) != 0 {
		t.Fatal("search not applied to database")
	}
}
