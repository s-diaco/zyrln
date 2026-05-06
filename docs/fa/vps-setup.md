# راه‌اندازی رله روی سرور مجازی (VPS)

رله VPS به عنوان گره خروجی (Exit Node) عمل می‌کند. این بخش درخواست‌ها را از اسکریپت گوگل دریافت کرده و محتوای واقعی سایت مقصد را فراخوانی می‌کند.

## پیش‌نیازها

- نصب Go نسخه ۱.۲۵ یا بالاتر روی سیستم محلی (برای کامپایل کردن).
- یک سرور مجازی لینوکس (معمولاً amd64). اگر از پردازنده‌های ARM استفاده می‌کنید، در دستور ساخت مقدار `GOARCH=amd64` را به `GOARCH=arm64` تغییر دهید.

## ساخت و انتقال (Build and Deploy)

روی سیستم محلی خود، برنامه را برای لینوکس کامپایل کنید:

```bash
GOOS=linux GOARCH=amd64 go build -o zyrln-relay ./relay/vps/main.go
```

فایل ساخته شده را به سرور منتقل کنید:

```bash
scp zyrln-relay root@YOUR_VPS_IP:/usr/local/bin/
```

## اجرا به عنوان سرویس (systemd)

فایل `/etc/systemd/system/zyrln-relay.service` را بسازید:

```ini
[Unit]
Description=Zyrln Exit Relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/zyrln-relay.env
ExecStart=/usr/local/bin/zyrln-relay
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

فایل تنظیمات محیطی `/etc/zyrln-relay.env` را بسازید:

```
ZYRLN_RELAY_LISTEN=0.0.0.0:8787
ZYRLN_RELAY_KEY=your-optional-relay-key
```

اگر مقدار `ZYRLN_RELAY_KEY` را تنظیم کردید، حتماً باید همین مقدار را در ثابت `EXIT_RELAY_KEY` در اسکریپت گوگل (Apps Script) نیز وارد کنید. در غیر این صورت، اسکریپت گوگل نمی‌تواند به VPS متصل شود و تمام درخواست‌ها با خطای 401 مواجه می‌شوند.

فعال‌سازی و اجرا:

```bash
systemctl daemon-reload
systemctl enable --now zyrln-relay
systemctl status zyrln-relay   # باید وضعیت active (running) را نشان دهد
```

## تنظیم دیوار آتش (Firewall)

اجازه ورود ترافیک روی پورت ۸۷۸۷ را بدهید:

```bash
ufw allow 8787/tcp
```

(اگر ارائه‌دهنده VPS شما از طریق داشبورد وب دیوار آتش را مدیریت می‌کند، این مرحله را در آنجا انجام دهید.)

## تست نهایی

```bash
# اگر کلید تنظیم کرده‌اید، هدر X-Relay-Key را هم بفرستید؛ در غیر این صورت آن را حذف کنید.
curl -X POST http://YOUR_VPS_IP:8787/relay \
  -H "Content-Type: application/json" \
  -H "X-Relay-Key: your-optional-relay-key" \
  -d '{"u":"https://www.gstatic.com/generate_204","m":"GET","h":{},"r":true}'
# باید خروجی {"s":204,...} دریافت کنید.
```

## پارامترها (Flags)

| پارامتر | پیش‌فرض | توضیحات |
|---|---|---|
| `-listen` | `127.0.0.1:8787` | آدرس گوش دادن (از `0.0.0.0:8787` برای دسترسی عمومی استفاده کنید) |
| `-key` | `""` | کلید اختیاری برای امنیت بیشتر که در هدر `X-Relay-Key` چک می‌شود |
| `-timeout` | `45s` | حداکثر زمان انتظار برای دریافت پاسخ از مقصد |

## جایگزین: استفاده از Cloudflare Worker

اگر تمایلی به استفاده از VPS ندارید، می‌توانید از `relay/cloudflare/worker.js` به عنوان رله خروجی استفاده کنید.
راهنمای نصب: [docs/fa/cloudflare-setup.md](cloudflare-setup.md)
