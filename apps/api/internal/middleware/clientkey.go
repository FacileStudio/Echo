package middleware

import (
	"net"
	"net/http"
	"net/netip"
)

// CloudflareHeader is the header Cloudflare names the visitor in. tronc's
// RealIP believes it only when the rightmost X-Forwarded-For hop, the one
// Traefik wrote, is a Cloudflare edge (httpx.Config.CDNProxies).
const CloudflareHeader = "Cf-Connecting-Ip"

// ClientAddr is the visitor's address for rate limiting: the host of
// RemoteAddr as tronc's RealIP rewrote it. Production's Traefik replaces the
// X-Forwarded-For Cloudflare sends with the edge alone, so RealIP recovers the
// visitor from CloudflareHeader on the genuine chain and leaves RemoteAddr
// alone on every other request. Nothing here reads a forwarded header.
func ClientAddr(r *http.Request) netip.Addr {
	return hostAddr(r.RemoteAddr)
}

// hostAddr parses an address with or without a port.
func hostAddr(raw string) netip.Addr {
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		host = raw
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

// addressKeys are the buckets one address spends from: itself for IPv4;
// its /64, which one subscriber usually holds whole, and its /48, so a
// network cannot rotate through /64s for fresh budgets, for IPv6.
func addressKeys(addr netip.Addr) []string {
	if !addr.IsValid() {
		return []string{"ip:unknown"}
	}
	addr = addr.Unmap()
	if !addr.Is6() {
		return []string{"ip:" + addr.String()}
	}
	p64, _ := addr.Prefix(64)
	p48, _ := addr.Prefix(48)
	return []string{"ip:" + p64.String(), "net:" + p48.String()}
}

// AddressKey is the narrowest key an address is limited by: itself for IPv4,
// its /64 for IPv6. Anything that holds one thing per client address keys on
// it, so an IPv6 subscriber cannot rotate through their /64.
func AddressKey(addr netip.Addr) string {
	return addressKeys(addr)[0]
}
