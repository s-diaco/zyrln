# راه‌اندازی رله روی VPS

VPS گره خروجی است — سایت‌های واقعی را از طرف Apps Script باز می‌کند.

## پیش‌نیازها

- یک VPS لینوکس (amd64 یا arm64) با IP عمومی
- Go نسخه ۱.۲۵+ روی ماشین محلی (برای کامپایل)
- پورت ۸۷۸۷ در فایروال باز باشد

## ساخت

روی ماشین محلی:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
# برای amd64 (اکثر VPS‌ها)
GOOS=linux GOARCH=amd64 go build -o zyrln-relay ./relay/vps/main.go

# برای arm64 (Oracle free tier و غیره)
GOOS=linux GOARCH=arm64 go build -o zyrln-relay ./relay/vps/main.go
```

</div>

انتقال به سرور:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
scp zyrln-relay root@IP_VPS_شما:/usr/local/bin/
```

</div>

## اجرا به عنوان سرویس systemd

فایل `/etc/zyrln-relay.env` را بساز:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```
ZYRLN_RELAY_LISTEN=0.0.0.0:8787
ZYRLN_RELAY_KEY=
```

</div>

> `ZYRLN_RELAY_KEY` اختیاری است. اگر مقدار تنظیم کردی، همان مقدار را در `EXIT_RELAY_KEY` در Apps Script هم قرار بده. اگر نیازی نداری، هر دو را خالی بگذار.

فایل `/etc/systemd/system/zyrln-relay.service` را بساز:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

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

</div>

فعال‌سازی و اجرا:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
systemctl daemon-reload
systemctl enable --now zyrln-relay
systemctl status zyrln-relay   # باید "active (running)" نشان بدهد
```

</div>

## باز کردن فایروال

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
ufw allow 8787/tcp
```

</div>

اگر VPS تو از طریق داشبورد وب فایروال را مدیریت می‌کند، این مرحله را آنجا انجام بده.

## تست

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
curl -s -X POST http://IP_VPS_شما:8787/relay \
  -H "Content-Type: application/json" \
  -d '{"u":"https://www.gstatic.com/generate_204","m":"GET","h":{},"r":true}'
# باید خروجی {"s":204,...} بگیری
```

</div>

## پارامترهای موجود

| پارامتر | پیش‌فرض | توضیح |
|---|---|---|
| `-listen` | `127.0.0.1:8787` | آدرس گوش دادن — برای اتصال خارجی از `0.0.0.0:8787` استفاده کن |
| `-key` | `""` | کلید امنیتی اختیاری که در هدر `X-Relay-Key` چک می‌شود |
| `-timeout` | `45s` | تایم‌اوت برای دریافت پاسخ از سایت مقصد |
