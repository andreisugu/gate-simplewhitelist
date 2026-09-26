package simplewhitelist

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is the ultra-lightweight, zero-fluff simple whitelist plugin for Gate proxy.
var Plugin = proxy.Plugin{
	Name: "SimpleWhitelist",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		mgr := NewManager("config")

		if err := mgr.Load(); err != nil {
			log.Error(err, "Failed to load simple whitelist configuration")
			return err
		}

		// Fast, lock-free check on player login
		event.Subscribe(p.Event(), 0, func(e *proxy.LoginEvent) {
			if !e.Allowed() {
				return
			}
			pl := e.Player()

			// Whitelist evaluation (respects simplewhitelist.bypass permission)
			if !pl.HasPermission("simplewhitelist.bypass") {
				if whitelisted, reason := mgr.IsWhitelisted(pl.Username(), pl.ID(), pl.RemoteAddr()); !whitelisted {
					e.Deny(reason)
					rAddr := ""
					if pl.RemoteAddr() != nil {
						rAddr = pl.RemoteAddr().String()
					}
					log.Info("Unwhitelisted player rejected", "player", pl.Username(), "remoteAddr", rAddr)
					return
				}
			}
		})

		// Register In-Game Commands: /whitelist, /simplewhitelist, /swl
		RegisterCommands(p, mgr)

		log.Info("SimpleWhitelist loaded (nicks, uuids, ips, in-game management active)")
		return nil
	},
}
