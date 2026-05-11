# TODO

## Next Release

- **macOS Desktop Builds:** Ship unsigned macOS desktop binaries for Apple Silicon (`darwin/arm64`) and Intel (`darwin/amd64`). The desktop GUI is already browser-based and uses embedded HTML assets, so the first release can be CLI/GUI binaries before a polished `.app` bundle.

## Research To Incorporate

- **Google-fronted Direct Fragmenter:** Incorporate the standalone lab probe from `tools/utls-frag-probe` later. Finding: direct Google-service reachability improved when the visible connection target stayed on an allowed Google front while the original end-to-end TLS stream was preserved for the requested service. Integrate this as a direct-mode strategy with fallback and keep the experimental probe out of normal builds for now.
