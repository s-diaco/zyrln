# VPS Relay Setup

The VPS relay is the exit node. It receives requests from Apps Script and fetches the real target URL.

## Build and Deploy

On your local machine, cross-compile for Linux:

```bash
GOOS=linux GOARCH=amd64 go build -o zyrln-relay ./relay/vps/main.go
```

Copy to the server:

```bash
scp zyrln-relay root@YOUR_VPS:/usr/local/bin/
```

## Run as a systemd Service

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

Create `/etc/zyrln-relay.env`:

```
ZYRLN_RELAY_LISTEN=0.0.0.0:8787
ZYRLN_RELAY_KEY=your-optional-relay-key
```

If you set `ZYRLN_RELAY_KEY`, you must set the same value in Apps Script's `EXIT_RELAY_KEY` constant. Otherwise Apps Script won't be able to reach the VPS and all relay requests will fail with 401.

Enable and start:

```bash
systemctl daemon-reload
systemctl enable --now zyrln-relay
```

## Open the Firewall

Allow inbound traffic on port 8787:

```bash
ufw allow 8787/tcp
```

(Skip if your VPS provider manages firewall rules via a dashboard instead.)

## Test

```bash
# If ZYRLN_RELAY_KEY is set, include the header; omit it if key is empty.
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
See [docs/cloudflare-setup.md](cloudflare-setup.md) for deployment instructions.
