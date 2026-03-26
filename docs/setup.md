# Setup Guide

This guide walks through setting up the full xemu-cartographer stack: xemu with QMP, the cartographer itself, and optional PocketBase persistence.

---

## 1. xemu prerequisites

The scraper needs two things from xemu:
- A **QMP Unix socket** — used once at startup to translate guest virtual addresses to host addresses
- A **running xemu process** with Halo CE loaded — the scraper reads its memory directly

### Exposing the QMP socket (Podman / Quadlet)

If you're running xemu in a Podman container, mount a host directory into the container for the socket:

Add to your `.container` file:
```ini
Volume=/var/lib/xemu/xemu-host-01/qmp:/qmp
```

Then in your `xemu.toml` under `[machine]`:
```toml
[machine]
qmp_socket_path = '/qmp/xemu-host-01.sock'
```

The socket will appear on the host at `/var/lib/xemu/xemu-host-01/qmp/xemu-host-01.sock`.

### Exposing the QMP socket (bare metal / direct launch)

Pass `-qmp unix:/path/to/xemu.sock,server,nowait` to xemu's command line, or set `qmp_socket_path` in `xemu.toml` pointing to any writable path.

### Verifying the QMP socket

Use `socat` to confirm the socket is up and xemu responds:

```bash
socat - UNIX-CONNECT:/var/lib/xemu/xemu-host-01/qmp/xemu-host-01.sock
```

You should see a JSON capabilities greeting. Type `{"execute":"qmp_capabilities"}` and press Enter to confirm two-way communication.

---

## 2. Configure xemu-cartographer

```bash
cp xemu-cartographer.toml.example xemu-cartographer.toml
```

Edit `xemu-cartographer.toml`. The only required change is the QMP socket path(s):

```toml
[[hosts]]
name     = "xemu-host-01"
qmp_sock = "/var/lib/xemu/xemu-host-01/qmp/xemu-host-01.sock"
```

Add one `[[hosts]]` block per xemu instance. Instances that fail to connect at startup are skipped; the rest continue running.

See the config reference in [README.md](../README.md#configuration) for all options.

---

## 3. Run the scraper

The scraper needs root to read `/proc/<pid>/mem`:

```bash
# Run directly
sudo go run ./cmd/cartographer

# Or build first, then run
go build -o cartographer ./cmd/cartographer
sudo ./cartographer
```

You should see log lines like:
```
config: loaded xemu-cartographer.toml
xemu-host-01: found PID 12345
xemu-host-01: QMP connected, translating addresses...
xemu-host-01: online
ws: listening on :9000
```

Open `web/debug/index.html` in your browser to confirm the WebSocket feed is live.

---

## 4. Run as a systemd service (optional)

Create `/etc/systemd/system/xemu-cartographer.service`:

```ini
[Unit]
Description=Halo CE memory scraper
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cartographer
WorkingDirectory=/etc/xemu-cartographer
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then:
```bash
# Copy binary and config
sudo cp cartographer /usr/local/bin/cartographer
sudo mkdir /etc/xemu-cartographer
sudo cp xemu-cartographer.toml /etc/xemu-cartographer/xemu-cartographer.toml

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now xemu-cartographer
sudo journalctl -fu xemu-cartographer
```

---

## 5. PocketBase (optional persistence)

If you want persistent match records, run PocketBase alongside the scraper:

```bash
# Download from https://pocketbase.io/docs/
./pocketbase serve --http="localhost:8090"
```

Then in `xemu-cartographer.toml`:
```toml
[pocketbase]
enabled = true
url     = "http://localhost:8090"
```

See [docs/pocketbase.md](pocketbase.md) for the collection schema to create.

---

## Troubleshooting

**`permission denied` on `/proc/<pid>/mem`**
The scraper must run as root. Use `sudo`.

**`xemu-host-01: failed to find PID`**
xemu isn't running, or the container name doesn't match. The scraper tries `podman inspect <name>` first, then falls back to scanning `/proc`. Check that the `name` in `xemu-cartographer.toml` matches the running container name.

**`QMP: connection refused` or `no such file`**
The QMP socket path is wrong or the socket hasn't been created yet. Check that xemu started with `qmp_socket_path` set and that the file exists on the host.

**Scraper starts but shows no events**
Halo CE needs to be loaded and in a game. The scraper detects game state via memory flags — it will silently idle until Halo is running.
