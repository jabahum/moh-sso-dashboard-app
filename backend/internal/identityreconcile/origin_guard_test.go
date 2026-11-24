package identityreconcile

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// An enabled save of a loopback origin must be refused before any token is
// minted or any request reaches the origin.
func TestRegistrySaveRefusesLoopbackOriginWithoutContactingIt(t *testing.T) {
	var appCalls atomic.Int32
	registry, _, origin := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		appCalls.Add(1)
		_, _ = io.WriteString(w, descriptorJSON("1.1", "alerts", "opaque", "accounts.list", "identity.link", "sessions.revoke"))
	})
	registry.AllowedPrivateHosts = nil // production default: nothing allowlisted
	integration := validIntegration()
	integration.Origin = origin // https://127.0.0.1:<port>
	integration.Enabled = true
	_, err := registry.Save(context.Background(), integration, "operator", "onboard")
	if !errors.Is(err, ErrPrivateOrigin) {
		t.Fatalf("saving a loopback origin: err = %v, want ErrPrivateOrigin", err)
	}
	if appCalls.Load() != 0 {
		t.Fatalf("the loopback origin received %d request(s) carrying a sync token", appCalls.Load())
	}
}

func TestRegistrySaveChecksResolvedAddresses(t *testing.T) {
	registry, repo, _ := registryFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request may be sent")
	})
	registry.AllowedPrivateHosts = nil
	resolved := map[string][]netip.Addr{
		"metadata.example.org": {netip.MustParseAddr("169.254.169.254")},
		"intranet.example.org": {netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("10.1.2.3")},
		"cgnat.example.org":    {netip.MustParseAddr("100.64.0.7")},
		"ula.example.org":      {netip.MustParseAddr("fd00:ec2::254")},
	}
	registry.Resolver = func(_ context.Context, host string) ([]netip.Addr, error) {
		if addrs, ok := resolved[host]; ok {
			return addrs, nil
		}
		return nil, errors.New("no such host")
	}
	for host := range resolved {
		integration := validIntegration()
		integration.Origin = "https://" + host
		integration.Enabled = true
		if _, err := registry.Save(context.Background(), integration, "operator", "onboard"); !errors.Is(err, ErrPrivateOrigin) {
			t.Errorf("%s: err = %v, want ErrPrivateOrigin", host, err)
		}
		// Even a disabled save may not store a non-public origin.
		integration.Enabled = false
		if _, err := registry.Save(context.Background(), integration, "operator", "park"); !errors.Is(err, ErrPrivateOrigin) {
			t.Errorf("%s disabled: err = %v, want ErrPrivateOrigin", host, err)
		}
	}
	if len(repo.rows) != 0 {
		t.Fatalf("a non-public origin was stored: %+v", repo.rows)
	}
}

func TestNonPublicAddresses(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.0.0.1", "172.16.5.4", "192.168.1.1", "169.254.169.254", "100.64.0.1",
		"0.0.0.0", "::", "::1", "fe80::1", "fc00::1", "fd12:3456::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1", "224.0.0.1", "255.255.255.255"} {
		if !nonPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s accepted as public", s)
		}
	}
	for _, s := range []string{"93.184.216.34", "8.8.8.8", "2606:4700:4700::1111"} {
		if nonPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s refused as non-public", s)
		}
	}
}

func TestOriginPolicyAllowlistAndEnv(t *testing.T) {
	hosts := AllowedPrivateHostsFromEnv(func(string) string { return " alerts.internal , 127.0.0.1,," })
	if strings.Join(hosts, ",") != "alerts.internal,127.0.0.1" {
		t.Fatalf("hosts = %v", hosts)
	}
	policy := newOriginPolicy(hosts, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("10.0.0.5")}, nil
	})
	if err := policy.checkHost(context.Background(), "alerts.internal"); err != nil {
		t.Fatalf("allowlisted host refused: %v", err)
	}
	if err := policy.checkHost(context.Background(), "other.internal"); !errors.Is(err, ErrPrivateOrigin) {
		t.Fatalf("other private host: %v", err)
	}
	if hosts := AllowedPrivateHostsFromEnv(func(string) string { return "" }); len(hosts) != 0 {
		t.Fatalf("default allowlist must be empty: %v", hosts)
	}
}

// The dial-time check runs on the address actually connected to, so a host
// that passed the registration check and later resolves to loopback (DNS
// rebinding) is still refused.
func TestOriginPolicyDialRefusesNonPublicPeer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	ctx := context.Background()

	deny := newOriginPolicy(nil, nil)
	for _, host := range []string{"127.0.0.1", "localhost"} {
		if _, err := deny.dialContext(dialer)(ctx, "tcp", net.JoinHostPort(host, port)); !errors.Is(err, ErrPrivateOrigin) {
			t.Errorf("dial %s: err = %v, want ErrPrivateOrigin", host, err)
		}
	}
	allow := newOriginPolicy([]string{"localhost"}, nil)
	conn, err := allow.dialContext(dialer)(ctx, "tcp", net.JoinHostPort("localhost", port))
	if err != nil {
		t.Fatalf("allowlisted dial: %v", err)
	}
	conn.Close()
}

// Use time: a connector built from a stored loopback origin (for example one
// whose DNS changed after registration) cannot reach it.
func TestRegistryConnectorRefusesNonPublicPeerAtDialTime(t *testing.T) {
	var appCalls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appCalls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	registry := NewRegistryService(&fakeRegistryRepo{}, "https://sso.example.test/token")
	registry.Getenv = func(string) string { return "fixture-secret" }
	registry.AllowedPrivateHosts = nil
	integration := validIntegration()
	integration.Origin = server.URL
	connector, err := registry.ConnectorFor(context.Background(), integration)
	if err != nil {
		t.Fatal(err)
	}
	connector.token = func(context.Context) (string, error) { return "token", nil }
	connector.sleep = func(context.Context, time.Duration) error { return nil }
	attempts := 0
	connector.sleep = func(context.Context, time.Duration) error { attempts++; return nil }
	if _, err := connector.Describe(context.Background()); !errors.Is(err, ErrPrivateOrigin) {
		t.Fatalf("describe through a loopback peer: err = %v, want ErrPrivateOrigin", err)
	}
	if attempts != 0 {
		t.Fatalf("a refused peer was retried %d time(s)", attempts)
	}
	if appCalls.Load() != 0 {
		t.Fatal("the loopback peer received a request")
	}
}

// Connectors must dial the registered origin directly. Behind an HTTP(S) proxy
// the dial-time check would only see the proxy's address, so environment proxy
// settings must not apply to connector transports (default or injected).
func TestConnectorTransportIgnoresEnvironmentProxy(t *testing.T) {
	injected := http.DefaultTransport.(*http.Transport).Clone() // keeps ProxyFromEnvironment
	for name, registry := range map[string]*RegistryService{
		"default":  {},
		"injected": {HTTPClient: &http.Client{Transport: injected}},
	} {
		transport, ok := registry.connectorTransport().(*http.Transport)
		if !ok {
			t.Fatalf("%s: connector transport is %T", name, registry.connectorTransport())
		}
		if transport.Proxy != nil {
			t.Errorf("%s: connector transport uses a proxy function; the origin check would be bypassed", name)
		}
		if transport.DialContext == nil {
			t.Errorf("%s: connector transport lost its dial-time origin check", name)
		}
	}
	if injected.Proxy == nil {
		t.Fatal("the injected transport itself must not be modified")
	}
}
