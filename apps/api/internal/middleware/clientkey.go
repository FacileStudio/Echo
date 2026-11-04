package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// ClientAddr is the visitor's address for rate limiting.
//
// Production's Traefik deletes the X-Forwarded-For Cloudflare sends and
// writes only the Cloudflare edge it received from, so after tronc's RealIP
// the request still carries Traefik's own container address. Cloudflare's
// Cf-Connecting-Ip then names the visitor, and it is believed only when the
// chain is the genuine one: the peer is on the private network, and the
// rightmost X-Forwarded-For hop, which Traefik itself wrote, is a Cloudflare
// edge. A request that reached Traefik any other way has its own address as
// that hop, so a Cf-Connecting-Ip it forged is never read. Every other
// request keys on RemoteAddr as RealIP resolved it.
func ClientAddr(r *http.Request) netip.Addr {
	peer := hostAddr(r.RemoteAddr)
	if !peer.IsValid() || !troncmiddleware.TrustedBy(peer, troncmiddleware.DefaultTrustedProxies) {
		return peer
	}
	if !troncmiddleware.TrustedBy(rightmostHop(r), troncmiddleware.CloudflareProxies) {
		return peer
	}
	visitor, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("Cf-Connecting-Ip")))
	if err != nil {
		return peer
	}
	return visitor.Unmap()
}

// rightmostHop is the last X-Forwarded-For entry, the one the nearest proxy
// wrote, or the zero address when there is none or it does not parse.
func rightmostHop(r *http.Request) netip.Addr {
	forwarded := r.Header.Values("X-Forwarded-For")
	if len(forwarded) == 0 {
		return netip.Addr{}
	}
	hops := strings.Split(forwarded[len(forwarded)-1], ",")
	return hostAddr(strings.TrimSpace(hops[len(hops)-1]))
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
