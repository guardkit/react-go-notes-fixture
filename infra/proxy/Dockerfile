# Caddy 2.10.2 serving the built React files, /api and /auth on one port per
# listener. Build context: the fixture root (see /.dockerignore); the web
# bundle is built beforehand by scripts/build-artifacts.sh.
FROM caddy:2.10.2-alpine@sha256:4c6e91c6ed0e2fa03efd5b44747b625fec79bc9cd06ac5235a779726618e530d
COPY infra/proxy/Caddyfile /etc/caddy/Caddyfile
COPY infra/proxy/public-on.caddy infra/proxy/public-off.caddy /etc/caddy/
COPY apps/web/dist /srv
