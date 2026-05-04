# Cloudflare Worker Setup

The Cloudflare Worker is an alternative exit relay to the VPS. Use it if you don't want to run your own server. The free tier is enough for personal use.

## Architecture with Cloudflare

```
device → local proxy → Google-fronted Apps Script → Cloudflare Worker → target site
```

## Deploy the Worker

1. Go to [dash.cloudflare.com](https://dash.cloudflare.com) → **Workers & Pages** → **Create application** → **Worker**
2. Replace the default code with the contents of `relay/cloudflare/worker.js`
3. Click **Deploy**
4. Copy the Worker URL — it looks like:
   ```
   https://your-worker.your-subdomain.workers.dev
   ```

## Configure Apps Script to Use the Worker

In `relay/apps-script/Code.gs`, set:

```js
const AUTH_KEY       = "YOUR_KEY_FROM_PREREQUISITES";  // same key as before
const EXIT_RELAY_URL = "https://your-worker.your-subdomain.workers.dev/relay";
const EXIT_RELAY_KEY = "";   // leave empty, Cloudflare Workers don't use this
```

Then redeploy the Apps Script web app (Deploy → Manage deployments → create a new version).

## Cloudflare vs VPS

| | Cloudflare Worker | VPS Relay |
|---|---|---|
| Cost | Free tier available | ~$5/mo |
| Setup | Deploy via dashboard | systemd service |
| Control | Less (Cloudflare sees traffic) | Full |
| Reliability | High (global CDN) | Depends on server |
| Request limit | 100k/day free tier | Unlimited |

## Notes

- The free tier has a 10ms CPU time limit per request and 100,000 requests/day — enough for light browsing, not heavy use
- Cloudflare can see the relay traffic metadata (destination URLs) — use the VPS relay if that's a concern
- The Worker code is in `relay/cloudflare/worker.js` and requires no dependencies
