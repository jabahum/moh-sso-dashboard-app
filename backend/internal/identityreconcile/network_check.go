package identityreconcile

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Network check stage statuses.
const (
	NetworkOK      = "ok"
	NetworkWarn    = "warn"
	NetworkFail    = "fail"
	NetworkSkipped = "skipped"
)

// Network check stage names, in the order they run.
const (
	StageOrigin  = "origin"
	StageDNS     = "dns"
	StagePolicy  = "address_policy"
	StageConnect = "connect"
	StageTLS     = "tls"
	StageHTTP    = "http"
)

// NetworkStage is one step of a network check.
type NetworkStage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// NetworkReport is the outcome of checking whether the dashboard can reach an
// application's identity-sync endpoint, stage by stage.
type NetworkReport struct {
	ApplicationID string         `json:"applicationId,omitempty"`
	Origin        string         `json:"origin"`
	Host          string         `json:"host,omitempty"`
	Addresses     []string       `json:"addresses,omitempty"`
	OK            bool           `json:"ok"`
	Stages        []NetworkStage `json:"stages"`
	CheckedAt     time.Time      `json:"checkedAt"`
}

const (
	networkStageTimeout = 5 * time.Second
	identitySyncPath    = "/api/v1/internal/identity-sync"
)

// CheckNetwork checks, from the dashboard's own network, each step needed to
// reach origin's identity-sync endpoint: DNS, the non-public address policy,
// a TCP connection, the TLS handshake and an unauthenticated HTTP request.
// It never sends a token or credential, never connects to an address the
// policy refuses, and reads only the response status. A failed stage skips
// the stages after it.
func (s *RegistryService) CheckNetwork(ctx context.Context, origin string) (report NetworkReport) {
	report = NetworkReport{Origin: origin, CheckedAt: time.Now().UTC()}
	stage := func(name, status, detail string) {
		report.Stages = append(report.Stages, NetworkStage{Name: name, Status: status, Detail: detail})
	}
	skipRest := func(from int) {
		for _, name := range []string{StageOrigin, StageDNS, StagePolicy, StageConnect, StageTLS, StageHTTP}[from:] {
			stage(name, NetworkSkipped, "")
		}
	}
	defer func() {
		report.OK = true
		for _, st := range report.Stages {
			if st.Status == NetworkFail {
				report.OK = false
			}
		}
	}()

	u, err := url.Parse(origin)
	if err != nil || !ValidOrigin(origin) {
		stage(StageOrigin, NetworkFail, "the origin must be an HTTPS scheme and host, for example https://app.health.go.ug")
		skipRest(1)
		return report
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "443"
	}
	report.Host = host
	stage(StageOrigin, NetworkOK, "")

	policy := s.originPolicy()
	var addrs []netip.Addr
	if addr, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{addr}
		stage(StageDNS, NetworkOK, "the origin is an IP address")
	} else {
		lookupCtx, cancel := context.WithTimeout(ctx, networkStageTimeout)
		addrs, err = policy.resolver(lookupCtx, host)
		cancel()
		if err != nil || len(addrs) == 0 {
			detail := host + " does not resolve from the portal server; add it to the DNS the portal server uses"
			var dnsErr *net.DNSError
			if errors.As(err, &dnsErr) && !dnsErr.IsNotFound {
				detail = host + " could not be resolved (" + dnsErr.Err + "); check the DNS servers the portal server uses"
			}
			stage(StageDNS, NetworkFail, detail)
			skipRest(2)
			return report
		}
		stage(StageDNS, NetworkOK, "")
	}
	for _, addr := range addrs {
		report.Addresses = append(report.Addresses, addr.String())
	}

	private := false
	for _, addr := range addrs {
		private = private || nonPublicAddr(addr)
	}
	switch {
	case !private:
		stage(StagePolicy, NetworkOK, "public address")
	case policy.allowedHost(host):
		stage(StagePolicy, NetworkOK, "private address, allowed by "+AllowedPrivateHostsEnv)
	default:
		stage(StagePolicy, NetworkFail, host+" resolves to a private address; list it in "+AllowedPrivateHostsEnv+" if the application is meant to be reached on a private network")
		skipRest(3)
		return report
	}

	dialer := &net.Dialer{Timeout: networkStageTimeout}
	var conn net.Conn
	var dialErr error
	for _, addr := range addrs {
		conn, dialErr = dialer.DialContext(ctx, "tcp", net.JoinHostPort(addr.String(), port))
		if dialErr == nil {
			break
		}
	}
	if conn == nil {
		detail := ReachabilityProblem(dialErr)
		if detail == "" {
			detail = "the portal server could not connect to the application"
		}
		stage(StageConnect, NetworkFail, detail+" (port "+port+")")
		skipRest(4)
		return report
	}
	defer conn.Close()
	stage(StageConnect, NetworkOK, "connected to "+conn.RemoteAddr().String())

	tlsConn := tls.Client(conn, &tls.Config{ServerName: host, RootCAs: s.networkCheckRoots(), MinVersion: tls.VersionTLS12})
	handshakeCtx, cancel := context.WithTimeout(ctx, networkStageTimeout)
	err = tlsConn.HandshakeContext(handshakeCtx)
	cancel()
	if err != nil {
		detail := ReachabilityProblem(err)
		if detail == "" {
			detail = "the TLS handshake failed"
		}
		stage(StageTLS, NetworkFail, detail)
		skipRest(5)
		return report
	}
	state := tlsConn.ConnectionState()
	detail := tls.VersionName(state.Version)
	if len(state.PeerCertificates) > 0 {
		detail += ", certificate valid until " + state.PeerCertificates[0].NotAfter.UTC().Format("2006-01-02")
	}
	stage(StageTLS, NetworkOK, detail)

	status, err := probeStatus(ctx, tlsConn, u.Host)
	if err != nil {
		stage(StageHTTP, NetworkFail, "the application did not answer an HTTP request in time")
		return report
	}
	stage(StageHTTP, httpStageStatus(status), httpStageDetail(status))
	return report
}

