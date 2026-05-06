# TODO

## High Priority

- **Android Manual Config (#6):** Add a "Manual Entry" dialog to the Android app. Users should be able to type in their Apps Script URL and Auth Key directly without relying on JSON/Clipboard.
- **Custom Proxy IP (#7):** Allow specifying a fixed IP for the domain-fronted connection to bypass specific IP-based blocks on Google's edge.

## Performance & Stealth

- **HTTP/3 (QUIC) Support:** Implement QUIC for the client-to-Google connection. This will reduce handshake latency and improve stability under packet loss (0-RTT support).
- **Proactive URL Rotation (#7):** Add an optional mode to rotate to the next Apps Script URL every ~500 requests to distribute load across multiple Google accounts.
- **Quota Tracking (#7):** Add a simple UI counter to estimate daily request usage against the 20k Google quota.

## Maintenance & Features

- **Safer Config Mode:** Add a way to pass the auth key via a secure file or environment variable to avoid it appearing in shell history.
- **Download Path (#2):** Allow Android users to customize the storage location for downloaded assets.
- **Chain Proxy (#7):** Explore supporting an upstream proxy (SSH/VLESS) at the exit relay level.
