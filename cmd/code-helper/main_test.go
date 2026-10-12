package main

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestRelayRequiresNumericLoopbackAndExplicitPort(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:3210", "http://[::1]:3210"} {
		if err := validateRelay(raw); err != nil {
			t.Fatal(raw, err)
		}
	}
	for _, raw := range []string{"", "https://127.0.0.1:1", "http://example.com:80", "http://127.0.0.1", "http://127.0.0.1:0", "http://u:p@127.0.0.1:80", "http://127.0.0.1:80/path", "http://127.0.0.1:80?x=1", "http://127.0.0.1:80#x"} {
		if validateRelay(raw) == nil {
			t.Fatal("accepted invalid relay", raw)
		}
	}
}
func TestStdioLifecycleAndProjectPersistence(t *testing.T) {
	root, project := t.TempDir(), t.TempDir()
	for iteration := 0; iteration < 2; iteration++ {
		inReader, inWriter := io.Pipe()
		outReader, outWriter := io.Pipe()
		done := make(chan error, 1)
		go func() { done <- serve(context.Background(), root, inReader, outWriter); outWriter.Close() }()
		enc, dec := json.NewEncoder(inWriter), json.NewDecoder(outReader)
		if iteration == 0 {
			if err := enc.Encode(request{ID: 1, Action: "project", Project: project}); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := enc.Encode(request{ID: 1, Action: "state"}); err != nil {
				t.Fatal(err)
			}
		}
		var reply struct {
			ID    uint64 `json:"id"`
			Error string `json:"error"`
			Data  struct {
				Project string `json:"project"`
			} `json:"data"`
		}
		if err := dec.Decode(&reply); err != nil {
			t.Fatal(err)
		}
		if reply.ID != 1 || reply.Error != "" || reply.Data.Project != project {
			t.Fatalf("unexpected reply: %+v", reply)
		}
		inWriter.Close()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		outReader.Close()
	}
}
