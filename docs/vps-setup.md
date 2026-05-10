# VPS Relay Setup

The VPS is the exit node — it fetches real websites on behalf of Apps Script.

## Requirements

- A Linux VPS (amd64 or arm64) with a public IP
- Go 1.25+ on your local machine (for cross-compiling)
- Port 8787 open on the firewall

## Build

On your local machine:

```bash
# For amd64 (most VPS providers)
GOOS=linux GOARCH=amd64 go build -o zyrln-relay ./relay/vps/main.go

# For arm64 (Oracle free tier, etc.)
GOOS=linux GOARCH=arm64 go build -o zyrln-relay ./relay/vps/main.go
```

Copy to the server:

```bash
scp zyrln-relay root@YOUR_VPS_IP:/usr/local/bin/
```

## Run as a systemd Service

Create `/etc/zyrln-relay.env`:

```
ZYRLN_RELAY_LISTEN=0.0.0.0:8787
ZYRLN_RELAY_KEY=
```

> `ZYRLN_RELAY_KEY` is optional. If you set it, put the same value in `EXIT_RELAY_KEY` in your Apps Script. Leave both empty if you don't need it.

Create `/etc/systemd/system/zyrln-relay.service`:

```ini
[Unit]
Description=Zyrln Exit Relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/zyrln-relay.env
ExecStart=/usr/local/bin/zyrln-relay
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
systemctl daemon-reload
systemctl enable --now zyrln-relay
systemctl status zyrln-relay   # should show "active (running)"
```

## Open the Firewall

```bash
ufw allow 8787/tcp
```

Skip if your VPS provider manages firewall rules via a web dashboard.

## Verify It's Working

```bash
curl -s -X POST http://YOUR_VPS_IP:8787/relay \
  -H "Content-Type: application/json" \
  -d '{"u":"https://www.gstatic.com/generate_204","m":"GET","h":{},"r":true}'
# expected: {"s":204,...}
```

## Available Flags

| Flag | Default | Description |
|---|---|---|
| `-listen` | `127.0.0.1:8787` | Listen address — use `0.0.0.0:8787` to accept external connections |
| `-key` | `""` | Optional auth key checked in `X-Relay-Key` request header |
| `-timeout` | `45s` | Timeout for fetching target URLs |
