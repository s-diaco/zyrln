# TODO

## Next

- First connection cold-start: first few requests after VPN connects are slow, gets faster once
  Apps Script instance and connection pool are warm. Pre-warm on startup with a cheap keepalive
  request so the first real request doesn't pay the cold-start cost.

- Add a safer config file mode so users do not paste auth keys into shell history.
