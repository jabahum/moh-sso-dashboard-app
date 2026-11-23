// Command consumer is the release check's independent consumer of
// github.com/mohuganda/moh-sso-dashboard/backend/core. It builds outside the
// repository against the module (a local replacement before tagging, the
// published version afterwards), serves the contract with the starter adapter
// and the in-memory store, and runs the conformance suite against it over
// real HTTPS. infra/scripts/test-core-release.py runs it; it is not part of
// the module's packages (testdata is ignored by go build ./...).
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/adapter/memory"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/conformance"
)

const (
	issuer = "https://sso.consumer.example/realms/moh"
	app    = "consumer-app"
	sync   = "consumer-app-identity-sync"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS: independent consumer served contract " + identitysync.Version11 + " with the starter adapter and passed the conformance suite")
}

func claims(mutate func(*adapter.Claims)) adapter.Claims {
	c := adapter.Claims{
		Issuer: issuer, Audience: []string{app}, AuthorizedParty: sync, Type: "Bearer",
		ExpiresAt: time.Now().Add(time.Hour), ResourceAccess: map[string][]string{app: {"identity_sync"}},
	}
	if mutate != nil {
		mutate(&c)
	}
	return c
}

func run() error {
	store := memory.New(issuer, identitysync.AccountIDPositiveInteger)
	for i := 1; i <= 12; i++ {
		store.Add(fmt.Sprint(i), fmt.Sprintf("person%d@consumer.example", i), true)
	}
	store.Add("99", "inactive@consumer.example", false)

	tokens := map[string]adapter.Claims{
		"sync":         claims(nil),
		"wrong-aud":    claims(func(c *adapter.Claims) { c.Audience = []string{"account"} }),
		"user":         claims(func(c *adapter.Claims) { c.AuthorizedParty = app }),
		"other-client": claims(func(c *adapter.Claims) { c.AuthorizedParty = "another-identity-sync" }),
		"no-role":      claims(func(c *adapter.Claims) { c.ResourceAccess = map[string][]string{app: {}} }),
	}
	handler, err := adapter.New(adapter.Config{
		Application: app, Issuer: issuer, SyncClientID: sync,
		AccountIDFormat: identitysync.AccountIDPositiveInteger,
		Store:           store,
		Verifier: adapter.TokenVerifierFunc(func(_ context.Context, raw string) (adapter.Claims, error) {
			if c, ok := tokens[raw]; ok {
				return c, nil
			}
			return adapter.Claims{}, errors.New("signature does not verify")
		}),
		ProbeEnabled: true, Probe: store,
	})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle(adapter.BasePath, handler)
	mux.Handle(adapter.BasePath+"/", handler)
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	report, err := conformance.Run(context.Background(), conformance.Config{
		BaseURL: server.URL + adapter.BasePath,
		Client:  server.Client(),
		Probe:   true,
		Tokens: conformance.Tokens{
			Sync:             conformance.StaticToken("sync"),
			ExpiredOrGarbage: conformance.StaticToken("expired"),
			WrongAudience:    conformance.StaticToken("wrong-aud"),
			User:             conformance.StaticToken("user"),
			OtherClient:      conformance.StaticToken("other-client"),
			MissingRole:      conformance.StaticToken("no-role"),
		},
		Fixtures: conformance.Fixtures{
			Application: app, Issuer: issuer,
			LinkableAccounts: []string{"4", "9"}, IneligibleAccount: "99",
		},
	})
	if err != nil {
		return err
	}
	pass, fail, skip := report.Counts()
	if fail != 0 || skip != 0 || pass < 40 {
		for _, r := range report.Results {
			if r.Status != conformance.Pass {
				fmt.Fprintf(os.Stderr, "%s %s %s\n", r.Status, r.ID, r.Detail)
			}
		}
		return fmt.Errorf("conformance pass=%d fail=%d skip=%d", pass, fail, skip)
	}
	return nil
}
