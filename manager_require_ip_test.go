package simplewhitelist

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.minekube.com/gate/pkg/util/uuid"
)

const requireIPMatchTestConfig = `enabled = true
require_ip_match = true
kick_message = "not authorized"
nicks = ["QA_Resident", "QA_Other"]
uuids = []
ips = ["100.126.65.114/32"]
`

func writeRequireIPMatchConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "whitelist.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write whitelist config: %v", err)
	}
}

func loadRequireIPMatchManager(t *testing.T, body string) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	writeRequireIPMatchConfig(t, dir, body)
	mgr := NewManager(dir)
	if err := mgr.Load(); err != nil {
		t.Fatalf("load whitelist: %v", err)
	}
	return mgr, dir
}

func assertRequireIPMatchEnforced(t *testing.T, mgr *Manager) {
	t.Helper()
	st := mgr.WhitelistState()
	if st == nil {
		t.Fatal("whitelist state is nil")
	}
	if !st.RequireIPMatch {
		t.Fatal("RequireIPMatch was not preserved")
	}

	id, err := uuid.Parse("00000000-0000-0000-0000-000000000003")
	if err != nil {
		t.Fatalf("parse test UUID: %v", err)
	}

	allowed := newMockPlayer("QA_Resident", id, "100.126.65.114:1234")
	if ok, _ := mgr.IsWhitelisted(allowed.Username(), allowed.ID(), allowed.RemoteAddr()); !ok {
		t.Error("expected listed name from listed IP to be allowed")
	}

	wrongName := newMockPlayer("NotAllowed01", id, "100.126.65.114:1234")
	if ok, _ := mgr.IsWhitelisted(wrongName.Username(), wrongName.ID(), wrongName.RemoteAddr()); ok {
		t.Error("expected unlisted name from listed IP to be denied")
	}

	wrongIP := newMockPlayer("QA_Resident", id, "100.94.79.89:1234")
	if ok, _ := mgr.IsWhitelisted(wrongIP.Username(), wrongIP.ID(), wrongIP.RemoteAddr()); ok {
		t.Error("expected listed name from unlisted IP to be denied")
	}
}

func TestRequireIPMatchSurvivesConfigMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manager) error
	}{
		{
			name: "set whitelist enabled",
			mutate: func(m *Manager) error {
				return m.SetWhitelistEnabled(true)
			},
		},
		{
			name: "add whitelist",
			mutate: func(m *Manager) error {
				_, _, added, err := m.AddWhitelist("QA_Added")
				if err != nil {
					return err
				}
				if !added {
					return fmt.Errorf("expected QA_Added to be added")
				}
				return nil
			},
		},
		{
			name: "remove whitelist",
			mutate: func(m *Manager) error {
				_, _, removed, err := m.RemoveWhitelist("QA_Other")
				if err != nil {
					return err
				}
				if !removed {
					return fmt.Errorf("expected QA_Other to be removed")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, dir := loadRequireIPMatchManager(t, requireIPMatchTestConfig)

			assertRequireIPMatchEnforced(t, mgr)

			if err := tt.mutate(mgr); err != nil {
				t.Fatalf("mutation failed: %v", err)
			}
			assertRequireIPMatchEnforced(t, mgr)

			reloaded := NewManager(dir)
			if err := reloaded.Load(); err != nil {
				t.Fatalf("reload whitelist: %v", err)
			}
			assertRequireIPMatchEnforced(t, reloaded)
		})
	}
}

func TestRequireIPMatchFailsClosedWhenConfiguredIPsDoNotParse(t *testing.T) {
	const invalidIPsConfig = `enabled = true
require_ip_match = true
kick_message = "not authorized"
nicks = ["QA_Resident"]
uuids = ["00000000-0000-0000-0000-000000000003"]
ips = ["not-an-ip", "999.999.999.999/33"]
`
	mgr, _ := loadRequireIPMatchManager(t, invalidIPsConfig)

	st := mgr.WhitelistState()
	if st == nil {
		t.Fatal("whitelist state is nil")
	}
	if !st.RequireIPMatch {
		t.Fatal("expected require_ip_match to be enabled")
	}
	if len(st.RawIPs) == 0 {
		t.Fatal("expected configured raw IP entries")
	}
	if len(st.IPs) != 0 || len(st.IPNets) != 0 {
		t.Fatalf("expected no parsed IPs, got %d IPs and %d networks", len(st.IPs), len(st.IPNets))
	}

	id, err := uuid.Parse("00000000-0000-0000-0000-000000000003")
	if err != nil {
		t.Fatalf("parse test UUID: %v", err)
	}

	nameMatch := newMockPlayer("QA_Resident", id, "100.126.65.114:1234")
	if ok, _ := mgr.IsWhitelisted(nameMatch.Username(), nameMatch.ID(), nameMatch.RemoteAddr()); ok {
		t.Error("expected listed name to be denied when configured IPs do not parse")
	}

	uuidMatch := newMockPlayer("AnotherName", id, "100.126.65.114:1234")
	if ok, _ := mgr.IsWhitelisted(uuidMatch.Username(), uuidMatch.ID(), uuidMatch.RemoteAddr()); ok {
		t.Error("expected listed UUID to be denied when configured IPs do not parse")
	}
}
