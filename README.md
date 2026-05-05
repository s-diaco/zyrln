# Zyrln

A domain-fronting relay that routes traffic through Google infrastructure to bypass DPI-based censorship.

## How It Works

```
your device
  → local proxy (Go)
  → Google-fronted Apps Script   ← looks like Google traffic to DPI
  → VPS exit relay
  → target site
```

TLS connections go to Google's IP ranges. The encrypted `Host` header targets your Apps Script deployment. From a DPI perspective the traffic is indistinguishable from normal Google traffic.

## Features

- **Domain-fronting via Google**: all traffic exits from Google IP ranges, indistinguishable from normal Google traffic to DPI filters
- **Full HTTPS support**: local MITM proxy terminates TLS and re-encrypts, so blocked HTTPS sites work transparently
- **Android VPN app**: one-tap connect routes all phone traffic through the relay without root or per-app config
- **Self-hosted exit relay**: run your own VPS exit node (or use a Cloudflare Worker) with no third-party relay services
- **Multi-URL quota failover**: configure multiple Apps Script deployments as a comma-separated list; the relay sticks to the first URL until it hits its 20k/day quota, then automatically switches to the next with no reconnection or downtime. Wraps back to the first when the last one exhausts (quota resets by then)
- **Multiple saved configs**: store and switch between relay configs on Android with a single tap
- **Desktop + Android**: the same relay core powers both the desktop CLI proxy and the Android app via gomobile

## Components

| Component | Path | Role |
|---|---|---|
| Desktop proxy | `platforms/desktop/` | Local HTTPS MITM proxy + reachability probes |
| Relay core | `relay/core/` | Shared Go relay logic used by desktop and Android |
| Mobile bindings | `platforms/mobile/` | gomobile API for Android |
| Apps Script relay | `relay/apps-script/Code.gs` | Google-side relay (the front door) |
| VPS relay | `relay/vps/main.go` | Exit relay running on your server |
| Cloudflare Worker | `relay/cloudflare/worker.js` | Optional alternative exit relay |
| Android app | `android/` | Android VPN app that routes phone traffic through the relay |

## Quick Start

### Prerequisites

**Required tools:**
- Go 1.25+
- `make`

Generate a secret auth key. You will use it in every component:

```bash
openssl rand -base64 32
# example output: 4Xv8mK2...  ← save this
```

### 1. Deploy the Apps Script relay

1. Open [script.google.com](https://script.google.com) → New project
2. Paste the contents of `relay/apps-script/Code.gs`
3. Set these constants at the top:

```js
const AUTH_KEY       = "YOUR_KEY_FROM_PREREQUISITES";
const EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay";
const EXIT_RELAY_KEY = "";   // optional extra key for the VPS relay
```

4. Deploy → New deployment → Web app → Execute as: Me → Who has access: Anyone
5. Copy the deployment URL (`https://script.google.com/macros/s/.../exec`)

### 2. Deploy the VPS relay

See [docs/vps-setup.md](docs/vps-setup.md) for build, systemd service, firewall, and testing.
Alternatively, use a Cloudflare Worker as the exit relay (see [docs/cloudflare-setup.md](docs/cloudflare-setup.md)).

### 3. Configure and run the desktop proxy

Create `config.env` (gitignored):

```
fronted-appscript-url = https://script.google.com/macros/s/YOUR_ID/exec,https://script.google.com/macros/s/BACKUP_ID/exec
auth-key              = YOUR_KEY_FROM_PREREQUISITES
listen                = 127.0.0.1:8085
```

`fronted-appscript-url` accepts a comma-separated list of Apps Script URLs. The proxy sticks to the first URL until it hits its daily quota, then automatically switches to the next one and sticks there. When the last URL is exhausted it wraps back to the first (which has reset by then). Each URL should be a separately deployed Apps Script, ideally under a different Google account to get a separate quota.

Generate the local CA once:

```bash
make desktop && ./zyrln -init-ca
```

Install `certs/zyrln-ca.pem` as a trusted CA in your browser:
- **Chrome/Edge**: Settings → Privacy → Security → Manage certificates → Authorities → Import
- **Firefox**: Settings → Privacy & Security → View Certificates → Authorities → Import

Start the proxy:

```bash
make proxy
```

Set your browser's HTTP and HTTPS proxy to `127.0.0.1:8085`.

### 4. Test

```bash
make test
```

This sends a real request through the full relay chain (Apps Script → VPS → gstatic.com). You should see `relay fetch ok` and `status: 204`. If not, check that your Apps Script deployment and VPS relay are both running and the auth key matches in all three places.

## Android

See [docs/android-setup.md](docs/android-setup.md) for the full build and setup guide.

**Quick summary:**
1. `make keystore && make android`: build signed APK
2. Copy the APK from `android/app/build/outputs/apk/release/` to your phone and install it
3. On desktop: `./zyrln -export-config` → copy the JSON
4. In the app: tap **Import Config from Clipboard** → tap the config to connect

## Build Reference

```bash
make desktop        # build desktop CLI binary
make proxy          # start desktop proxy (reads config.env)
make test           # smoke test the full relay chain
make android        # build signed release APK (requires keystore + Android SDK)
make android-debug  # build debug APK (no keystore needed)
```

See [docs/android-setup.md](docs/android-setup.md) for first-time Android build setup.

## Limitations

- **Browser-based only**: this is an HTTP proxy, not a full VPN. Only browser traffic and apps that respect the system proxy are relayed. Apps like Instagram, WhatsApp, and Telegram bypass it entirely.
- **Apps Script quota**: each Google account gets 20,000 relay requests/day. Heavy sites like YouTube can exhaust this quickly. Each user should deploy their own Apps Script.

## Credits

The domain-fronting technique used here, routing traffic through Google Apps Script with a Cloudflare Worker as the exit relay, was pioneered by [denuitt1/mhr-cfw](https://github.com/denuitt1/mhr-cfw). This project takes that core idea and extends it with a self-hosted VPS exit relay, a full Go rewrite, an Android VPN app, and HTTPS MITM proxy support.

Developed with the assistance of [Claude](https://claude.ai) by Anthropic.

## License

MIT — see [LICENSE](LICENSE).

## Security Notes

- Each user should deploy their own Apps Script and generate their own auth key
- Never commit `config.env`, `certs/`, or any auth keys
- Rotate your auth key if it appeared in logs or chat
- The local CA private key (`certs/zyrln-ca-key.pem`) must not be shared
- Google and your VPS provider can see metadata (timing, volume) even though they cannot read content
