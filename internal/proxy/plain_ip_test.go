package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeFreeIPResponses(t *testing.T) {
	for _, tc := range []struct{ body, ip string }{
		{"203.0.113.7\n", "203.0.113.7"},
		{" 2001:db8::2\r\n", "2001:db8::2"},
		{`{"ip":"203.0.113.7"}`, "203.0.113.7"},
		{"<html>503 Service Unavailable</html>", ""},
		{"203.0.113.7, 203.0.113.8", ""},
		{`{"ip":"not an IP"}`, ""},
	} {
		got, err := DecodeIP([]byte(tc.body))
		if got != tc.ip || (err != nil) != (tc.ip == "") {
			t.Fatalf("body %q: got %q, error %v", tc.body, got, err)
		}
	}
}

func TestUnavailableMetadataDoesNotInvalidatePlainIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ip":
			w.Write([]byte("203.0.113.7\n"))
		default:
			w.WriteHeader(503)
		}
	}))
	defer server.Close()
	o, err := DetectAcross(context.Background(), server.Client(), []string{server.URL + "/unavailable", server.URL + "/ip"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	GeoWith(context.Background(), server.Client(), server.URL+"/geo", o)
	if o.IP != "203.0.113.7" || o.CountryCode != "" || o.Revision != 3 {
		t.Fatalf("unexpected observation: %+v", o)
	}
}
