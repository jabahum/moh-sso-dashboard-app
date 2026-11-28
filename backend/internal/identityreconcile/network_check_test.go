package identityreconcile

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

// networkFixture serves the identity-sync path with status on a TLS test
// server whose certificate is valid for example.com, and resolves example.com
// to it.
func networkFixture(t *testing.T, status int) (*RegistryService, string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "" || r.URL.Path != identitySyncPath {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	s := &RegistryService{
		HTTPClient:          server.Client(),
		AllowedPrivateHosts: []string{"example.com"},
		Resolver: func(_ context.Context, host string) ([]netip.Addr, error) {
			if host == "example.com" {
				return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
			}
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		},
	}
	return s, "https://example.com:" + port, &hits
}

func stageStatuses(r NetworkReport) string {
	var parts []string
	for _, st := range r.Stages {
		parts = append(parts, st.Name+"="+st.Status)
	}
	return strings.Join(parts, " ")
}

func stageDetail(r NetworkReport, name string) string {
	for _, st := range r.Stages {
		if st.Name == name {
			return st.Detail
		}
	}
	return ""
}

func TestCheckNetworkReachableEndpoint(t *testing.T) {
	s, origin, hits := networkFixture(t, http.StatusUnauthorized)
	report := s.CheckNetwork(context.Background(), origin)
	want := "origin=ok dns=ok address_policy=ok connect=ok tls=ok http=ok"
	if got := stageStatuses(report); got != want || !report.OK {
		t.Fatalf("stages = %s (ok=%v), want %s", got, report.OK, want)
	}
	if !strings.Contains(stageDetail(report, StagePolicy), AllowedPrivateHostsEnv) || report.Host != "example.com" || len(report.Addresses) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if hits.Load() != 1 {
		t.Fatalf("requests = %d, want 1", hits.Load())
	}
}

func TestCheckNetworkHTTPAnswers(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   string
		ok     bool
		phrase string
	}{
		{http.StatusForbidden, NetworkOK, true, "as expected"},
		{http.StatusNotFound, NetworkWarn, true, "IDENTITY_SYNC_CLIENT_ID"},
		{http.StatusOK, NetworkWarn, true, "must require a token"},
		{http.StatusFound, NetworkWarn, true, "redirect"},
		{http.StatusBadGateway, NetworkFail, false, "502"},
	} {
		s, origin, _ := networkFixture(t, tc.status)
		report := s.CheckNetwork(context.Background(), origin)
		last := report.Stages[len(report.Stages)-1]
		if last.Name != StageHTTP || last.Status != tc.want || report.OK != tc.ok || !strings.Contains(last.Detail, tc.phrase) {
			t.Errorf("HTTP %d: %+v ok=%v", tc.status, last, report.OK)
		}
	}
}

func TestCheckNetworkFailuresStopEarly(t *testing.T) {
	t.Run("invalid origin", func(t *testing.T) {
		s, _, _ := networkFixture(t, 401)
		report := s.CheckNetwork(context.Background(), "http://example.com")
		if got := stageStatuses(report); got != "origin=fail dns=skipped address_policy=skipped connect=skipped tls=skipped http=skipped" || report.OK {
			t.Fatal(got)
		}
	})
	t.Run("unresolvable host", func(t *testing.T) {
		s, _, hits := networkFixture(t, 401)
		report := s.CheckNetwork(context.Background(), "https://clims.example.org")
		if got := stageStatuses(report); got != "origin=ok dns=fail address_policy=skipped connect=skipped tls=skipped http=skipped" || report.OK {
			t.Fatal(got)
		}
		if !strings.Contains(stageDetail(report, StageDNS), "clims.example.org does not resolve from the portal server") || hits.Load() != 0 {
			t.Fatalf("%q hits=%d", stageDetail(report, StageDNS), hits.Load())
		}
	})
	t.Run("private address not allowed is never contacted", func(t *testing.T) {
		s, origin, hits := networkFixture(t, 401)
		s.AllowedPrivateHosts = nil
		report := s.CheckNetwork(context.Background(), origin)
		if got := stageStatuses(report); got != "origin=ok dns=ok address_policy=fail connect=skipped tls=skipped http=skipped" || report.OK {
			t.Fatal(got)
		}
		if hits.Load() != 0 {
			t.Fatalf("contacted a refused address %d times", hits.Load())
		}
	})
	t.Run("connection refused", func(t *testing.T) {
		s, _, _ := networkFixture(t, 401)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_, port, _ := net.SplitHostPort(listener.Addr().String())
		listener.Close()
		report := s.CheckNetwork(context.Background(), "https://example.com:"+port)
		if got := stageStatuses(report); got != "origin=ok dns=ok address_policy=ok connect=fail tls=skipped http=skipped" || !strings.Contains(stageDetail(report, StageConnect), "refused") {
			t.Fatalf("%s: %q", got, stageDetail(report, StageConnect))
		}
	})
	t.Run("untrusted certificate", func(t *testing.T) {
		s, origin, hits := networkFixture(t, 401)
		s.HTTPClient = nil
		report := s.CheckNetwork(context.Background(), origin)
		if got := stageStatuses(report); got != "origin=ok dns=ok address_policy=ok connect=ok tls=fail http=skipped" || !strings.Contains(stageDetail(report, StageTLS), "certificate") {
			t.Fatalf("%s: %q", got, stageDetail(report, StageTLS))
		}
		if hits.Load() != 0 {
			t.Fatalf("sent a request over an untrusted connection")
		}
	})
}

type listingRegistry struct {
	fakeRegistryRepo
	items []Integration
}

func (l *listingRegistry) ListIntegrations(context.Context) ([]Integration, error) {
	return l.items, nil
}

func TestCheckIntegrationsNetwork(t *testing.T) {
	s, origin, _ := networkFixture(t, 401)
	s.Repository = &listingRegistry{items: []Integration{
		{ApplicationClientID: "alerts", Origin: origin},
		{ApplicationClientID: "clims", Origin: "https://clims.example.org"},
	}}
	reports, err := s.CheckIntegrationsNetwork(context.Background(), "")
	if err != nil || len(reports) != 2 || reports[0].ApplicationID != "alerts" || !reports[0].OK || reports[1].ApplicationID != "clims" || reports[1].OK {
		t.Fatalf("reports = %+v, %v", reports, err)
	}
}
