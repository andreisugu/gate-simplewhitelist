package simplewhitelist

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/go-logr/logr"
	"github.com/pelletier/go-toml/v2"
	"github.com/robinbraemer/event"
	"go.minekube.com/brigodier"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/common/minecraft/component/codec/legacy"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

const defaultConfigTemplate = `# =============================================================================
#                        Simple Whitelist
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
uuids = [
    # "069a79f4-44e9-4726-a5be-fca90e38aaf5",
]

# Allowed IP addresses or CIDR subnets.
ips = [
    # "127.0.0.1",
    # "192.168.1.50",
    # "10.0.0.0/24",
]
`

type config struct {
	Enabled     bool     `toml:"enabled"`
	KickMessage string   `toml:"kick_message"`
	Nicks       []string `toml:"nicks"`
	UUIDs       []string `toml:"uuids"`
	IPs         []string `toml:"ips"`
}

type whitelistState struct {
	enabled    bool
	nicks      map[string]struct{}
	uuids      map[string]struct{}
	ips        []net.IP
	ipNets     []*net.IPNet
	kickReason c.Component
}

// Plugin is the ultra-lightweight, zero-fluff simple whitelist plugin for Gate proxy.
var Plugin = proxy.Plugin{
	Name: "SimpleWhitelist",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		configPath := filepath.Join("config", "whitelist.toml")

		var state atomic.Pointer[whitelistState]

		loadConfig := func() error {
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				_ = os.MkdirAll(filepath.Dir(configPath), 0o755)
				_ = os.WriteFile(configPath, []byte(defaultConfigTemplate), 0o644)
			}

			data, err := os.ReadFile(configPath)
			if err != nil {
				return err
			}

			var cfg config
			if err := toml.Unmarshal(data, &cfg); err != nil {
				return err
			}

			nickMap := make(map[string]struct{}, len(cfg.Nicks))
			for _, n := range cfg.Nicks {
				clean := strings.ToLower(strings.TrimSpace(n))
				if clean != "" {
					nickMap[clean] = struct{}{}
				}
			}

			uuidMap := make(map[string]struct{}, len(cfg.UUIDs))
			for _, u := range cfg.UUIDs {
				clean := strings.ToLower(strings.TrimSpace(u))
				if clean != "" {
					uuidMap[clean] = struct{}{}
				}
			}

			var parsedIPs []net.IP
			var parsedNets []*net.IPNet
			for _, ipStr := range cfg.IPs {
				clean := strings.TrimSpace(ipStr)
				if clean == "" {
					continue
				}
				if strings.Contains(clean, "/") {
					if _, ipNet, err := net.ParseCIDR(clean); err == nil {
						parsedNets = append(parsedNets, ipNet)
					}
				} else if ip := net.ParseIP(clean); ip != nil {
					parsedIPs = append(parsedIPs, ip)
				}
			}

			legacyCodec := &legacy.Legacy{Char: legacy.AmpersandChar}
			comp, _ := legacyCodec.Unmarshal([]byte(cfg.KickMessage))

			state.Store(&whitelistState{
				enabled:    cfg.Enabled,
				nicks:      nickMap,
				uuids:      uuidMap,
				ips:        parsedIPs,
				ipNets:     parsedNets,
				kickReason: comp,
			})
			return nil
		}

		if err := loadConfig(); err != nil {
			log.Error(err, "Failed to load simple whitelist config")
			return err
		}

		// Zero-lock, lock-free check on player login
		event.Subscribe(p.Event(), 0, func(e *proxy.LoginEvent) {
			if !e.Allowed() {
				return
			}
			st := state.Load()
			if !st.enabled {
				return
			}

			// 0. Permission bypass check (simplewhitelist.bypass or *)
			if e.Player().HasPermission("simplewhitelist.bypass") {
				return
			}

			// 1. O(1) Nick check
			if _, ok := st.nicks[strings.ToLower(e.Player().Username())]; ok {
				return
			}

			// 2. O(1) UUID check
			if _, ok := st.uuids[strings.ToLower(e.Player().ID().String())]; ok {
				return
			}

			// 3. IP / CIDR subnet check
			if len(st.ips) > 0 || len(st.ipNets) > 0 {
				if addr := e.Player().RemoteAddr(); addr != nil {
					host := addr.String()
					if h, _, err := net.SplitHostPort(host); err == nil {
						host = h
					}
					if pip := net.ParseIP(host); pip != nil {
						for _, ip := range st.ips {
							if ip.Equal(pip) {
								return
							}
						}
						for _, netBlock := range st.ipNets {
							if netBlock.Contains(pip) {
								return
							}
						}
					}
				}
			}

			// Player is not whitelisted -> kick
			e.Deny(st.kickReason)
		})

		// Admin command builder with permission requirement (simplewhitelist.admin)
		buildAdminCmd := func(alias string) brigodier.LiteralNodeBuilder {
			return brigodier.Literal(alias).
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return c.Source.HasPermission("simplewhitelist.admin")
				})).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					st := state.Load()
					status := "§cDisabled"
					if st.enabled {
						status = "§aEnabled"
					}
					return cmdCtx.Source.SendMessage(&c.Text{
						Content: fmt.Sprintf("§b[SimpleWhitelist] §7Status: %s §7(Nicks: §f%d§7, UUIDs: §f%d§7, IPs: §f%d§7)\n§7Use §f/%s reload §7to reload config.",
							status, len(st.nicks), len(st.uuids), len(st.ips)+len(st.ipNets), alias),
					})
				})).
				Then(brigodier.Literal("reload").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						if err := loadConfig(); err != nil {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Reload failed: %v", err)})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleWhitelist] Configuration reloaded successfully!"})
					})),
				)
		}

		p.Command().Register(buildAdminCmd("simplewhitelist"))
		p.Command().Register(buildAdminCmd("swl"))

		log.Info("SimpleWhitelist loaded (nicks, uuids, ips, permissions enabled)")
		return nil
	},
}
