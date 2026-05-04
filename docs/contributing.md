# Contributing

For a high-level overview of what each component does, see the [Components table in the README](../README.md#components).

## Project Structure

```
zyrln/
├── platforms/
│   ├── desktop/        # Desktop CLI binary (main package)
│   │   ├── main.go     # CLI flags, probe runner, relay-fetch, proxy launcher
│   │   └── main_test.go
│   └── mobile/         # gomobile bindings for Android
│       └── mobile.go   # Exported API: Start, Stop, IsRunning, GenerateCA
│
├── relay/
│   ├── core/           # Shared relay logic (imported by desktop and mobile)
│   │   ├── relay.go    # RelayRequest, domain-fronted HTTP, payload encoding
│   │   ├── proxy.go    # StartProxy, HTTP+HTTPS MITM handler
│   │   ├── cert.go     # GenerateCA, LoadCA, CertForHost (per-host leaf certs)
│   │   └── *_test.go
│   ├── apps-script/
│   │   └── Code.gs     # Google Apps Script relay (runs on Google's servers)
│   ├── vps/
│   │   └── main.go     # Exit relay binary for a self-hosted VPS
│   └── cloudflare/
│       └── worker.js   # Alternative exit relay as a Cloudflare Worker
│
├── android/            # Android Studio project
│   └── app/src/main/java/com/zyrln/relay/
│       ├── MainActivity.kt      # UI: connect/disconnect, CA cert install flow
│       └── RelayVpnService.kt   # VpnService: starts Go proxy, sets system proxy
│
├── docs/               # Setup guides
├── Makefile
└── go.mod
```

## Key Concepts

**`relay/core`** is the heart of the project. Both the desktop binary and the Android app import it.

- `relay.go` — builds and sends a relay request through domain-fronting. The domain-fronting trick: `req.URL.Host` is set to the front domain (e.g. `www.google.com`) so TLS connects to Google's IPs, while `req.Host` carries the real Apps Script hostname inside the encrypted TLS tunnel.
- `proxy.go` — HTTP proxy that intercepts browser traffic. HTTP requests are relayed directly; HTTPS connections use `CONNECT` tunneling with local TLS termination (MITM).
- `cert.go` — generates a local CA and signs per-hostname leaf certificates on demand, cached in memory.

**`platforms/mobile`** exposes a flat string-based API (`Start`, `Stop`, etc.) because gomobile only supports primitive types at the boundary. All errors are returned as strings, not Go `error` values.

## Running Tests

```bash
go test ./relay/core/... ./platforms/desktop/...
```

Or everything at once:

```bash
go test ./...
```

Tests use only the standard library — no external test frameworks.

## Building

```bash
make desktop          # build ./zyrln CLI binary
make aar              # build Android .aar (requires gomobile + NDK)
make android-debug    # build debug APK (no keystore needed)
make android          # build signed release APK (requires keystore)
```

First-time gomobile setup:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
export ANDROID_HOME=~/Android/Sdk
```

## Adding a New Probe

Probes are defined in `platforms/desktop/main.go` in the `defaultProbes()` function. Each probe is a `probe` struct:

```go
{
    ID:          "unique-id",
    Name:        "Human readable name",
    Category:    "baseline",   // used for --category filtering
    Method:      http.MethodGet,
    URL:         "https://example.com/",
    Expectation: "what a passing result means",
}
```

## Changing the Relay Protocol

The relay payload format is defined in `relay/core/relay.go` (`buildRelayPayload`) and must match what `relay/apps-script/Code.gs` expects. If you change either side, update both.

The Apps Script response format is `workerResponse` in `relay.go`:

```go
type workerResponse struct {
    Status  int               `json:"s"`
    Headers map[string]string `json:"h"`
    Body    string            `json:"b"` // base64-encoded
    Error   string            `json:"e"`
}
```

## Secrets and Gitignore

Never commit:
- `config.env` — contains your Apps Script URL and auth key
- `certs/` — contains the local CA private key
- Any file containing `AUTH_KEY` or relay keys

These are covered by `.gitignore`. See [Key Generation in the README](../README.md#prerequisites) for how to generate a key.
