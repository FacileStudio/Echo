#!/bin/sh
set -e

XMPP_DOMAIN="${XMPP_DOMAIN:-meet.jitsi}"
PUBLIC_URL="${PUBLIC_URL:-https://${XMPP_DOMAIN}}"

# Strip trailing slash
PUBLIC_URL="${PUBLIC_URL%/}"

# Generate config.js: replace placeholder domain with actual values
sed \
    -e "s|https://jitsi-meet.example.com|${PUBLIC_URL}|g" \
    -e "s|wss://jitsi-meet.example.com|wss://${PUBLIC_URL#https://}|g" \
    -e "s|jitsi-meet.example.com|${XMPP_DOMAIN}|g" \
    /srv/echo/config.js.template > /srv/echo/config.js

# Cache busting: replace __ECHO_VERSION__ placeholder in index.html
ECHO_VERSION="$(cat /tmp/echo-version 2>/dev/null || date +%s)"
sed -i "s|__ECHO_VERSION__|${ECHO_VERSION}|g" /srv/echo/index.html

echo "Echo ready: domain=${XMPP_DOMAIN} url=${PUBLIC_URL} version=${ECHO_VERSION}"

exec nginx -g 'daemon off;'
