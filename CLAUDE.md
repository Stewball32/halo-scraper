# cairo-station-LAN

**This file provides guidance to Claude Code (claude.ai/code) when working with this repository.**

**Maintenance:** Keep this file up to date as the project evolves. When adding new features, changing addresses, modifying CLI flags, or altering architecture, update the relevant sections here.

**Repository:** [`Stewball32/halo-scraper`](https://github.com/Stewball32/halo-scraper) (branch: `cairo-station-lan`)
**Maintainer:** Roasted-Codes
**License:** GPL-3.0

---

## Project Overview

cairo-station-LAN is a stripped-down Docker stack that runs xemu (original Xbox emulator) with pcap-bridged networking and halo-scraper embedded, supporting up to 3 independent instances on the same machine. No Tailscale, XLink Kai, dnsmasq, nettools, or telemetry — just the emulator, the scraper, and a socat relay for XBDM/FTP access via SSH tunnel.

### What This Provides

- **xemu** with pcap backend — emulated Xbox gets its own static IP on a Docker bridge
- **halo-scraper** embedded in the xemu container — QMP-based Halo stats scraper on `:9000`
- **socat relay** on a separate bridge port — forwards XBDM (`:731`) and FTP (`:21`) to the Xbox
- **Multi-instance** — up to 3 simultaneous isolated instances via `.env.1` / `.env.2` / `.env.3`
- **SSH tunnel access** — all ports bind `127.0.0.1`; user SSH-tunnels XBDM and FTP to workstation

### Critical Network Fixes

This project solves three bugs that cause silent failures with pcap networking in Docker:

1. **libpcap immediate mode** (CRITICAL): xemu cannot receive packets without `pcap_set_immediate_mode()`. Fixed with LD_PRELOAD shim ([`pcap_immediate.c`](services/xemu/data/emulator/pcap_immediate.c)).

2. **TX checksum offloading**: Host kernel writes placeholder checksums expecting NIC hardware to complete them, but the Xbox's pcap-injected IPs traverse a software bridge — no hardware ever fills in the checksum, and the Xbox silently drops every TCP packet (ICMP ping still works). Fixed with `ethtool -K eth0 tx off` in the relay container's startup command.

3. **Bridge hairpin mode**: A container cannot reach an IP that sits on the same bridge port as itself. The socat relay is on a separate bridge port from xemu, so it can reach the Xbox IPs without hairpin.

---

## Important Constraints ⚠️

- **🚨 NEVER push to GitHub without explicit user confirmation first.**
- **🔄 Keep CLAUDE.md and README.md in sync** when changing compose, init scripts, IPs, or architecture.
- **Never apply `setcap` at build time** — must be runtime via `services/xemu/init/10-xemu-setcap`.
- **`setcap` + `ldconfig` + `/etc/ld.so.preload` must always be applied together** — skipping any one silently breaks xemu.
- **`pcap_immediate.so` must be in `/etc/ld.so.preload`** — without it xemu can send packets but never receive them.
- **`ethtool -K eth0 tx off` must run on the relay container** — without it, all inbound TCP to the Xbox silently fails while ping works.
- **The `custom-cont-init.d` mount must go to `/custom-cont-init.d`** (root) — LinuxServer's s6-overlay only scans the root path.

### ⚠️ Critical: setcap + ldconfig + LD_PRELOAD Must Form a Unit

```
┌─────────────────────────────────────────────────────────────────┐
│ CRITICAL UNIT - All three must be present:                      │
├─────────────────────────────────────────────────────────────────┤
│ 1. ldconfig: Register AppImage libraries system-wide            │
│ 2. LD_PRELOAD: Write shims to /etc/ld.so.preload (not env var) │
│ 3. setcap: Apply capabilities (triggers secure exec mode)       │
└─────────────────────────────────────────────────────────────────┘
```

- `setcap` triggers secure execution mode (AT_SECURE), stripping `LD_PRELOAD` and `LD_LIBRARY_PATH` env vars
- xemu's AppImage bundles libraries in `/opt/xemu/usr/lib/` — without `ldconfig`, they're unfindable after setcap
- Selkies gamepad input requires `LD_PRELOAD` joystick interposer via `/etc/ld.so.preload`
- pcap receive requires `LD_PRELOAD` immediate mode shim via `/etc/ld.so.preload`

**What happens if you skip one:**
- Skip ldconfig → `error while loading shared libraries: libSDL2-2.0.so.0`
- Skip LD_PRELOAD → gamepad and pcap packet receive fail silently
- Skip setcap → pcap networking doesn't work

**Implemented in:** `services/xemu/init/10-xemu-setcap`

---

## Directory Structure

```
cairo-station-LAN/
├── docker-compose.yml                  # 2 services: xemu + relay (all vars from .env file)
├── .env.1                              # Instance 1 config (ports 3000/731/2121, subnet 172.20.1.x)
├── .env.2                              # Instance 2 config (ports 3010/732/2122, subnet 172.20.2.x)
├── .env.3                              # Instance 3 config (ports 3020/733/2123, subnet 172.20.3.x)
├── README.md
├── CLAUDE.md                           # This file
├── LICENSE
├── .gitignore
│
├── services/
│   └── xemu/
│       ├── Dockerfile                  # Overlay image: adds wmctrl, halo-scraper, custom autostart
│       ├── root/
│       │   └── defaults/
│       │       └── autostart           # Openbox startup: launches xemu in xterm
│       ├── init/                       # s6-overlay init scripts (mounted to /custom-cont-init.d)
│       │   ├── 01-install-autostart    # Syncs autostart, creates xemu.toml symlink
│       │   └── 10-xemu-setcap          # Network caps, TX checksum fix, ldconfig, ld.so.preload
│       └── data/                       # Instance 1 runtime volume (DATA_DIR=./services/xemu/data)
│           ├── emulator/
│           │   ├── xemu.toml           # xemu config (pcap on eth0, BIOS paths, input bindings)
│           │   ├── pcap_immediate.c    # LD_PRELOAD shim source
│           │   ├── pcap_immediate.so   # Compiled shim (built inside container, not in git)
│           │   ├── mcpx_1.0.bin        # Xbox boot ROM
│           │   ├── CerbiosDebug.bin    # BIOS
│           │   ├── iguana-eeprom.bin   # EEPROM
│           │   └── iguana-dev.qcow2    # Xbox HDD image (not in git, ~3.6GB)
│           ├── games/                  # Game ISOs (not in git)
│           └── halo-scraper/
│               └── halo-scraper.toml  # Scraper config (shared read-only across all instances)
│
└── instances/                          # Data dirs for instances 2 and 3 (not in git — has qcow2)
    ├── 2/
    │   └── emulator/                   # Separate xemu.toml + separate qcow2 for instance 2
    └── 3/
        └── emulator/                   # Separate xemu.toml + separate qcow2 for instance 3
```

### Files NOT in Git

- `*.qcow2` — Xbox HDD images (~3.6GB each)
- `*.iso` — Game ISOs
- `*.so` — Compiled shared libraries (built inside container at runtime)
- `instances/` — Contains qcow2 files
- `services/xemu/data/.cache/`, `.local/`, etc. — Runtime-generated LinuxServer base image state

---

## Multi-Instance Architecture

Each instance is fully isolated:

| Variable | Instance 1 | Instance 2 | Instance 3 |
|----------|-----------|-----------|-----------|
| `COMPOSE_PROJECT_NAME` | xemu-1 | xemu-2 | xemu-3 |
| `BRIDGE_NAME` | br-xemu1 | br-xemu2 | br-xemu3 |
| `SUBNET` | 172.20.1.0/24 | 172.20.2.0/24 | 172.20.3.0/24 |
| `XEMU_IP` | 172.20.1.49 | 172.20.2.49 | 172.20.3.49 |
| `XBOX_TITLE_IP` | 172.20.1.50 | 172.20.2.50 | 172.20.3.50 |
| `XBOX_DEBUG_IP` | 172.20.1.51 | 172.20.2.51 | 172.20.3.51 |
| `RELAY_IP` | 172.20.1.11 | 172.20.2.11 | 172.20.3.11 |
| Selkies HTTP | 3000 | 3010 | 3020 |
| Selkies HTTPS | 3001 | 3011 | 3021 |
| QMP | 4444 | 4454 | 4464 |
| halo-scraper WS | 9000 | 9010 | 9020 |
| XBDM relay | 731 | 732 | 733 |
| FTP relay | 2121 | 2122 | 2123 |
| `DATA_DIR` | ./services/xemu/data | ./instances/2 | ./instances/3 |

`COMPOSE_PROJECT_NAME` namespaces Docker container names and network names, preventing collisions between instances.

---

## Build & Run Commands

### Build (once, or after Dockerfile changes)

```bash
cd cairo-station-LAN
docker compose --env-file .env.1 build
```

**Built image:** `xemu-bridged:latest` (shared by all instances)

### Run

```bash
docker compose --env-file .env.1 up -d
docker compose --env-file .env.2 up -d
docker compose --env-file .env.3 up -d
```

### Logs

```bash
docker compose --env-file .env.1 logs -f xemu
docker compose --env-file .env.1 logs -f relay
```

### Stop

```bash
docker compose --env-file .env.1 down
```

### Rebuild After Changes

```bash
docker compose --env-file .env.1 down
docker compose --env-file .env.1 build --no-cache
docker compose --env-file .env.1 up -d
```

### SSH Tunnels (from workstation)

```bash
# Instance 1
ssh -L 731:127.0.0.1:731 -L 21:127.0.0.1:2121 user@server
# Assembly → localhost:731  |  FTP → localhost:21

# Instance 2
ssh -L 731:127.0.0.1:732 -L 21:127.0.0.1:2122 user@server

# Instance 3
ssh -L 731:127.0.0.1:733 -L 21:127.0.0.1:2123 user@server
```

**FTP passive mode:** NOT supported via SSH tunnel. The Xbox sends its internal IP for passive data connections, which isn't reachable from the workstation. Use active mode: `lftp` with `set ftp:passive-mode off`, or FileZilla in active mode.

---

## Container Startup Flow

1. **s6-overlay runs `01-install-autostart`** (root):
   - Force-copies `/defaults/autostart` to `/config/.config/openbox/autostart` (base image only does this on first run)
   - Creates xemu directory tree, symlinks `xemu.toml`

2. **s6-overlay runs `10-xemu-setcap`** (root):
   - `ip link set eth0 promisc on` — enables promiscuous mode for Xbox-addressed frames
   - `ethtool -K eth0 tx off` — disables TX checksum offloading (critical for TCP to Xbox)
   - `ldconfig` — registers AppImage libraries so they survive setcap stripping LD_LIBRARY_PATH
   - `setcap cap_net_raw,cap_net_admin+eip` — grants pcap capabilities to xemu binary
   - Writes `/etc/ld.so.preload`: Selkies interposer + fake udev + `pcap_immediate.so`

3. **Desktop session starts** (user `abc`):
   - Openbox autostart launches xemu via `xterm -e /opt/xemu/AppRun`
   - halo-scraper starts separately in the container, connects via `/tmp/qmp.sock`

---

## Known Gotchas

### setcap Strips LD_PRELOAD and LD_LIBRARY_PATH

`setcap` triggers AT_SECURE (secure execution mode), same as setuid. Linux strips both `LD_PRELOAD` and `LD_LIBRARY_PATH`. Without the ldconfig + `/etc/ld.so.preload` workaround, xemu crashes with missing library errors and pcap receive silently stops working.

### NEVER Apply setcap at Build Time

File capabilities set during `docker build` don't persist at runtime (overlay filesystem xattr handling). Always apply in `services/xemu/init/10-xemu-setcap`.

### TX Checksum Offloading Breaks Xbox TCP

**Symptom:** `ping 172.20.x.51` works ✅, `nc 172.20.x.51 731` times out ❌

The relay container fixes this for itself with `ethtool -K eth0 tx off` in its startup command. The xemu container's init script also disables it on xemu's own eth0 — both are needed.

### Bridge Hairpin Mode

The xemu container and its pcap-injected Xbox IPs (.50/.51) share the same bridge port. Linux bridge hairpin is off by default, so xemu cannot reach the Xbox IPs directly from inside the container. The relay container is on a different bridge port and has no such restriction.

### FTP Passive Mode

SSH tunnel + socat only supports active mode FTP. Passive mode fails because the Xbox tells the FTP client to open a data connection to `172.20.x.50` directly, which is unreachable from the user's home network.

### Instance Data Directories

Each instance needs its own qcow2 (independent Xbox HDD state). Instances 2 and 3 must have separate copies in `instances/2/emulator/` and `instances/3/emulator/`. Sharing a qcow2 across running instances will corrupt it.

### halo-scraper "Unmapped" Errors

Expected when Halo CE isn't loaded. The scraper polls QMP memory constantly; "unmapped" in logs just means the game isn't running yet.

---

## Quick Reference

```bash
# Verify TX checksum disabled (xemu container)
docker exec xemu-1 ethtool -k eth0 | grep tx-checksum
# Expected: tx-checksum-ip-generic: off

# Check LD_PRELOAD shims
docker exec xemu-1 cat /etc/ld.so.preload
# Expected: selkies_joystick_interposer.so, libudev-fake, pcap_immediate.so

# Check xemu capabilities
docker exec xemu-1 getcap /opt/xemu/usr/bin/xemu
# Expected: cap_net_admin,cap_net_raw+eip

# Test relay connectivity (from relay container to Xbox)
docker exec relay-1 nc -zv -w3 172.20.1.51 731

# Recompile pcap_immediate.so inside container
docker exec xemu-1 bash -c 'gcc -shared -fPIC -o /config/emulator/pcap_immediate.so /config/emulator/pcap_immediate.c -ldl'
```

---

## Architecture

```
Host (127.0.0.1)
│
├── :731  → relay-1 container (172.20.1.11)
│               └── socat → 172.20.1.51:731  (Xbox debug — XBDM)
├── :2121 → relay-1 container (172.20.1.11)
│               └── socat → 172.20.1.50:21   (Xbox title — FTP)
│
├── :3000/:3001 → xemu-1 container (172.20.1.49) — Selkies web UI
├── :4444       → xemu-1 container — QMP
├── :9000       → xemu-1 container — halo-scraper WebSocket
│
│   Docker bridge: br-xemu1 (172.20.1.0/24)
│   ├── 172.20.1.49   xemu-1 container (eth0)
│   │       └── pcap injects frames for:
│   │           172.20.1.50  Xbox title interface (FTP :21)
│   │           172.20.1.51  Xbox debug interface (XBDM :731, ping)
│   └── 172.20.1.11   relay-1 container (eth0, TX checksums OFF)
│
├── :732/:2122 → relay-2 → br-xemu2 (172.20.2.x) — same pattern
└── :733/:2123 → relay-3 → br-xemu3 (172.20.3.x) — same pattern
```

---

## References

- [xemu Documentation](https://xemu.app/docs/)
- [LinuxServer.io xemu Image](https://github.com/linuxserver/docker-xemu)
- [halo-scraper](https://github.com/Stewball32/halo-scraper)
- [libpcap Immediate Mode Issue](https://github.com/the-tcpdump-group/libpcap/issues/1099)
