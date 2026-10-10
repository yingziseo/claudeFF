//go:build windows

package main

import (
	"context"
	"encoding/json"
	"github.com/yingziseo/claudeFF/internal/code"
	"github.com/yingziseo/claudeFF/internal/licensing"
	"github.com/yingziseo/claudeFF/internal/secret"
	"github.com/yingziseo/claudeFF/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionBindingsEnforceActivationAndRevocation(t *testing.T) {
	old := releaseMode
	releaseMode = "production"
	defer func() { releaseMode = old }()
	root := t.TempDir()
	db, e := store.Open(root, secret.Vault{})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	server, e := licensing.Open(filepath.Join(t.TempDir(), "license.db"), strings.Repeat("test-admin", 4))
	if e != nil {
		t.Fatal(e)
	}
	defer server.DB.Close()
	web := httptest.NewServer(server.Handler())
	defer web.Close()
	client, e := licensing.NewClient(web.URL, "")
	if e != nil {
		t.Fatal(e)
	}
	app := &App{ctx: context.Background(), root: root, db: db, code: code.New(root, nil), license: client}
	if _, e = app.Invoke("browser.start", "{}"); e == nil {
		t.Fatal("production feature bypass")
	}
	r := httptest.NewRequest("POST", "/v1/admin/cards", strings.NewReader(`{"Label":"native-test"}`))
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("test-admin", 4))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var card struct {
		ID   int
		Card string
	}
	json.Unmarshal(w.Body.Bytes(), &card)
	body, _ := json.Marshal(map[string]string{"Username": "nativeuser", "Password": "native-test-password", "Card": card.Card})
	if _, e = app.Invoke("auth.register", string(body)); e != nil {
		t.Fatal(e)
	}
	raw, e := app.snapshot()
	if e != nil || !strings.Contains(raw, `"authorized":true`) {
		t.Fatal(raw, e)
	}
	saved, e := db.LicenseToken()
	if e != nil || saved == "" {
		t.Fatal("persistent session missing", e)
	}
	r = httptest.NewRequest(http.MethodPost, "/v1/admin/revoke", strings.NewReader(`{"ID":1}`))
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("test-admin", 4))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	app.checkLicense()
	if _, e = app.Invoke("profile.save", `{"Language":"en-US","Timezone":"UTC"}`); e == nil {
		t.Fatal("revoked feature still allowed")
	}
}
