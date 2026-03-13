# cairo-station-LAN

Run up to three independent xemu (original Xbox emulator) instances in Docker, each with its own bridge network. Access XBDM and FTP from anywhere via SSH tunnel — no Tailscale required.

[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)

---

## What's Running

Two containers per instance:

| Container | What It Does |
|-----------|--------------|
| `xemu-N` | The emulator — browser UI, QMP, halo-scraper WebSocket |
| `relay-N` | socat — forwards XBDM (`:731`) and FTP (`:21`) to the Xbox |

### Port Map

| | Instance 1 | Instance 2 | Instance 3 |
|-|-----------|-----------|-----------|
| Selkies HTTP | 3000 | 3010 | 3020 |
| Selkies HTTPS | 3001 | 3011 | 3021 |
| QMP | 4444 | 4454 | 4464 |
| halo-scraper WS | 9000 | 9010 | 9020 |
| XBDM relay | 731 | 732 | 733 |
| FTP relay | 2121 | 2122 | 2123 |

All ports bind to `127.0.0.1` only — not exposed publicly.

---

## Setup

**You'll need:** A Linux host with Docker Compose V2.

Put your Xbox files in `services/xemu/data/emulator/`:

| File | What It Is |
|------|------------|
| `mcpx_1.0.bin` | Boot ROM |
| `CerbiosDebug.bin` | BIOS |
| `iguana-eeprom.bin` | EEPROM |
| `iguana-dev.qcow2` | Hard drive image (~3.6 GB — not in git) |

Build the image (only needed once, or after Dockerfile changes):

```bash
docker compose --env-file .env.1 build
```

---

## Running Instances

```bash
# Start
docker compose --env-file .env.1 up -d
docker compose --env-file .env.2 up -d
docker compose --env-file .env.3 up -d

# Logs
docker compose --env-file .env.1 logs -f xemu

# Stop
docker compose --env-file .env.1 down
```

Instances 2 and 3 require their own data directories with separate qcow2 files (each Xbox needs independent HDD state). See `DATA_DIR` in `.env.2` / `.env.3`.

---

## Access via SSH Tunnel

All ports are `127.0.0.1`-only on the server. Tunnel from your workstation:

```bash
# Instance 1
ssh -L 731:127.0.0.1:731 -L 21:127.0.0.1:2121 user@server
# Assembly → localhost:731  |  FTP → localhost:21

# Instance 2
ssh -L 731:127.0.0.1:732 -L 21:127.0.0.1:2122 user@server

# Instance 3
ssh -L 731:127.0.0.1:733 -L 21:127.0.0.1:2123 user@server
```

**FTP note:** Active mode only. Passive fails because the Xbox advertises its internal IP (`172.20.x.50`) for data connections, which isn't reachable from home. Use `lftp` with `set ftp:passive-mode off`, or FileZilla in active mode.

---

## Xbox Network Setup

Each instance has its own isolated Docker bridge subnet. Configure static IPs inside Xbox Dashboard → Settings → Network Settings → Manual:

| | Instance 1 | Instance 2 | Instance 3 |
|-|-----------|-----------|-----------|
| Title IP | 172.20.1.50 | 172.20.2.50 | 172.20.3.50 |
| Debug IP | 172.20.1.51 | 172.20.2.51 | 172.20.3.51 |
| Gateway | 172.20.1.1 | 172.20.2.1 | 172.20.3.1 |
| Subnet | 255.255.255.0 | 255.255.255.0 | 255.255.255.0 |

---

## Under the Hood

Getting xemu's pcap networking to work inside Docker required solving three bugs that cause silent failures:

1. **Packet receive never works** — xemu can send packets but the receive socket never fires. Fixed by a small shim ([`pcap_immediate.c`](services/xemu/data/emulator/pcap_immediate.c)) that enables immediate mode on the capture device. Compiled inside the container at startup and injected via `/etc/ld.so.preload`.

2. **TCP always times out even though ping works** — the Linux kernel leaves TCP checksums partially filled, expecting the NIC hardware to complete them. On a software bridge, nothing ever does, so the Xbox drops every TCP packet. The relay container runs `ethtool -K eth0 tx off` at startup to fix this.

3. **XBDM/FTP relay** — the socat relay sits on a different bridge port than xemu, bypassing Linux bridge hairpin mode. It relays `:731` → Xbox debug IP and `:21` → Xbox title IP.

Full details in [CLAUDE.md](CLAUDE.md).

---

## Troubleshooting

| Symptom | What to Check |
|---------|---------------|
| xemu won't start | File paths in `xemu.toml` under `[sys.files]` — must match what's in `emulator/` |
| Sends but never receives packets | `docker exec xemu-1 cat /etc/ld.so.preload` — must include `pcap_immediate.so` |
| Ping works but FTP/XBDM times out | Check relay logs: `docker compose --env-file .env.1 logs relay` |
| SSH tunnel connects but no data flows | Confirm relay is running: `docker ps \| grep relay` |
| Gamepad stops working in-game | xemu auto-saves input config — `git diff services/xemu/data/emulator/xemu.toml` and revert if port bindings changed |

---

[xemu.app](https://xemu.app) · [halo-scraper](https://github.com/Stewball32/halo-scraper) · GPL-3.0 — built on [linuxserver/docker-xemu](https://github.com/linuxserver/docker-xemu)
