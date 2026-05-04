# VPS Relay Setup

The VPS relay is the exit node — it receives requests from Apps Script and fetches the real target URL.

## Build and Deploy

On your local machine, cross-compile for Linux:

```bash
GOOS=linux GOARCH=amd64 go build -o zephyr-relay ./relay/vps/main.go
```

Copy to the server:

```bash
scp zephyr-relay root@YOUR_VPS:/usr/local/bin/
```

## Run as a systemd Service

Create `/etc/systemd/system/zephyr-relay.service`:

```ini
[Unit]
Description=Zephyr Exit Relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/zephyr-relay.env
ExecStart=/usr/local/bin/zephyr-relay
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

Create `/etc/zephyr-relay.env`:

```
ZEPHYR_RELAY_LISTEN=0.0.0.0:8787
ZEPHYR_RELAY_KEY=your-optional-relay-key
```

Enable and start:

```bash
systemctl daemon-reload
systemctl enable --now zephyr-relay
```

## Open the Firewall

Allow inbound traffic on port 8787:

```bash
ufw allow 8787/tcp
```

(Skip if your VPS provider manages firewall rules via a dashboard instead.)

## Test

```bash
# If ZEPHYR_RELAY_KEY is set, include the header; omit it if key is empty.
curl -X POST http://YOUR_VPS:8787/relay \
  -H "Content-Type: application/json" \
  -H "X-Relay-Key: your-optional-relay-key" \
  -d '{"u":"https://www.gstatic.com/generate_204","m":"GET","h":{},"r":true}'
# {"s":204,...}
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-listen` | `127.0.0.1:8787` | Listen address (use `0.0.0.0:8787` for public) |
| `-key` | `""` | Optional auth key required in `X-Relay-Key` header |
| `-timeout` | `45s` | Timeout for requests to the target |

## Using Cloudflare Worker Instead

If you prefer not to run a VPS, use `relay/cloudflare/worker.js` as the exit relay.
Deploy it on Cloudflare Workers (free tier works) and set `EXIT_RELAY_URL` in your Apps Script to the Worker URL.
