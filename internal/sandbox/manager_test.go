package sandbox

import "testing"

func TestInstalledTerminalRemainsStartableAfterStop(t *testing.T) {
	root := t.TempDir()
	m := New(root, "", nil)
	m.mu.Lock()
	m.status.Version = "2.1.287 (Claude Code)"
	m.mu.Unlock()
	if err := m.MarkInstalled(); err != nil {
		t.Fatal(err)
	}
	m.set("running", "terminal")
	m.Stop()
	if state := m.State(); state.Phase != "ready" || state.Version != "2.1.287 (Claude Code)" {
		t.Fatalf("stopping an installed terminal must preserve restart and version: %+v", state)
	}
	reopened := New(root, "", nil)
	reopened.RestoreReady()
	if state := reopened.State(); !reopened.Installed() || state.Phase != "ready" || state.Version != m.State().Version {
		t.Fatalf("reopening must preserve the installed version: %+v", state)
	}
	notInstalled := New(t.TempDir(), "", nil)
	notInstalled.Stop()
	if notInstalled.State().Phase != "idle" {
		t.Fatal("stopping before installation must not mark the terminal ready")
	}
}
