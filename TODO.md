# TODO

## Next Release

- **macOS Desktop Builds:** Ship unsigned macOS desktop binaries for Apple Silicon (`darwin/arm64`) and Intel (`darwin/amd64`). The desktop GUI is already browser-based and uses embedded HTML assets, so the first release can be CLI/GUI binaries before a polished `.app` bundle.

## Future: Direct Mode Auto-Probe

- **Goal:** Make Direct Mode adaptive instead of hardcoded. On first run, or when the user clicks "Optimize Direct Mode," run a local fragmentation probe, test several safe profiles, detect which one works best for the user's ISP/path, save that profile locally, and use it for Direct Mode.
- **Probe matrix:** Test a small predefined set of front domains, fragment profiles, delay profiles, and split styles. Keep it bounded and intentional, not random guessing forever.
- **Front domains:** Start with known Google fronts such as `www.google.com` and `script.google.com`, with room to add other allowed Google fronts later.
- **Profiles:** Test small, medium, and high fragmentation profiles; low, medium, and high delay profiles; equal chunks, TLS-header split, SNI-aware split, and padded pre-SNI split styles.
- **Targets:** Probe representative Google services such as YouTube, Gmail, Meet, Drive, Docs, Gemini, and Maps.
- **Selection:** Record success/fail, handshake time, and repeat stability. Choose the fastest stable profile, preferably one that succeeds 2/2 or 3/3 times.
- **Saved config:** Persist the selected profile locally, for example `direct-front-domain` and `fragment-profile`, so Direct Mode can reuse it automatically.
- **Fallback:** If the saved profile starts failing repeatedly, fall back to the default profile and suggest running optimization again.
- **Reporting:** Public reports should not expose exact fragment or timing values. They can show target, result, and handshake time, while profile details remain local.
- **Principle:** The code is open source, so do not rely on secrecy. The value is adaptability across ISP, city, carrier, and time.
