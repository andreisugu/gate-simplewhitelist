package simplewhitelist

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/uuid"
)

type mockAddr struct {
	addr string
}

func (a *mockAddr) Network() string { return "tcp" }
func (a *mockAddr) String() string  { return a.addr }

type mockPlayer struct {
	proxy.Player
	id          uuid.UUID
	username    string
	remoteAddr  net.Addr
	permissions map[string]bool
}

func newMockPlayer(username string, id uuid.UUID, ip string) *mockPlayer {
	return &mockPlayer{
		id:          id,
		username:    username,
		remoteAddr:  &mockAddr{addr: ip},
		permissions: make(map[string]bool),
	}
}

func (m *mockPlayer) ID() uuid.UUID               { return m.id }
func (m *mockPlayer) Username() string           { return m.username }
func (m *mockPlayer) RemoteAddr() net.Addr       { return m.remoteAddr }
func (m *mockPlayer) HasPermission(p string) bool { return m.permissions[p] }

func TestTargetDetection(t *testing.T) {
	// Nick
	tt, can := DetectTargetType("Steve")
	if tt != TargetNick || can != "Steve" {
		t.Fatalf("Expected Steve as TargetNick, got %v (%s)", tt, can)
	}

	// UUID
	validUUID := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	tt, can = DetectTargetType(validUUID)
	if tt != TargetUUID || can != validUUID {
		t.Fatalf("Expected TargetUUID, got %v (%s)", tt, can)
	}

	// IP
	tt, can = DetectTargetType("192.168.1.100")
	if tt != TargetIP || can != "192.168.1.100" {
		t.Fatalf("Expected TargetIP, got %v (%s)", tt, can)
	}

	// CIDR
	tt, can = DetectTargetType("10.0.0.0/24")
	if tt != TargetCIDR || can != "10.0.0.0/24" {
		t.Fatalf("Expected TargetCIDR, got %v (%s)", tt, can)
	}
}

func TestManagerWhitelistAndBlacklist(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "simplewhitelist_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Failed to initialize manager: %v", err)
	}

	testUUID, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	otherUUID, _ := uuid.Parse("00000000-0000-0000-0000-000000000002")

	// 1. Test In-Game Add/Remove Whitelist
	_, _, added, err := mgr.AddWhitelist("Notch")
	if err != nil || !added {
		t.Fatalf("Failed to add Notch to whitelist: %v", err)
	}

	_, _, added, err = mgr.AddWhitelist(testUUID.String())
	if err != nil || !added {
		t.Fatalf("Failed to add UUID to whitelist: %v", err)
	}

	_, _, added, err = mgr.AddWhitelist("192.168.1.50")
	if err != nil || !added {
		t.Fatalf("Failed to add IP to whitelist: %v", err)
	}

	// Duplicate add returns added=false
	_, _, added, err = mgr.AddWhitelist("Notch")
	if err != nil || added {
		t.Fatalf("Expected duplicate Notch add to return false, got %v", added)
	}

	// Verify Whitelist evaluation
	pNotch := newMockPlayer("Notch", otherUUID, "1.1.1.1:1234")
	if ok, _ := mgr.IsWhitelisted(pNotch.Username(), pNotch.ID(), pNotch.RemoteAddr()); !ok {
		t.Errorf("Expected Notch whitelisted")
	}

	pUUID := newMockPlayer("Stranger", testUUID, "1.1.1.1:1234")
	if ok, _ := mgr.IsWhitelisted(pUUID.Username(), pUUID.ID(), pUUID.RemoteAddr()); !ok {
		t.Errorf("Expected testUUID whitelisted")
	}

	pIP := newMockPlayer("Stranger2", otherUUID, "192.168.1.50:5555")
	if ok, _ := mgr.IsWhitelisted(pIP.Username(), pIP.ID(), pIP.RemoteAddr()); !ok {
		t.Errorf("Expected 192.168.1.50 whitelisted")
	}

	pUnlisted := newMockPlayer("Hacker", otherUUID, "99.99.99.99:5555")
	if ok, _ := mgr.IsWhitelisted(pUnlisted.Username(), pUnlisted.ID(), pUnlisted.RemoteAddr()); ok {
		t.Errorf("Expected Hacker blocked by whitelist")
	}

	// 2. Test In-Game Add/Remove Blacklist
	_, _, added, err = mgr.AddBlacklist("BadGuy")
	if err != nil || !added {
		t.Fatalf("Failed to add BadGuy to blacklist: %v", err)
	}

	_, _, added, err = mgr.AddBlacklist("10.0.0.0/24")
	if err != nil || !added {
		t.Fatalf("Failed to add 10.0.0.0/24 to blacklist: %v", err)
	}

	pBad := newMockPlayer("BadGuy", otherUUID, "1.1.1.1:1234")
	if ok, _ := mgr.IsBlacklisted(pBad.Username(), pBad.ID(), pBad.RemoteAddr()); !ok {
		t.Errorf("Expected BadGuy blocked by blacklist")
	}

	pSubnet := newMockPlayer("Someone", otherUUID, "10.0.0.45:1234")
	if ok, _ := mgr.IsBlacklisted(pSubnet.Username(), pSubnet.ID(), pSubnet.RemoteAddr()); !ok {
		t.Errorf("Expected 10.0.0.45 blocked by CIDR blacklist")
	}

	// 3. Test File Persistence across Manager reload
	mgr2 := NewManager(tmpDir)
	if err := mgr2.Load(); err != nil {
		t.Fatalf("Failed to reload from disk: %v", err)
	}

	if ok, _ := mgr2.IsWhitelisted(pNotch.Username(), pNotch.ID(), pNotch.RemoteAddr()); !ok {
		t.Errorf("Expected Notch whitelisted after file reload")
	}
	if ok, _ := mgr2.IsBlacklisted(pBad.Username(), pBad.ID(), pBad.RemoteAddr()); !ok {
		t.Errorf("Expected BadGuy blacklisted after file reload")
	}

	// 4. Test Remove
	_, _, removed, err := mgr2.RemoveWhitelist("Notch")
	if err != nil || !removed {
		t.Fatalf("Expected Notch removed: %v", err)
	}
	if ok, _ := mgr2.IsWhitelisted(pNotch.Username(), pNotch.ID(), pNotch.RemoteAddr()); ok {
		t.Errorf("Expected Notch blocked after removal")
	}

	_, _, removed, err = mgr2.RemoveBlacklist("BadGuy")
	if err != nil || !removed {
		t.Fatalf("Expected BadGuy removed: %v", err)
	}
	if ok, _ := mgr2.IsBlacklisted(pBad.Username(), pBad.ID(), pBad.RemoteAddr()); ok {
		t.Errorf("Expected BadGuy no longer blacklisted after removal")
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "whitelist.toml")); os.IsNotExist(err) {
		t.Errorf("whitelist.toml does not exist")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "blacklist.toml")); os.IsNotExist(err) {
		t.Errorf("blacklist.toml does not exist")
	}
}
