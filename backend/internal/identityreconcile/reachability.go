package identityreconcile

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"syscall"
)

// ErrOriginUnresolvable reports an origin whose host name the dashboard's DNS
// cannot resolve, so nothing can be sent to it.
var ErrOriginUnresolvable = errors.New("identity sync origin host does not resolve")

// ReachabilityProblem explains, for an administrator, why the dashboard could
// not reach an application's identity-sync endpoint. It returns "" when err
// is not a network or address-policy fault. The text never includes upstream
// response bodies, addresses beyond the origin's own, or credentials.
func ReachabilityProblem(err error) string {
	if err == nil {
		return ""
	}
	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	var recordErr tls.RecordHeaderError
	var netErr net.Error
	var opErr *net.OpError
	switch {
	case errors.Is(err, ErrOriginUnresolvable) || errors.As(err, &dnsErr):
		return "the host name does not resolve from the portal server; add it to the DNS the portal server uses"
	case errors.Is(err, ErrPrivateOrigin):
		return "the host resolves to a private address; list it in " + AllowedPrivateHostsEnv + " if the application is meant to be reached on a private network"
	case errors.As(err, &certErr), errors.As(err, &unknownAuthority), errors.As(err, &hostnameErr), errors.As(err, &invalidCert):
		return "the application's TLS certificate is not trusted by the portal server, or does not match the host name"
	case errors.As(err, &recordErr):
		return "the application did not answer with TLS; check that the origin uses HTTPS on the right port"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "the connection was refused; check that the application is running and listens for HTTPS"
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()):
		return "the connection timed out; check firewalls and routing between the portal server and the application"
	case errors.As(err, &opErr):
		return "the portal server could not connect to the application"
	}
	return ""
}
