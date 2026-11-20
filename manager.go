package simplewhitelist

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pelletier/go-toml/v2"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/common/minecraft/component/codec/legacy"
	"go.minekube.com/gate/pkg/util/uuid"
)

// TargetType identifies the kind of entity added or removed.
type TargetType string

const (
	TargetNick TargetType = "Nick"
	TargetUUID TargetType = "UUID"
	TargetIP   TargetType = "IP"
	TargetCIDR TargetType = "CIDR"
)

// ListConfig represents the TOML file structure for whitelist.
type ListConfig struct {
	Enabled        bool     `toml:"enabled"`
	KickMessage    string   `toml:"kick_message"`
	RequireIPMatch bool     `toml:"require_ip_match"`
	Nicks          []string `toml:"nicks"`
	UUIDs          []string `toml:"uuids"`
	IPs            []string `toml:"ips"`
}

// ListState holds parsed, fast O(1) in-memory structures for login evaluations.
type ListState struct {
	Enabled        bool
	KickMessage    string
	RequireIPMatch bool
	Nicks          map[string]struct{}
	UUIDs          map[string]struct{}
	IPs            []net.IP
	IPNets         []*net.IPNet
	RawNicks       []string
	RawUUIDs       []string
	RawIPs         []string
	KickReason     c.Component
}

const defaultWhitelistTemplate = `# =============================================================================
#                        Simple Whitelist Configuration
# =============================================================================
# Set to true to block anyone not listed in nicks, uuids, or ips below.
enabled = true

# Disconnect message shown to unwhitelisted players.
kick_message = "&cYou are not whitelisted on this server!"

# Allowed player nicknames (case-insensitive).
nicks = [
    "Steve",
    "Alex",
]

# Allowed player UUIDs.
uuids = []

# Allowed IP addresses or CIDR subnets.
ips = []
`

// Manager handles concurrent in-memory caching and persistent TOML storage for whitelist.
type Manager struct {
	whitelistPath  string
	mu             sync.Mutex
	whitelistState atomic.Pointer[ListState]
	legacyCodec    *legacy.Legacy
}

// NewManager initializes a Manager instance.
func NewManager(configDir string) *Manager {
	return &Manager{
		whitelistPath: filepath.Join(configDir, "whitelist.toml"),
		legacyCodec:   &legacy.Legacy{Char: legacy.AmpersandChar},
	}
}

// Load reads whitelist.toml from disk.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(m.whitelistPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(m.whitelistPath), 0o755)
		_ = os.WriteFile(m.whitelistPath, []byte(defaultWhitelistTemplate), 0o644)
	}

	data, err := os.ReadFile(m.whitelistPath)
	if err != nil {
		return fmt.Errorf("failed to read whitelist file: %w", err)
	}

	var cfg ListConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse whitelist.toml: %w", err)
	}

	m.whitelistState.Store(m.compileState(&cfg))
	return nil
}

func (m *Manager) compileState(cfg *ListConfig) *ListState {
	nickMap := make(map[string]struct{}, len(cfg.Nicks))
	rawNicks := make([]string, 0, len(cfg.Nicks))
	for _, n := range cfg.Nicks {
		clean := strings.TrimSpace(n)
		if clean != "" {
			nickMap[strings.ToLower(clean)] = struct{}{}
			rawNicks = append(rawNicks, clean)
		}
	}

	uuidMap := make(map[string]struct{}, len(cfg.UUIDs))
	rawUUIDs := make([]string, 0, len(cfg.UUIDs))
	for _, u := range cfg.UUIDs {
		clean := strings.TrimSpace(u)
		if clean != "" {
			uuidMap[strings.ToLower(clean)] = struct{}{}
			rawUUIDs = append(rawUUIDs, clean)
		}
	}

	var parsedIPs []net.IP
	var parsedNets []*net.IPNet
	rawIPs := make([]string, 0, len(cfg.IPs))
	for _, ipStr := range cfg.IPs {
		clean := strings.TrimSpace(ipStr)
		if clean == "" {
			continue
		}
		rawIPs = append(rawIPs, clean)
		if strings.Contains(clean, "/") {
			if _, ipNet, err := net.ParseCIDR(clean); err == nil {
				parsedNets = append(parsedNets, ipNet)
			}
		} else if ip := net.ParseIP(clean); ip != nil {
			parsedIPs = append(parsedIPs, ip)
		}
	}

	kickComp, _ := m.legacyCodec.Unmarshal([]byte(cfg.KickMessage))
	if kickComp == nil {
		kickComp = &c.Text{Content: cfg.KickMessage}
	}

	return &ListState{
		Enabled:        cfg.Enabled,
		KickMessage:    cfg.KickMessage,
		RequireIPMatch: cfg.RequireIPMatch,
		Nicks:          nickMap,
		UUIDs:          uuidMap,
		IPs:            parsedIPs,
		IPNets:         parsedNets,
		RawNicks:       rawNicks,
		RawUUIDs:       rawUUIDs,
		RawIPs:         rawIPs,
		KickReason:     kickComp,
	}
}

// DetectTargetType classifies an input string into IP, CIDR, UUID, or Nick.
func DetectTargetType(input string) (TargetType, string) {
	trimmed := strings.TrimSpace(input)
	if strings.Contains(trimmed, "/") {
		if _, _, err := net.ParseCIDR(trimmed); err == nil {
			return TargetCIDR, trimmed
		}
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		return TargetIP, trimmed
	}
	if parsedUUID, err := uuid.Parse(trimmed); err == nil {
		return TargetUUID, parsedUUID.String()
	}
	return TargetNick, trimmed
}

