package identityreconcile

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestReachabilityProblem(t *testing.T) {
	dial := func(err error) error { return &net.OpError{Op: "dial", Net: "tcp", Err: err} }
	for _, tc := range []struct {
		name   string
		err    error
		phrase string
	}{
		{"nil", nil, ""},
		{"unrelated", errors.New("HTTP 403"), ""},
		{"unresolvable sentinel", fmt.Errorf("%w: alerts.example.test", ErrOriginUnresolvable), "does not resolve"},
		{"dns error from the dialer", fmt.Errorf("giving up: %w", &net.DNSError{Err: "no such host", Name: "alerts.example.test"}), "does not resolve"},
		{"private address", fmt.Errorf("%w: alerts connects to 10.0.0.5", ErrPrivateOrigin), AllowedPrivateHostsEnv},
		{"untrusted certificate", fmt.Errorf("get: %w", x509.UnknownAuthorityError{}), "certificate"},
		{"refused", dial(os.NewSyscallError("connect", syscall.ECONNREFUSED)), "refused"},
		{"deadline", fmt.Errorf("get: %w", context.DeadlineExceeded), "timed out"},
		{"other dial error", dial(errors.New("network is unreachable")), "could not connect"},
	} {
		got := ReachabilityProblem(tc.err)
		if tc.phrase == "" && got != "" || !strings.Contains(got, tc.phrase) {
			t.Errorf("%s: %q, want it to mention %q", tc.name, got, tc.phrase)
		}
	}
}

func TestCheckHostMarksResolutionFailures(t *testing.T) {
	failing := newOriginPolicy(nil, func(context.Context, string) ([]netip.Addr, error) {
		return nil, &net.DNSError{Err: "no such host", Name: "alerts.example.test", IsNotFound: true}
	})
	if err := failing.checkHost(context.Background(), "alerts.example.test"); !errors.Is(err, ErrOriginUnresolvable) {
		t.Fatalf("lookup failure: %v", err)
	}
	empty := newOriginPolicy(nil, func(context.Context, string) ([]netip.Addr, error) { return nil, nil })
	if err := empty.checkHost(context.Background(), "alerts.example.test"); !errors.Is(err, ErrOriginUnresolvable) {
		t.Fatalf("no addresses: %v", err)
	}
}
