package identitysync

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ContractName identifies the identity-sync contract in an application's descriptor.
const ContractName = "moh-identity-sync"

// Supported contract versions. Minor versions are backwards compatible.
const (
	Version10 = "1.0"
	Version11 = "1.1"
)

// MaxPageSize is the largest account page any party may request or return.
const MaxPageSize = 100

// Capability names advertised in a 1.1 descriptor.
type Capability string

const (
	CapabilityListAccounts   Capability = "accounts.list"
	CapabilityLinkIdentity   Capability = "identity.link"
	CapabilityRevokeSessions Capability = "sessions.revoke"
)

// AccountIDFormat is the application's declared local account ID format.
type AccountIDFormat string

const (
	AccountIDUUID            AccountIDFormat = "uuid"
	AccountIDPositiveInteger AccountIDFormat = "positive-integer"
	AccountIDOpaque          AccountIDFormat = "opaque"
)

var (
	accountIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	cursorPattern    = regexp.MustCompile(`^[A-Za-z0-9._:~-]{1,256}$`)
	uuidPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	versionPattern   = regexp.MustCompile(`^1\.[0-9]+$`)
)

// Descriptor is the response of GET /api/v1/internal/identity-sync (contract 1.1).
type Descriptor struct {
	Contract        string          `json:"contract"`
	Version         string          `json:"version"`
	Application     string          `json:"application"`
	AccountIDFormat AccountIDFormat `json:"accountIdFormat"`
	MaxPageSize     int             `json:"maxPageSize"`
	Capabilities    []Capability    `json:"capabilities"`
}

// LegacyDescriptor describes a 1.0 application, which has no descriptor endpoint.
// The account ID format comes from the dashboard's registry.
func LegacyDescriptor(application string, format AccountIDFormat) Descriptor {
	return Descriptor{
		Contract:        ContractName,
		Version:         Version10,
		Application:     application,
		AccountIDFormat: format,
		MaxPageSize:     MaxPageSize,
		Capabilities:    []Capability{CapabilityListAccounts, CapabilityLinkIdentity},
	}
}

// Validate checks a descriptor against the application the dashboard registered.
func (d Descriptor) Validate(application string) error {
	switch {
	case d.Contract != ContractName:
		return fmt.Errorf("unexpected identity-sync contract %q", d.Contract)
	case !versionPattern.MatchString(d.Version):
		return fmt.Errorf("unsupported identity-sync contract version %q", d.Version)
	case application == "" || d.Application != application:
		return fmt.Errorf("descriptor names application %q, registry expects %q", d.Application, application)
	case d.AccountIDFormat != AccountIDUUID && d.AccountIDFormat != AccountIDPositiveInteger && d.AccountIDFormat != AccountIDOpaque:
		return fmt.Errorf("unknown account ID format %q", d.AccountIDFormat)
	case d.MaxPageSize < 1 || d.MaxPageSize > MaxPageSize:
		return fmt.Errorf("descriptor page size %d outside 1-%d", d.MaxPageSize, MaxPageSize)
	}
	seen := map[Capability]bool{}
	for _, capability := range d.Capabilities {
		switch capability {
		case CapabilityListAccounts, CapabilityLinkIdentity, CapabilityRevokeSessions:
		default:
			return fmt.Errorf("unknown capability %q", capability)
		}
		if seen[capability] {
			return fmt.Errorf("duplicate capability %q", capability)
		}
		seen[capability] = true
	}
	if !seen[CapabilityListAccounts] || !seen[CapabilityLinkIdentity] {
		return fmt.Errorf("descriptor must offer %s and %s", CapabilityListAccounts, CapabilityLinkIdentity)
	}
	return nil
}

// Supports reports whether the descriptor advertises a capability.
func (d Descriptor) Supports(capability Capability) bool {
	for _, c := range d.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// ValidAccountID checks an account ID against the contract and the declared format.
func ValidAccountID(format AccountIDFormat, id string) bool {
	if !accountIDPattern.MatchString(id) {
		return false
	}
	switch format {
	case AccountIDUUID:
		return uuidPattern.MatchString(id) && id != "00000000-0000-0000-0000-000000000000"
	case AccountIDPositiveInteger:
		n, err := strconv.ParseInt(id, 10, 64)
		return err == nil && n > 0 && strconv.FormatInt(n, 10) == id
	case AccountIDOpaque:
		return true
	default:
		return false
	}
}

// ValidCursor checks an opaque continuation cursor. The empty string means
// "first page" in a request and "last page" in a response.
func ValidCursor(cursor string) bool {
	return cursor == "" || cursorPattern.MatchString(cursor)
}

// ValidIdentity checks an identity for the given expected issuer.
func ValidIdentity(identity Identity, issuer string) bool {
	subject := strings.TrimSpace(identity.Subject)
	return issuer != "" && identity.Issuer == issuer && subject != "" && subject == identity.Subject && len(subject) <= 255
}