// WhitelistState returns the current active whitelist state.
func (m *Manager) WhitelistState() *ListState {
	return m.whitelistState.Load()
}

// SetWhitelistEnabled enables or disables the whitelist and saves to disk.
func (m *Manager) SetWhitelistEnabled(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := m.whitelistState.Load()
	cfg := &ListConfig{
		Enabled:        enabled,
		KickMessage:    st.KickMessage,
		RequireIPMatch: st.RequireIPMatch,
		Nicks:          st.RawNicks,
		UUIDs:          st.RawUUIDs,
		IPs:            st.RawIPs,
	}
	if err := m.saveFileLocked(m.whitelistPath, cfg); err != nil {
		return err
	}
	m.whitelistState.Store(m.compileState(cfg))
	return nil
}

// AddWhitelist adds an entry to the whitelist and persists to disk.
func (m *Manager) AddWhitelist(rawTarget string) (TargetType, string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tType, canonical := DetectTargetType(rawTarget)
	st := m.whitelistState.Load()
	cfg := &ListConfig{
		Enabled:        st.Enabled,
		KickMessage:    st.KickMessage,
		RequireIPMatch: st.RequireIPMatch,
		Nicks:          append([]string{}, st.RawNicks...),
		UUIDs:          append([]string{}, st.RawUUIDs...),
		IPs:            append([]string{}, st.RawIPs...),
	}

	alreadyExists := false
	switch tType {
	case TargetNick:
		for _, n := range cfg.Nicks {
			if strings.EqualFold(n, canonical) {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			cfg.Nicks = append(cfg.Nicks, canonical)
		}
	case TargetUUID:
		for _, u := range cfg.UUIDs {
			if strings.EqualFold(u, canonical) {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			cfg.UUIDs = append(cfg.UUIDs, canonical)
		}
	case TargetIP, TargetCIDR:
		for _, ip := range cfg.IPs {
			if strings.EqualFold(ip, canonical) {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			cfg.IPs = append(cfg.IPs, canonical)
		}
	}

	if alreadyExists {
		return tType, canonical, false, nil
	}

	if err := m.saveFileLocked(m.whitelistPath, cfg); err != nil {
		return tType, canonical, false, err
	}
	m.whitelistState.Store(m.compileState(cfg))
	return tType, canonical, true, nil
}

// RemoveWhitelist removes an entry from the whitelist and persists to disk.
func (m *Manager) RemoveWhitelist(rawTarget string) (TargetType, string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tType, canonical := DetectTargetType(rawTarget)
	st := m.whitelistState.Load()
	cfg := &ListConfig{
		Enabled:        st.Enabled,
		KickMessage:    st.KickMessage,
		RequireIPMatch: st.RequireIPMatch,
		Nicks:          make([]string, 0, len(st.RawNicks)),
		UUIDs:          make([]string, 0, len(st.RawUUIDs)),
		IPs:            make([]string, 0, len(st.RawIPs)),
	}

	removed := false
	for _, n := range st.RawNicks {
		if strings.EqualFold(n, canonical) {
			removed = true
		} else {
			cfg.Nicks = append(cfg.Nicks, n)
		}
	}
	for _, u := range st.RawUUIDs {
		if strings.EqualFold(u, canonical) {
			removed = true
		} else {
			cfg.UUIDs = append(cfg.UUIDs, u)
		}
	}
	for _, ip := range st.RawIPs {
		if strings.EqualFold(ip, canonical) {
			removed = true
		} else {
			cfg.IPs = append(cfg.IPs, ip)
		}
	}

	if !removed {
		return tType, canonical, false, nil
	}

	if err := m.saveFileLocked(m.whitelistPath, cfg); err != nil {
		return tType, canonical, false, err
	}
	m.whitelistState.Store(m.compileState(cfg))
	return tType, canonical, true, nil
}

// IsWhitelisted checks if a player is allowed by the whitelist.
func (m *Manager) IsWhitelisted(username string, id uuid.UUID, remoteAddr net.Addr) (bool, c.Component) {
	st := m.whitelistState.Load()
	if st == nil || !st.Enabled {
		return true, nil
	}

	_, nickMatch := st.Nicks[strings.ToLower(username)]
	_, uuidMatch := st.UUIDs[strings.ToLower(id.String())]

	ipMatch := false
	if remoteAddr != nil && (len(st.IPs) > 0 || len(st.IPNets) > 0) {
		host := remoteAddr.String()
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if pip := net.ParseIP(host); pip != nil {
			for _, ip := range st.IPs {
				if ip.Equal(pip) {
					ipMatch = true
					break
				}
			}
			if !ipMatch {
				for _, netBlock := range st.IPNets {
					if netBlock.Contains(pip) {
						ipMatch = true
						break
					}
				}
			}
		}
	}

	// require_ip_match turns the legacy OR semantics into an AND for the
	// configured identity and source IP. Gate on configured entries, not
	// successfully parsed ones: malformed configured IPs must fail closed.
	if st.RequireIPMatch && len(st.RawIPs) > 0 {
		if (nickMatch || uuidMatch) && ipMatch {
			return true, nil
		}
		return false, st.KickReason
	}

	if nickMatch || uuidMatch || ipMatch {
		return true, nil
	}
	return false, st.KickReason
}

func (m *Manager) saveFileLocked(path string, cfg *ListConfig) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
