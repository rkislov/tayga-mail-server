# Tayga Mail — Thunderbird WebExtension

Companion add-on for **Thunderbird 115+** (MailExtensions).

## Features

- Sign-in to Tayga (`/api/v1/auth/login`) and store Bearer token
- Apply a dark Tayga-inspired theme
- Edit **Sieve** scripts and **vacation** via REST
- Helper for **large-file share links** (`/api/v1/files` + `/s/{token}`)
- Points users at Mozilla Autoconfig + CalDAV/CardDAV URLs

Mail account creation still uses server autoconfig:

`{public_url}/.well-known/autoconfig/mail/config-v1.1.xml`

CalDAV / CardDAV base: `{public_url}/dav/`

## Install (temporary)

1. Tools → Add-ons → gear → *Debug Add-ons* → *Load Temporary Add-on*
2. Select `manifest.json` from this folder

## Package

```bash
cd extensions/thunderbird-tayga
zip -r ../../dist/tayga-thunderbird-v0.9.0.xpi . -x '*.md'
```

## Icon

Add a 48×48 `icon-48.png` before publishing (optional for temporary load).
