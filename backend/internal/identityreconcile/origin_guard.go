package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
)

// Registered origins receive a live sync-client token, so they must not point
// at the dashboard's own network: loopback, link-local (including the cloud
// metadata address 169.254.169.254), private (RFC 1918, IPv6 ULA), CGNAT,
// unspecified, multicast and other non-routable addresses are refused, both
// when an origin is saved (after DNS resolution) and when the connector dials
// (on the address actually connected to, which defeats DNS rebinding).
// Hosts listed in IDENTITY_SYNC_ALLOWED_PRIVATE_HOSTS (comma-separated host
// names or IP literals; empty by default) are exempt, for development and e2e
// stacks whose applications run on a private network.

// AllowedPrivateHostsEnv names the allowlist variable.
const AllowedPrivateHostsEnv = "IDENTITY_SYNC_ALLOWED_PRIVATE_HOSTS"

// ErrPrivateOrigin reports an origin that resolves to a non-public address.
var ErrPrivateOrigin = errors.New("identity sync origin resolves to a loopback, link-local, private or otherwise non-public address")

// AllowedPrivateHostsFromEnv parses the allowlist from getenv (nil uses os.Getenv).
func AllowedPrivateHostsFromEnv(getenv func(string) string) []string {
	if getenv == nil {
		getenv = os.Getenv
	}
	var hosts []string
	for _, host := range strings.Split(getenv(AllowedPrivateHostsEnv), ",") {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

var nonPublicPrefixes = func() []netip.Prefix {
	var prefixes []netip.Prefix
	for _, p := range []string{
		"0.0.0.0/8",       // "this network"
		"10.0.0.0/8",      // RFC 1918
		"100.64.0.0/10",   // CGNAT (RFC 6598)
		"127.0.0.0/8",     // loopback
		"169.254.0.0/16",  // link-local, cloud metadata
		"172.16.0.0/12",   // RFC 1918
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // TEST-NET-1
		"192.168.0.0/16",  // RFC 1918
		"198.18.0.0/15",   // benchmarking
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"224.0.0.0/4",     // multicast
		"240.0.0.0/4",     // reserved, broadcast
		"::/128",          // unspecified
		"::1/128",         // loopback
		"64:ff9b:1::/48",  // local-use NAT64
		"100::/64",        // discard
		"2001:db8::/32",   // documentation
		"fc00::/7",        // unique local (ULA), includes fd00:ec2::254 metadata
		"fe80::/10",       // link-local
		"ff00::/8",        // multicast
	} {
		prefixes = append(prefixes, netip.MustParsePrefix(p))
	}
	return prefixes
}()

// nonPublicAddr reports whether addr must not receive a sync token.
func nonPublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
		return true
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// HostResolver resolves a host name to addresses.
type HostResolver func(ctx context.Context, host string) ([]netip.Addr, error)

func defaultResolver(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// originPolicy applies the allowlist and the address rules.
type originPolicy struct {
	allowed  map[string]bool
	resolver HostResolver
}

func newOriginPolicy(allowed []string, resolver HostResolver) originPolicy {
	p := originPolicy{allowed: map[string]bool{}, resolver: resolver}
	for _, host := range allowed {
		if host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]")); host != "" {
			p.allowed[host] = true
		}
	}
	if p.resolver == nil {
		p.resolver = defaultResolver
	}
	return p
}

func (p originPolicy) allowedHost(host string) bool {
	return p.allowed[strings.ToLower(strings.Trim(host, "[]"))]
}

// checkHost resolves host and refuses it when any address is non-public.
func (p originPolicy) checkHost(ctx context.Context, host string) error {
	if p.allowedHost(host) {
		return nil
	}
	host = strings.Trim(host, "[]")
	var addrs []netip.Addr
	if addr, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{addr}
	} else {
		resolved, err := p.resolver(ctx, host)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrOriginUnresolvable, host, err)
		}
		if len(resolved) == 0 {
			return fmt.Errorf("%w: %s: no addresses", ErrOriginUnresolvable, host)
		}
		addrs = resolved
	}
	for _, addr := range addrs {
		if nonPublicAddr(addr) {
			return fmt.Errorf("%w: %s (%s); list the host in %s only for development or e2e stacks", ErrPrivateOrigin, host, addr, AllowedPrivateHostsEnv)
		}
	}
	return nil
}

// dialContext dials addr, refusing a non-public peer address at connect time
// unless the requested host is allowlisted.
func (p originPolicy) dialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if p.allowedHost(host) {
			return dialer.DialContext(ctx, network, addr)
		}
		guarded := *dialer
		guarded.Control = func(_, address string, _ syscall.RawConn) error {
			peer, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(peer)
			if err != nil || nonPublicAddr(ip) {
				return fmt.Errorf("%w: %s connects to %s", ErrPrivateOrigin, host, peer)
			}
			return nil
		}
		return guarded.DialContext(ctx, network, addr)
	}
}