// networkCheckRoots uses the trust store of the configured HTTP client when it
// has one (tests trust httptest servers that way), otherwise the system roots.
func (s *RegistryService) networkCheckRoots() *x509.CertPool {
	if s.HTTPClient != nil {
		if transport, ok := s.HTTPClient.Transport.(*http.Transport); ok && transport.TLSClientConfig != nil {
			return transport.TLSClientConfig.RootCAs
		}
	}
	return nil
}

// probeStatus sends GET <identity-sync path> without credentials over conn and
// returns only the status code.
func probeStatus(ctx context.Context, conn *tls.Conn, hostPort string) (int, error) {
	deadline := time.Now().Add(networkStageTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+hostPort+identitySyncPath, nil)
	if err != nil {
		return 0, err
	}
	req.Host = hostPort
	req.Header.Set("User-Agent", "moh-sso-network-check")
	req.Close = true
	if err := req.Write(conn); err != nil {
		return 0, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func httpStageStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return NetworkOK
	case status >= 500:
		return NetworkFail
	default:
		return NetworkWarn
	}
}

func httpStageDetail(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Sprintf("HTTP %d without a token: the identity-sync endpoint answers, as expected", status)
	case status == http.StatusNotFound:
		return "HTTP 404: the application's identity-sync routes are not enabled (IDENTITY_SYNC_CLIENT_ID unset) or the origin points at the wrong service"
	case status >= 300 && status < 400:
		return fmt.Sprintf("HTTP %d redirect: identity sync does not follow redirects; register the final origin", status)
	case status >= 200 && status < 300:
		return fmt.Sprintf("HTTP %d without a token: the identity-sync endpoint must require a token", status)
	case status >= 500:
		return fmt.Sprintf("HTTP %d: the application or its proxy reported an error", status)
	}
	return fmt.Sprintf("HTTP %d: unexpected answer from the identity-sync endpoint", status)
}

// IntegrationLister is implemented by registries that can list every
// registered integration (PostgresRegistry).
type IntegrationLister interface {
	ListIntegrations(ctx context.Context) ([]Integration, error)
}

// ErrListUnsupported reports a registry repository that cannot list integrations.
var ErrListUnsupported = errors.New("identity sync registry cannot list integrations")

// CheckIntegrationsNetwork checks application (or, when empty, every
// registered integration) and labels each report with its application.
func (s *RegistryService) CheckIntegrationsNetwork(ctx context.Context, application string) ([]NetworkReport, error) {
	var integrations []Integration
	if application != "" {
		integration, err := s.Get(ctx, application)
		if err != nil {
			return nil, err
		}
		integrations = []Integration{integration}
	} else {
		lister, ok := s.Repository.(IntegrationLister)
		if !ok {
			return nil, ErrListUnsupported
		}
		var err error
		if integrations, err = lister.ListIntegrations(ctx); err != nil {
			return nil, err
		}
	}
	reports := make([]NetworkReport, 0, len(integrations))
	for _, integration := range integrations {
		report := s.CheckNetwork(ctx, integration.Origin)
		report.ApplicationID = strings.TrimSpace(integration.ApplicationClientID)
		reports = append(reports, report)
	}
	return reports, nil
}
