package simplewhitelist

import (
	"fmt"
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

// RegisterCommands registers /whitelist, /simplewhitelist, and /swl command trees into Gate.
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

	hasPerm := func(src command.Source, required ...string) bool {
		if src.HasPermission("simplewhitelist.admin") || src.HasPermission("simplewhitelist.*") || src.HasPermission("*") {
			return true
		}
		for _, r := range required {
			if src.HasPermission(r) {
				return true
			}
		}
		return false
	}

	buildWhitelistCmd := func(alias string) brigodier.LiteralNodeBuilder {
		return brigodier.Literal(alias).
			Requires(command.Requires(func(c *command.RequiresContext) bool {
				return hasPerm(c.Source, "simplewhitelist.use", "simplewhitelist.list", "simplewhitelist.add",
					"simplewhitelist.remove", "simplewhitelist.toggle", "simplewhitelist.reload")
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
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.toggle")
				})).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(true); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to enable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleWhitelist] Whitelist enabled."})
				})),
			).
			Then(brigodier.Literal("enable").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.toggle")
				})).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(true); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to enable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleWhitelist] Whitelist enabled."})
				})),
			).
			Then(brigodier.Literal("off").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.toggle")
				})).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(false); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to disable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleWhitelist] Whitelist disabled."})
				})),
			).
			Then(brigodier.Literal("disable").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.toggle")
				})).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.SetWhitelistEnabled(false); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Failed to disable: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[SimpleWhitelist] Whitelist disabled."})
				})),
			).
			Then(brigodier.Literal("reload").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.reload")
				})).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					if err := mgr.Load(); err != nil {
						return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[SimpleWhitelist] Reload failed: %v", err)})
					}
					return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[SimpleWhitelist] Whitelist reloaded successfully!"})
				})),
			).
			Then(brigodier.Literal("list").
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.list")
				})).
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
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.add")
				})).
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
				Requires(command.Requires(func(c *command.RequiresContext) bool {
					return hasPerm(c.Source, "simplewhitelist.remove")
				})).
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

	p.Command().Register(buildWhitelistCmd("whitelist"))
	p.Command().Register(buildWhitelistCmd("simplewhitelist"))
	p.Command().Register(buildWhitelistCmd("swl"))
}
