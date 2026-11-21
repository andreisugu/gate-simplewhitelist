<div align="center">

# 🛡️ Gate Simple Whitelist

**Lock-free `atomic.Pointer` whitelist for Minekube Gate with username, UUID, IP/CIDR matching & optional `require_ip_match` AND mode.**

<p>
  <a href="https://github.com/andreisugu/gate-simplewhitelist/actions/workflows/ci.yml"><img src="https://github.com/andreisugu/gate-simplewhitelist/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://pkg.go.dev/github.com/andreisugu/gate-simplewhitelist"><img src="https://pkg.go.dev/badge/github.com/andreisugu/gate-simplewhitelist.svg" alt="Go Reference" /></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26" />
  <a href="https://github.com/minekube/gate"><img src="https://img.shields.io/badge/Platform-Minekube_Gate-7c3aed?style=flat-square" alt="Minekube Gate" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg?style=flat-square" alt="Apache 2.0" /></a>
</p>

<p>
  <a href="#-features">Features</a> •
  <a href="#-installation">Installation</a> •
  <a href="#-configuration">Configuration</a> •
  <a href="#-commands--permissions">Commands</a> •
  <a href="#-minekube-gate-extensions-suite">Gate Suite</a>
</p>

</div>

---

## ✨ Features

- **Lock-Free Login Checks:** Evaluates incoming logins via `sync/atomic.Pointer` with zero mutex contention.
- **Zero-Allocation Kick Message:** Pre-marshals the legacy-formatted disconnect component on load/reload.
- **Flexible Identity & Network Rules:** Allow by case-insensitive **nicknames**, **UUIDs**, or **IP addresses / CIDR blocks**.
- **Strict Dual-Check (`require_ip_match`):** Require *both* a matching nick/UUID **and** a matching source IP/CIDR when enabled (fail-closed if configured IP/CIDR entries are malformed).
- **Live In-Game Management & Bypass:** Add, remove, toggle, inspect, and hot-reload entries without proxy restarts, plus `simplewhitelist.bypass` permission support.

---

## 📦 Installation

### Using `gate-plugin-builder` / Pelican Egg
Add to your `plugins.txt` or `GATE_PLUGINS`:
```text
github.com/andreisugu/gate-simplewhitelist
```

### In Go Code
```bash
go get github.com/andreisugu/gate-simplewhitelist
```

```go
package main

import (
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"

	simplewhitelist "github.com/andreisugu/gate-simplewhitelist"
)

func main() {
	proxy.Plugins = append(proxy.Plugins, simplewhitelist.Plugin)
	gate.Execute()
}
```

---

## ⚙️ Configuration

Generated at `config/whitelist.toml`:

```toml
# Set to true to block anyone not permitted below
enabled = true

# Disconnect message shown to unwhitelisted players
kick_message = "&cYou are not whitelisted on this server!"

# When true and ips/CIDRs are non-empty, require BOTH matching nick/UUID AND matching source IP/CIDR
require_ip_match = false

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

## ⌨️ Commands & Permissions

**Aliases:** `/whitelist`, `/simplewhitelist`, `/swl`

| Command | Permission | Description |
| :--- | :--- | :--- |
| `/whitelist` | `simplewhitelist.use` *(or `simplewhitelist.admin`)* | Show current whitelist status and entry counts |
| `/whitelist on` / `enable` / `off` / `disable` | `simplewhitelist.toggle` | Enable or disable enforcement at runtime |
| `/whitelist add <nick \| uuid \| ip \| cidr>` | `simplewhitelist.add` | Auto-detect type, add entry, and persist to `whitelist.toml` |
| `/whitelist remove <target>` | `simplewhitelist.remove` | Remove a nick, UUID, or IP/CIDR entry and persist |
| `/whitelist list` | `simplewhitelist.list` | Display all whitelisted nicknames, UUIDs, and IPs/CIDRs |
| `/whitelist reload` | `simplewhitelist.reload` | Reload `config/whitelist.toml` from disk |

**Additional Permission Nodes:**
- `simplewhitelist.admin` / `simplewhitelist.*` — Grants all whitelist management subcommands.
- `simplewhitelist.bypass` — Allows a player to bypass the whitelist at login.

---

## 🌐 Minekube Gate Extensions Suite

| Project | Role | Stack |
| :--- | :--- | :--- |
| [`gate-stonyperms`](https://github.com/andreisugu/gate-stonyperms) | LuckPerms-style permissions, contexts, tracks & v0.2 R/W HTTP + SSE API | `Go 1.26` |
| [`stonyperms-bridge`](https://github.com/andreisugu/stonyperms-bridge) | Paper & Fabric permission adapters, live SSE, `/spb` commands, Vault & PAPI | `Java 25` |
| [`gate-bettertab`](https://github.com/andreisugu/gate-bettertab) | Animated RGB / MiniMessage TAB lists, server groups, weights & placeholders | `Go 1.26` |
| [`gate-discord4gate`](https://github.com/andreisugu/gate-discord4gate) | Bidirectional Discord <> Minecraft chat, slash commands & webhook avatars | `Go 1.26` |
| [`gate-hybridforwarding`](https://github.com/andreisugu/gate-hybridforwarding) | Per-backend Velocity Modern, Legacy BungeeCord, BungeeGuard & None forwarding | `Go 1.26` |
| **[`gate-simplewhitelist`](https://github.com/andreisugu/gate-simplewhitelist)** *(this repo)* | Lock-free `atomic.Pointer` whitelist by username, UUID, IP & CIDR | `Go 1.26` |
| [`gate-skinschanger`](https://github.com/andreisugu/gate-skinschanger) | Resilient multi-provider & Mineskin URL skin switcher with live backend refresh | `Go 1.26` |
| [`gate-smartlimbo`](https://github.com/andreisugu/gate-smartlimbo) | Limbo failover, protected sandbox & batched backend auto-reconnect queues | `Go 1.26` |
| [`gate-plugin-builder`](https://github.com/andreisugu/gate-plugin-builder) | Zero-config static Gate bundler (`build.sh`) + Pelican / Pterodactyl egg | `Shell` |

---

## 📄 License

Licensed under the [Apache License 2.0](LICENSE) © Andrei.
