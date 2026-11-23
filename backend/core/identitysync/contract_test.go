package identitysync

import (
	"encoding/json"
	"strings"
	"testing"
)

func validDescriptor() Descriptor {
	return Descriptor{
		Contract:        ContractName,
		Version:         Version11,
		Application:     "clims",
		AccountIDFormat: AccountIDUUID,
		MaxPageSize:     100,
		Capabilities:    []Capability{CapabilityListAccounts, CapabilityLinkIdentity, CapabilityRevokeSessions},
	}
}

func TestDescriptorValidate(t *testing.T) {
	if err := validDescriptor().Validate("clims"); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
	if err := LegacyDescriptor("poesgo", AccountIDPositiveInteger).Validate("poesgo"); err != nil {
		t.Fatalf("legacy descriptor rejected: %v", err)
	}
	cases := map[string]func(*Descriptor){
		"contract":        func(d *Descriptor) { d.Contract = "other" },
		"version 2":       func(d *Descriptor) { d.Version = "2.0" },
		"version garbage": func(d *Descriptor) { d.Version = "1\\x" },
		"application":     func(d *Descriptor) { d.Application = "poesgo" },
		"format":          func(d *Descriptor) { d.AccountIDFormat = "email" },
		"page size 0":     func(d *Descriptor) { d.MaxPageSize = 0 },
		"page size 101":   func(d *Descriptor) { d.MaxPageSize = 101 },
		"unknown cap":     func(d *Descriptor) { d.Capabilities = append(d.Capabilities, "accounts.delete") },
		"duplicate cap":   func(d *Descriptor) { d.Capabilities = append(d.Capabilities, CapabilityLinkIdentity) },
		"missing link":    func(d *Descriptor) { d.Capabilities = []Capability{CapabilityListAccounts} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := validDescriptor()
			mutate(&d)
			if err := d.Validate("clims"); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestDescriptorSupports(t *testing.T) {
	if !validDescriptor().Supports(CapabilityRevokeSessions) {
		t.Fatal("1.1 descriptor should support session revocation")
	}
	if LegacyDescriptor("poesgo", AccountIDPositiveInteger).Supports(CapabilityRevokeSessions) {
		t.Fatal("1.0 applications cannot revoke sessions")
	}
}

func TestDescriptorJSONMatchesContract(t *testing.T) {
	raw := `{"contract":"moh-identity-sync","version":"1.1","application":"clims","accountIdFormat":"uuid",
		"maxPageSize":100,"capabilities":["accounts.list","identity.link","sessions.revoke"]}`
	var d Descriptor
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate("clims"); err != nil {
		t.Fatal(err)
	}
}

func TestValidAccountID(t *testing.T) {
	cases := []struct {
		format AccountIDFormat
		id     string
		want   bool
	}{
		{AccountIDUUID, "8b0c6f1e-2d3a-4b5c-9d8e-7f6a5b4c3d2e", true},
		{AccountIDUUID, "8B0C6F1E-2D3A-4B5C-9D8E-7F6A5B4C3D2E", false},
		{AccountIDUUID, "00000000-0000-0000-0000-000000000000", false},
		{AccountIDUUID, "42", false},
		{AccountIDPositiveInteger, "42", true},
		{AccountIDPositiveInteger, "0", false},
		{AccountIDPositiveInteger, "042", false},
		{AccountIDPositiveInteger, "-1", false},
		{AccountIDOpaque, "acct:alerts.42", true},
		{AccountIDOpaque, "", false},
		{AccountIDOpaque, "../etc", false},
		{AccountIDOpaque, "a/b", false},
		{AccountIDOpaque, strings.Repeat("a", 129), false},
		{"", "42", false},
	}
	for _, tc := range cases {
		if got := ValidAccountID(tc.format, tc.id); got != tc.want {
			t.Errorf("ValidAccountID(%q, %q) = %v, want %v", tc.format, tc.id, got, tc.want)
		}
	}
}

func TestValidCursorAndIdentity(t *testing.T) {
	for cursor, want := range map[string]bool{"": true, "42": true, "a~b.c_d-e:f": true, "a b": false, "a/b": false, strings.Repeat("c", 257): false} {
		if got := ValidCursor(cursor); got != want {
			t.Errorf("ValidCursor(%q) = %v, want %v", cursor, got, want)
		}
	}
	issuer := "https://sso.health.go.ug/realms/moh"
	for _, tc := range []struct {
		identity Identity
		want     bool
	}{
		{Identity{issuer, "5f2e"}, true},
		{Identity{"https://other/realms/moh", "5f2e"}, false},
		{Identity{issuer, ""}, false},
		{Identity{issuer, " 5f2e"}, false},
		{Identity{issuer, strings.Repeat("s", 256)}, false},
	} {
		if got := ValidIdentity(tc.identity, issuer); got != tc.want {
			t.Errorf("ValidIdentity(%+v) = %v, want %v", tc.identity, got, tc.want)
		}
	}
}

func TestAccountDecodesContractLink(t *testing.T) {
	var page struct {
		Accounts []Account
	}
	raw := `{"accounts":[{"id":"1","email":"a@x","link":null},{"id":"2","email":"b@x","link":{"issuer":"https://i","subject":"s"}}]}`
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	if page.Accounts[0].Link != (Identity{}) || page.Accounts[1].Link != (Identity{Issuer: "https://i", Subject: "s"}) {
		t.Fatalf("links decoded as %+v", page.Accounts)
	}
}
