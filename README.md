# Gate Simple Whitelist

An ultra-lightweight, high-performance whitelist plugin for the [Gate](https://github.com/minekube/gate) Minecraft proxy.

## Features

- ⚡ **Zero-Lock Reads:** Uses Go `sync/atomic.Pointer` for 100% lock-free evaluation during player logins (zero mutex contention).
- 🚀 **Zero Heap Allocation on Deny:** Pre-marshals the kick message once on startup to eliminate allocations.
- 🎯 **Multiple Matching Types:** Whitelist by **Nicknames** (case-insensitive), **UUIDs**, or **IP addresses / CIDR subnet blocks**.
- 🔄 **Hot Reload:** In-game `/simplewhitelist reload` command.
- 🌐 **Container & Pelican Ready:** Fully compatible with containerized deployments and the Pelican/Pterodactyl panel.

---

## Installation

### 1. Add the package to your Gate proxy

```bash
go get github.com/andreisugu/gate-simplewhitelist
```

### 2. Register the plugin in your `main.go` / `gate.go`

```go
package main

import (
	"github.com/andreisugu/gate-simplewhitelist"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func main() {
	proxy.Plugins = append(proxy.Plugins,
		simplewhitelist.Plugin,
	)

	gate.Execute()
}
```

---

## Configuration

The configuration file is automatically created at `config/whitelist.toml`:

```toml
# Set to true to block anyone not listed below.
enabled = true

# Disconnect message shown to unwhitelisted players.
kick_message = "&cYou are not whitelisted on this server!"

# Allowed player nicknames (case-insensitive)
nicks = [
    "Steve",
    "Alex",
]

# Allowed player UUIDs
uuids = [
    # "069a79f4-44e9-4726-a5be-fca90e38aaf5",
]

# Allowed IP addresses or CIDR subnets
ips = [
    # "127.0.0.1",
    # "192.168.1.50",
    # "10.0.0.0/24",
]
```

---

## In-Game Commands

| Command | Description |
| :--- | :--- |
| `/simplewhitelist reload` | Reloads `config/whitelist.toml` without restarting the proxy. |

---

## License

[MIT](LICENSE)
