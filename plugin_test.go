package simplewhitelist

import (
	"net"
	"strings"
	"sync/atomic"
	"testing"

	c "go.minekube.com/common/minecraft/component"
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
	id         uuid.UUID
	username   string
	remoteAddr net.Addr
}

func newMockPlayer(username string, id uuid.UUID, ip string) *mockPlayer {
	return &mockPlayer{
		id:         id,
		username:   username,
		remoteAddr: &mockAddr{addr: ip},
	}
}

func (m *mockPlayer) ID() uuid.UUID         { return m.id }
func (m *mockPlayer) Username() string     { return m.username }
func (m *mockPlayer) RemoteAddr() net.Addr { return m.remoteAddr }

func TestSimpleWhitelistEvaluation(t *testing.T) {
	nicks := map[string]struct{}{
		"steve": {},
		"alex":  {},
	}
	uuids := map[string]struct{}{
		"00000000-0000-0000-0000-000000000001": {},
	}
	ips := []net.IP{net.ParseIP("192.168.1.50")}
	_, cidr, _ := net.ParseCIDR("10.0.0.0/24")
	ipNets := []*net.IPNet{cidr}

	var state atomic.Pointer[whitelistState]
	state.Store(&whitelistState{
		enabled:    true,
		nicks:      nicks,
		uuids:      uuids,
		ips:        ips,
		ipNets:     ipNets,
		kickReason: &c.Text{Content: "Denied"},
	})

	isAllowed := func(p proxy.Player) bool {
		st := state.Load()
		if !st.enabled {
			return true
		}
		if _, ok := st.nicks[strings.ToLower(p.Username())]; ok {
			return true
		}
		if _, ok := st.uuids[strings.ToLower(p.ID().String())]; ok {
			return true
		}
		if len(st.ips) > 0 || len(st.ipNets) > 0 {
			if addr := p.RemoteAddr(); addr != nil {
				host := addr.String()
				if h, _, err := net.SplitHostPort(host); err == nil {
					host = h
				}
				if pip := net.ParseIP(host); pip != nil {
					for _, ip := range st.ips {
						if ip.Equal(pip) {
							return true
						}
					}
					for _, netBlock := range st.ipNets {
						if netBlock.Contains(pip) {
							return true
						}
					}
				}
			}
		}
		return false
	}

	testUUID1, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	randomUUID, _ := uuid.Parse("00000000-0000-0000-0000-000000000099")

	// Nick match
	if !isAllowed(newMockPlayer("Steve", randomUUID, "1.1.1.1:1234")) {
		t.Errorf("Expected Steve allowed by nick")
	}

	// UUID match
	if !isAllowed(newMockPlayer("CustomName", testUUID1, "1.1.1.1:1234")) {
		t.Errorf("Expected CustomName allowed by UUID")
	}

	// Exact IP match
	if !isAllowed(newMockPlayer("Stranger", randomUUID, "192.168.1.50:5555")) {
		t.Errorf("Expected Stranger allowed by exact IP")
	}

	// CIDR match
	if !isAllowed(newMockPlayer("Stranger2", randomUUID, "10.0.0.99:5555")) {
		t.Errorf("Expected Stranger2 allowed by CIDR range")
	}

	// Unlisted player
	if isAllowed(newMockPlayer("Hacker", randomUUID, "123.45.67.89:5555")) {
		t.Errorf("Expected Hacker denied")
	}
}
