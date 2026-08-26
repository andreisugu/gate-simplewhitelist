package simplewhitelist

import (
	"fmt"
	"net"
	"strings"

	"go.minekube.com/brigodier"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// SuggestionFunc adapts a function into a brigodier.SuggestionProvider.
type SuggestionFunc func(ctx *brigodier.CommandContext, builder *brigodier.SuggestionsBuilder) *brigodier.Suggestions

// Suggestions implements brigodier.SuggestionProvider.
func (f SuggestionFunc) Suggestions(ctx *brigodier.CommandContext, builder *brigodier.SuggestionsBuilder) *brigodier.Suggestions {
	return f(ctx, builder)
}

func suggestCandidates(getCandidates func() []string) SuggestionFunc {
	return func(ctx *brigodier.CommandContext, builder *brigodier.SuggestionsBuilder) *brigodier.Suggestions {
		input := builder.Input
		wordStart := builder.Start
		for wordStart < len(input) && input[wordStart] == ' ' {
			wordStart++
		}
		prefix := ""
		if wordStart < len(input) {
			prefix = strings.ToLower(input[wordStart:])
		}

		var results []*brigodier.Suggestion
		seen := make(map[string]struct{})
		for _, candidate := range getCandidates() {
			if _, exists := seen[candidate]; exists {
				continue
			}
			if strings.HasPrefix(strings.ToLower(candidate), prefix) {
				seen[candidate] = struct{}{}
				results = append(results, &brigodier.Suggestion{
					Range: brigodier.StringRange{
						Start: wordStart,
						End:   len(input),
					},
					Text: candidate,
				})
			}
		}
		return brigodier.CreateSuggestion(builder.Input, results)
	}
}

// RegisterCommands registers /whitelist and /blacklist command trees into Gate.
func RegisterCommands(p *proxy.Proxy, mgr *Manager) {
	onlinePlayerCandidates := func() []string {
		if p == nil {
			return nil
		}
		players := p.Players()
		res := make([]string, 0, len(players))
		for _, pl := range players {
			res = append(res, pl.Username())
		}
		return res
	}

	whitelistEntriesCandidates := func() []string {
		st := mgr.WhitelistState()
		if st == nil {
			return nil
		}
		res := make([]string, 0, len(st.RawNicks)+len(st.RawUUIDs)+len(st.RawIPs))
		res = append(res, st.RawNicks...)
		res = append(res, st.RawUUIDs...)
		res = append(res, st.RawIPs...)
		return res
	}

	blacklistEntriesCandidates := func() []string {
		st := mgr.BlacklistState()
		if st == nil {
			return nil
		}
		res := make([]string, 0, len(st.RawNicks)+len(st.RawUUIDs)+len(st.RawIPs))
		res = append(res, st.RawNicks...)
		res = append(res, st.RawUUIDs...)
		res = append(res, st.RawIPs...)
		return res
	}

	// -------------------------------------------------------------------------
	// 1. /whitelist (/simplewhitelist, /swl)
	// -------------------------------------------------------------------------
	buildWhitelistCmd := func(alias string) brigodier.LiteralNodeBuilder {
		return brigodier.Literal(alias).
			Requires(command.Requires(func(c *command.RequiresContext) bool {
				return c.Source.HasPermission("simplewhitelist.admin")
			})).
			Executes(command.Command(func(cmdCtx *command.Context) error {
				st := mgr.WhitelistState()
				status := "§cDisabled"
				if st != nil && st.Enabled {
					status = "§aEnabled"
				}
				nicks, uuids, ips := 0, 0, 0
				if st != nil {
					nicks, uuids, ips = len(st.RawNicks), len(st.RawUUIDs), len(st.RawIPs)
				}
				return cmdCtx.Source.SendMessage(&c.Text{
					Content: fmt.Sprintf("§b[SimpleWhitelist] §7Status: %s §7(Nicks: §f%d§7, UUIDs: §f%d§7, IPs: §f%d§7)\n"+
						"§7Commands: §f/%s <on|off|add|remove|list|reload>",
						status, nicks, uuids, ips, alias),
				})
			})).
			Then(brigodier.Literal("on").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(true); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to enable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleWhitelist] Whitelist enabled."})
				})),
			).
			Then(brigodier.Literal("enable").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(true); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to enable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleWhitelist] Whitelist enabled."})
				})),
			).
			Then(brigodier.Literal("off").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(false); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to disable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleWhitelist] Whitelist disabled."})
				})),
			).
			Then(brigodier.Literal("disable").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(false); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to disable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleWhitelist] Whitelist disabled."})
				})),
			).
			Then(brigodier.Literal("reload").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.Load(); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Reload failed: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleWhitelist] Whitelist & Blacklist reloaded successfully!"})
				})),
			).
			Then(brigodier.Literal("list").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					st := mgr.WhitelistState()
					if st == nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleWhitelist] Whitelist is empty."})
					}
					total := len(st.RawNicks) + len(st.RawUUIDs) + len(st.RawIPs)
					var b strings.Builder
					b.WriteString(fmt.Sprintf("§b[SimpleWhitelist] §7Entries (Total: §f%d§7):\n", total))
					if len(st.RawNicks) > 0 {
						b.WriteString(fmt.Sprintf("  §7• Nicks (§f%d§7): §f%s\n", len(st.RawNicks), strings.Join(st.RawNicks, "§7, §f")))
					}
					if len(st.RawUUIDs) > 0 {
						b.WriteString(fmt.Sprintf("  §7• UUIDs (§f%d§7): §f%s\n", len(st.RawUUIDs), strings.Join(st.RawUUIDs, "§7, §f")))
					}
					if len(st.RawIPs) > 0 {
						b.WriteString(fmt.Sprintf("  §7• IPs/CIDRs (§f%d§7): §f%s\n", len(st.RawIPs), strings.Join(st.RawIPs, "§7, §f")))
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: strings.TrimRight(b.String(), "\n")})
				})),
			).
			Then(brigodier.Literal("add").
				Then(brigodier.Argument("target", brigodier.String).
					Suggests(suggestCandidates(onlinePlayerCandidates)).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						target := cmdCtx.String("target")
						tType, canonical, added, err := mgr.AddWhitelist(target)
						if err != nil {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Error adding %s: %v", target, err)})
						}
						if !added {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§e[SimpleWhitelist] %s (%s) is already whitelisted.", canonical, tType)})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§a[SimpleWhitelist] Added %s (%s) to whitelist.", canonical, tType)})
					})),
				),
			).
			Then(brigodier.Literal("remove").
				Then(brigodier.Argument("target", brigodier.String).
					Suggests(suggestCandidates(whitelistEntriesCandidates)).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						target := cmdCtx.String("target")
						tType, canonical, removed, err := mgr.RemoveWhitelist(target)
						if err != nil {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Error removing %s: %v", target, err)})
						}
						if !removed {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] %s was not found in whitelist.", target)})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§a[SimpleWhitelist] Removed %s (%s) from whitelist.", canonical, tType)})
					})),
				),
			)
	}

	// -------------------------------------------------------------------------
	// 2. /blacklist (/simpleblacklist, /sbl)
	// -------------------------------------------------------------------------
	kickOnlineMatches := func(tType TargetType, canonical string, customReason c.Component) int {
		if p == nil {
			return 0
		}
		kickedCount := 0
		for _, pl := range p.Players() {
			shouldKick := false
			switch tType {
			case TargetNick:
				if strings.EqualFold(pl.Username(), canonical) {
					shouldKick = true
				}
			case TargetUUID:
				if strings.EqualFold(pl.ID().String(), canonical) {
					shouldKick = true
				}
			case TargetIP:
				if addr := pl.RemoteAddr(); addr != nil {
					host := addr.String()
					if h, _, err := net.SplitHostPort(host); err == nil {
						host = h
					}
					if strings.EqualFold(host, canonical) {
						shouldKick = true
					}
				}
			case TargetCIDR:
				if addr := pl.RemoteAddr(); addr != nil {
					host := addr.String()
					if h, _, err := net.SplitHostPort(host); err == nil {
						host = h
					}
					if pip := net.ParseIP(host); pip != nil {
						if _, ipNet, err := net.ParseCIDR(canonical); err == nil && ipNet.Contains(pip) {
							shouldKick = true
						}
					}
				}
			}

			if shouldKick {
				// Check blacklist bypass permission
				if !pl.HasPermission("simpleblacklist.bypass") {
					pl.Disconnect(customReason)
					kickedCount++
				}
			}
		}
		return kickedCount
	}

	buildBlacklistCmd := func(alias string) brigodier.LiteralNodeBuilder {
		handleBlacklistAdd := func(cmdCtx *command.Context, reason string) error {
			target := cmdCtx.String("target")
			tType, canonical, added, err := mgr.AddBlacklist(target)
			if err != nil {
				return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] Error adding %s: %v", target, err)})
			}

			st := mgr.BlacklistState()
			kickReason := st.KickReason
			if reason != "" {
				kickReason = &c.Text{Content: fmt.Sprintf("§cYou are blacklisted: %s", reason)}
			}

			kicked := kickOnlineMatches(tType, canonical, kickReason)
			extra := ""
			if kicked > 0 {
				extra = fmt.Sprintf(" §7(Disconnected %d online player(s))", kicked)
			}

			if !added {
				return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§e[SimpleBlacklist] %s (%s) is already blacklisted.%s", canonical, tType, extra)})
			}
			return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§a[SimpleBlacklist] Added %s (%s) to blacklist.%s", canonical, tType, extra)})
		}

		return brigodier.Literal(alias).
			Requires(command.Requires(func(c *command.RequiresContext) bool {
				return c.Source.HasPermission("simpleblacklist.admin")
			})).
			Executes(command.Command(func(cmdCtx *command.Context) error {
				st := mgr.BlacklistState()
				status := "§cDisabled"
				if st != nil && st.Enabled {
					status = "§aEnabled"
				}
				nicks, uuids, ips := 0, 0, 0
				if st != nil {
					nicks, uuids, ips = len(st.RawNicks), len(st.RawUUIDs), len(st.RawIPs)
				}
				return cmdCtx.Source.SendMessage(&c.Text{
					Content: fmt.Sprintf("§b[SimpleBlacklist] §7Status: %s §7(Nicks: §f%d§7, UUIDs: §f%d§7, IPs: §f%d§7)\n"+
						"§7Commands: §f/%s <on|off|add|remove|list|reload>",
						status, nicks, uuids, ips, alias),
				})
			})).
			Then(brigodier.Literal("on").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetBlacklistEnabled(true); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] Failed to enable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleBlacklist] Blacklist enabled."})
				})),
			).
			Then(brigodier.Literal("enable").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetBlacklistEnabled(true); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] Failed to enable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleBlacklist] Blacklist enabled."})
				})),
			).
			Then(brigodier.Literal("off").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetBlacklistEnabled(false); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] Failed to disable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleBlacklist] Blacklist disabled."})
				})),
			).
			Then(brigodier.Literal("disable").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetBlacklistEnabled(false); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] Failed to disable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleBlacklist] Blacklist disabled."})
				})),
			).
			Then(brigodier.Literal("reload").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.Load(); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] Reload failed: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleBlacklist] Configuration reloaded successfully!"})
				})),
			).
			Then(brigodier.Literal("list").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					st := mgr.BlacklistState()
					if st == nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleBlacklist] Blacklist is empty."})
					}
					total := len(st.RawNicks) + len(st.RawUUIDs) + len(st.RawIPs)
					var b strings.Builder
					b.WriteString(fmt.Sprintf("§b[SimpleBlacklist] §7Entries (Total: §f%d§7):\n", total))
					if len(st.RawNicks) > 0 {
						b.WriteString(fmt.Sprintf("  §7• Nicks (§f%d§7): §f%s\n", len(st.RawNicks), strings.Join(st.RawNicks, "§7, §f")))
					}
					if len(st.RawUUIDs) > 0 {
						b.WriteString(fmt.Sprintf("  §7• UUIDs (§f%d§7): §f%s\n", len(st.RawUUIDs), strings.Join(st.RawUUIDs, "§7, §f")))
					}
					if len(st.RawIPs) > 0 {
						b.WriteString(fmt.Sprintf("  §7• IPs/CIDRs (§f%d§7): §f%s\n", len(st.RawIPs), strings.Join(st.RawIPs, "§7, §f")))
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: strings.TrimRight(b.String(), "\n")})
				})),
			).
			Then(brigodier.Literal("add").
				Then(brigodier.Argument("target", brigodier.String).
					Suggests(suggestCandidates(onlinePlayerCandidates)).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						return handleBlacklistAdd(cmdCtx, "")
					})).
					Then(brigodier.Argument("reason", brigodier.String).
						Executes(command.Command(func(cmdCtx *command.Context) error {
							return handleBlacklistAdd(cmdCtx, cmdCtx.String("reason"))
						})),
					),
				),
			).
			Then(brigodier.Literal("remove").
				Then(brigodier.Argument("target", brigodier.String).
					Suggests(suggestCandidates(blacklistEntriesCandidates)).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						target := cmdCtx.String("target")
						tType, canonical, removed, err := mgr.RemoveBlacklist(target)
						if err != nil {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] Error removing %s: %v", target, err)})
						}
						if !removed {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleBlacklist] %s was not found in blacklist.", target)})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§a[SimpleBlacklist] Removed %s (%s) from blacklist.", canonical, tType)})
					})),
				),
			)
	}

	// Register Whitelist aliases
	p.Command().Register(buildWhitelistCmd("whitelist"))
	p.Command().Register(buildWhitelistCmd("simplewhitelist"))
	p.Command().Register(buildWhitelistCmd("swl"))

	// Register Blacklist aliases
	p.Command().Register(buildBlacklistCmd("blacklist"))
	p.Command().Register(buildBlacklistCmd("simpleblacklist"))
	p.Command().Register(buildBlacklistCmd("sbl"))
}
