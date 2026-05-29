#!/bin/sh
set -e

XMPP_DOMAIN="${XMPP_DOMAIN:-meet.echo.local}"
XMPP_AUTH_DOMAIN="${XMPP_AUTH_DOMAIN:-auth.${XMPP_DOMAIN}}"
XMPP_GUEST_DOMAIN="${XMPP_GUEST_DOMAIN:-guest.${XMPP_DOMAIN}}"
XMPP_MUC_DOMAIN="${XMPP_MUC_DOMAIN:-muc.${XMPP_DOMAIN}}"
PUBLIC_URL="${PUBLIC_URL:-https://${XMPP_DOMAIN}}"

# Strip trailing slash
PUBLIC_URL="${PUBLIC_URL%/}"

# Generate config.js: replace placeholder domain with actual values
sed \
    -e "s|https://jitsi-meet.example.com|${PUBLIC_URL}|g" \
    -e "s|wss://jitsi-meet.example.com|wss://${PUBLIC_URL#https://}|g" \
    -e "s|guest.example.com|${XMPP_GUEST_DOMAIN}|g" \
    -e "s|jitsi-meet.example.com|${XMPP_DOMAIN}|g" \
    /srv/echo/config.js.template > /srv/echo/config.js

echo "Echo ready: domain=${XMPP_DOMAIN} guest=${XMPP_GUEST_DOMAIN} url=${PUBLIC_URL}"

exec nginx -g 'daemon off;'
