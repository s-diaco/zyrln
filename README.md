# Zephyr

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

## Components

| Component | Path | Role |
|---|---|---|
| Desktop proxy | `platforms/desktop/` | Local HTTPS MITM proxy + reachability probes |
| Relay core | `relay/core/` | Shared Go relay logic used by desktop and Android |
| Mobile bindings | `platforms/mobile/` | gomobile API for Android |
| Apps Script relay | `relay/apps-script/Code.gs` | Google-side relay (the front door) |
| VPS relay | `relay/vps/main.go` | Exit relay running on your server |
| Cloudflare Worker | `relay/cloudflare/worker.js` | Optional alternative exit relay |
| Android app | `android/` | Android VPN app — routes phone traffic through the relay |

## Quick Start

### 1. Deploy the Apps Script relay

1. Open [script.google.com](https://script.google.com) → New project
2. Paste the contents of `relay/apps-script/Code.gs`
3. Set these constants at the top:

```js
const AUTH_KEY      = "your-long-random-secret";   // openssl rand -base64 32
const EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay";
const EXIT_RELAY_KEY = "";                          // optional extra key for the VPS
```

4. Deploy → New deployment → Web app → Execute as: Me → Who has access: Anyone
5. Copy the deployment URL (`https://script.google.com/macros/s/.../exec`)

### 2. Deploy the VPS relay

Copy the binary to your server and run it:

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 go build -o zephyr-relay ./relay/vps/main.go

# Copy to server and run
scp zephyr-relay root@YOUR_VPS:/usr/local/bin/
ssh root@YOUR_VPS "ZEPHYR_RELAY_LISTEN=0.0.0.0:8787 /usr/local/bin/zephyr-relay"
```

See `docs/vps-setup.md` for running it as a systemd service.

### 3. Configure and run the desktop proxy

Create `config.env` (gitignored):

```
fronted-appscript-url = https://script.google.com/macros/s/YOUR_ID/exec
auth-key              = your-long-random-secret
listen                = 127.0.0.1:8085
```

Generate the local CA once:

```bash
make desktop && ./zephyr -init-ca
```

Install `certs/zephyr-ca.pem` as a trusted CA in your browser:
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
# relay fetch ok
# status: 204
```

## Android

See `docs/android-setup.md` for building and installing the Android app.

**Quick summary:**
1. Build the APK: `make android`
2. Install: `adb install android/app/build/outputs/apk/debug/app-debug.apk`
3. Open the app → tap **Install CA Certificate** → follow the steps
4. Enter your Apps Script URL and auth key → tap **Connect**

## Build Reference

```bash
make desktop    # build desktop CLI binary
make proxy      # start desktop proxy (reads config.env)
make test       # smoke test the full relay chain
make aar        # build Android .aar (requires gomobile)
make android    # build Android APK (requires Android SDK)
```

First-time Android build setup:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
export ANDROID_HOME=~/Android/Sdk
make android
```

## Credits

The domain-fronting technique used here — routing traffic through Google Apps Script with a Cloudflare Worker as the exit relay — was pioneered by [denuitt1/mhr-cfw](https://github.com/denuitt1/mhr-cfw). This project takes that core idea and extends it with a self-hosted VPS exit relay, a full Go rewrite, an Android VPN app, and HTTPS MITM proxy support.

## Security Notes

- Each user should deploy their own Apps Script and generate their own auth key
- Never commit `config.env`, `certs/`, or any auth keys
- Rotate your auth key if it appeared in logs or chat
- The local CA private key (`certs/zephyr-ca-key.pem`) must not be shared
- Google and your VPS provider can see metadata (timing, volume) even though they cannot read content
