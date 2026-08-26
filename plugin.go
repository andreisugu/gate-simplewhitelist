package simplewhitelist

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is the ultra-lightweight, zero-fluff simple whitelist & blacklist plugin for Gate proxy.
var Plugin = proxy.Plugin{
	Name: "SimpleWhitelist",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		mgr := NewManager("config")

		if err := mgr.Load(); err != nil {
			log.Error(err, "Failed to load simple whitelist & blacklist configuration")
			return err
		}

		// Fast, lock-free check on player login
		event.Subscribe(p.Event(), 0, func(e *proxy.LoginEvent) {
			if !e.Allowed() {
				return
			}
			pl := e.Player()

			// 1. Blacklist evaluation (first)
			if !pl.HasPermission("simpleblacklist.bypass") {
				if blacklisted, reason := mgr.IsBlacklisted(pl.Username(), pl.ID(), pl.RemoteAddr()); blacklisted {
					e.Deny(reason)
					return
				}
			}

			// 2. Whitelist evaluation (second)
			if !pl.HasPermission("simplewhitelist.bypass") {
				if whitelisted, reason := mgr.IsWhitelisted(pl.Username(), pl.ID(), pl.RemoteAddr()); !whitelisted {
					e.Deny(reason)
					return
				}
			}
		})

		// Register In-Game Commands: /whitelist, /swl, /blacklist, /sbl
		RegisterCommands(p, mgr)

		log.Info("SimpleWhitelist & SimpleBlacklist loaded (nicks, uuids, ips, in-game management active)")
		return nil
	},
}
