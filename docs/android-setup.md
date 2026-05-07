# Android App Setup

The Android app runs the relay proxy directly on the phone, so all browser traffic is routed through the relay chain without needing a desktop.

## Traffic Path

```
Android app → Go MITM proxy (on-device, port 8085)
            → Google-fronted Apps Script
            → VPS relay
            → target site
```

The app uses Android's `VpnService` to set a system-wide HTTP proxy pointing to the local Go proxy.

## Prerequisites

- Apps Script relay deployed (see [README](../README.md))
- VPS relay running (see [docs/vps-setup.md](vps-setup.md))
- Go 1.25+
- Android SDK with NDK (install NDK via Android Studio: SDK Manager → SDK Tools → NDK (Side by side))
- `ANDROID_HOME` environment variable pointing to your SDK (e.g. `export ANDROID_HOME=~/Android/Sdk`)

## Build

Install gomobile once:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

Build the APK:

```bash
make keystore   # generate signing keystore (run once — creates android/zyrln.jks with default credentials)
make android    # builds the Go AAR and the signed release APK
# APK → android/app/build/outputs/apk/release/zyrln-1.5.1.apk
```

Or a debug build (no keystore needed):

```bash
make android-debug    # compiles .aar and builds debug APK
```

## Install

Copy the APK to your phone and open it. You may need to enable "Install unknown apps" for your file manager in Android settings.

## First Run

### 1. Install the CA Certificate

The app intercepts HTTPS traffic locally (MITM) so it can relay it. Your browser needs to trust the local CA.

1. Open the app and tap **Install CA Certificate**
2. The cert is saved to `Downloads/zyrln-ca.pem`
3. Tap **Open Settings** in the dialog
4. Go to **Biometrics & security → Other security settings → Install from device storage**
5. Browse to Downloads, select `zyrln-ca.pem`
6. Choose **CA certificate** when prompted

This is a one-time step.

⚠️ **Important Note:** Never copy the certificate file from your computer to your phone. The Android app generates its own unique certificate, and using the Windows/Desktop file will result in SSL protocol errors.

### 2. Import your config

On your desktop, make sure `config.env` is set up (see [README](../README.md#3-configure-and-run-the-desktop-proxy)), then export your config as JSON:

```bash
./zyrln -export-config
# prints: {"url":"https://script.google.com/...","key":"your-auth-key"}
```

Copy that JSON, then on your phone:

1. Tap **Import Config from Clipboard**
2. The config is saved to the list automatically (duplicates are skipped)

You can save multiple configs and switch between them.

**Multiple Apps Script URLs:** For better resilience when one deployment is slow, blocked, or out of quota, put multiple Apps Script URLs in the `url` field as a comma-separated list:

```json
{"url":"https://script.google.com/.../exec1,https://script.google.com/.../exec2","key":"your-auth-key"}
```

The app races the configured URLs in parallel and uses the first successful response. Each URL should be a separate Apps Script deployment under a different Google account.

### 3. Connect

1. Tap a config in the list
2. Allow VPN permission when prompted

The status dot turns green and the relay is active. Tap the config again to disconnect. Tap a different config to switch.

### 4. Test

Open Chrome and visit any blocked site. HTTPS should work transparently.

If you see SSL errors, the CA certificate is not trusted yet. Repeat step 1.

## How the VPN Works

The app creates a minimal Android VPN that sets a system HTTP proxy to `127.0.0.1:8085`. Most apps (Chrome, Firefox, system WebView) honor this setting. The local Go proxy:

- For HTTP: relays the request through Apps Script
- For HTTPS: performs local TLS termination (using the installed CA), then relays the decrypted request

Apps that pin their own certificates (some banking/payment apps) will not work through the relay.

## Limitations

- CA cert installation is a one-time manual step (slightly involved on Samsung/Android 10+)

For general limitations see [README](../README.md#limitations).
