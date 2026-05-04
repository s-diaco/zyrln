# TODO

## Done

- Desktop Go reachability probes.
- Google-fronted Apps Script flow.
- Apps Script relay template.
- Cloudflare Worker exit template.
- VPS exit relay:
  ```text
  relay/vps
  ```
- Local desktop HTTP proxy:
  ```text
  -serve-proxy
  ```
- Local HTTPS MITM proxy using generated CA:
  ```text
  -init-ca
  ```
- VPS relay deployed and verified with:
  ```text
  https://www.gstatic.com/generate_204 -> 204
  ```

## Current Desktop Path

```text
browser -> local Go proxy -> Google-fronted Apps Script -> VPS relay -> target site
```

Cloudflare Worker is now optional. The VPS relay can replace it.

## Next

- Add a small `examples/` folder with copy-paste commands using placeholders.
- Add a `make` or shell wrapper for:
  - build desktop client
  - build VPS relay
  - run local proxy
  - run relay smoke test
- Add basic automated tests for:
  - relay JSON parsing
  - header filtering
  - fronted URL conversion
- Add a safer config file mode so users do not paste auth keys into shell history.
- Add a release build script for Linux desktop.

## Security Cleanup Before Sharing

- Rotate exposed Apps Script auth key.
- Rotate exposed VPS relay key.
- Rotate VPS root password and prefer SSH keys.
- Do not share:
  ```text
  certs/mhr-local-ca-key.pem
  ```
- Keep `certs/` and `reports/` out of public commits.
