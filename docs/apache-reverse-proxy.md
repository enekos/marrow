# Running Marrow Behind Apache

Marrow is a Go binary that serves HTTP on its own (default `:8080`). Apache cannot
host it the way it hosts PHP — instead, run Marrow as a long-lived service and put
Apache in front as a reverse proxy. This is the same pattern shown in the
[Caddyfile](../Caddyfile), translated to Apache `httpd`.

## Prerequisites

- Apache 2.4+ installed
- Marrow installed and running (e.g. via the systemd unit in [`marrow.service`](../marrow.service))
- A domain / subdomain pointing at the host
- Root / `sudo` access

## 1. Run Marrow as a service

Marrow listens on `:8080` by default (see `[server].addr` in your `config.toml`).
Bind it to loopback so only Apache can reach it:

```toml
[server]
addr = "127.0.0.1:8080"
```

Install and start the systemd unit:

```bash
sudo cp marrow.service /etc/systemd/system/marrow.service
sudo systemctl daemon-reload
sudo systemctl enable --now marrow
sudo systemctl status marrow
```

Verify it responds locally:

```bash
curl -i http://127.0.0.1:8080/healthz
```

## 2. Enable required Apache modules

```bash
sudo a2enmod proxy proxy_http headers rewrite
# only if you use TLS:
sudo a2enmod ssl
sudo systemctl reload apache2
```

On RHEL / CentOS / Rocky these modules are usually already loaded; check
`/etc/httpd/conf.modules.d/` for `mod_proxy`, `mod_proxy_http`, `mod_headers`,
`mod_rewrite`.

## 3. VirtualHost configuration

Create `/etc/apache2/sites-available/marrow.conf` (Debian/Ubuntu) or
`/etc/httpd/conf.d/marrow.conf` (RHEL family):

```apache
<VirtualHost *:80>
    ServerName search.blog.example.com

    ProxyPreserveHost On
    ProxyRequests Off

    ProxyPass        / http://127.0.0.1:8080/
    ProxyPassReverse / http://127.0.0.1:8080/

    # Forward client info so Marrow sees the real host/IP
    RequestHeader set X-Forwarded-Proto "http"
    RequestHeader set X-Forwarded-Host  "%{HTTP_HOST}e"

    # CORS — mirror the per-site rules from the Caddyfile
    Header set Access-Control-Allow-Origin  "https://blog.example.com"
    Header set Access-Control-Allow-Methods "GET, POST, OPTIONS"
    Header set Access-Control-Allow-Headers "Content-Type, Authorization, X-Site"

    ErrorLog  ${APACHE_LOG_DIR}/marrow_error.log
    CustomLog ${APACHE_LOG_DIR}/marrow_access.log combined
</VirtualHost>
```

Enable and reload (Debian/Ubuntu):

```bash
sudo a2ensite marrow
sudo apachectl configtest
sudo systemctl reload apache2
```

## 4. TLS with Let's Encrypt

Use `certbot` to add HTTPS automatically:

```bash
sudo apt install certbot python3-certbot-apache
sudo certbot --apache -d search.blog.example.com
```

Certbot rewrites the VirtualHost to listen on `:443` with the certificate paths
filled in and adds an HTTP→HTTPS redirect. After it runs, change the forwarded
proto header to match:

```apache
RequestHeader set X-Forwarded-Proto "https"
```

## 5. Multiple sites

Marrow routes by `Host` header (see [multi-site.md](./multi-site.md)). For each
host listed under `[[sites]]` in `config.toml`, add a separate `<VirtualHost>`
block with its own `ServerName` and CORS origin — all of them proxy to the same
`http://127.0.0.1:8080/` backend. `ProxyPreserveHost On` ensures Marrow sees the
original hostname and picks the right site config.

## Troubleshooting

- **502 Bad Gateway** — Marrow isn't running, or it's listening on a different
  address. Check `systemctl status marrow` and `ss -ltnp | grep 8080`.
- **403 from Apache** — SELinux on RHEL blocks outbound proxy connections by
  default: `sudo setsebool -P httpd_can_network_connect 1`.
- **Wrong site served** — `ProxyPreserveHost On` is missing, so Marrow sees
  `127.0.0.1` instead of the real host and falls back to the default site.
- **CORS errors in the browser** — the `Access-Control-Allow-Origin` header
  must match the origin of the page embedding the search widget exactly
  (scheme + host, no trailing slash).
