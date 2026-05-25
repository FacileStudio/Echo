#!/bin/sh
set -e

XMPP_DOMAIN="${XMPP_DOMAIN:-meet.echo.local}"
PUBLIC_URL="${PUBLIC_URL:-https://${XMPP_DOMAIN}}"

# Strip trailing slash
PUBLIC_URL="${PUBLIC_URL%/}"

# Generate config.js: replace placeholder domain with actual values
sed \
    -e "s|https://jitsi-meet.example.com|${PUBLIC_URL}|g" \
    -e "s|wss://jitsi-meet.example.com|wss://${PUBLIC_URL#https://}|g" \
    -e "s|jitsi-meet.example.com|${XMPP_DOMAIN}|g" \
    /srv/echo/config.js.template > /srv/echo/config.js

echo "Echo ready: domain=${XMPP_DOMAIN} url=${PUBLIC_URL}"

exec nginx -g 'daemon off;'
