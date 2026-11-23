package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync/conformance"
)

const identitySyncBasePath = "/api/v1/internal/identity-sync"

type conformanceTokenEnv struct {
	sync, expired, wrongAudience, user, otherClient, missingRole string
}

// newIdentitySyncConformanceCommand builds `identity-sync conformance`. The
// parent `identity-sync` command (Track A) attaches it.
func newIdentitySyncConformanceCommand() *cobra.Command {
	var (
		app, origin, baseURL, fixturesPath, caFile string
		probe, asJSON, allowHTTP                   bool
		timeout                                    time.Duration
		envNames                                   conformanceTokenEnv
	)
	cmd := &cobra.Command{
		Use:   "conformance",
		Short: "Run the identity-sync contract conformance suite against an application in a test environment",
		Long: `Runs every identity-sync contract 1.1 check against an application's
endpoints and prints one result per check. The suite creates links for the
fixture accounts: run it only against a disposable environment with synthetic
accounts. Tokens are read from environment variables; an optional token that
is not set makes its check report "skip", never "pass".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			base, err := conformanceBaseURL(baseURL, origin)
			if err != nil {
				return err
			}
			fixtures, err := readConformanceFixtures(fixturesPath)
			if err != nil {
				return err
			}
			if app != "" {
				fixtures.Application = app
			}
			tokens, err := conformanceTokens(envNames, os.Getenv)
			if err != nil {
				return err
			}
			client, err := conformanceHTTPClient(caFile)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			report, err := conformance.Run(ctx, conformance.Config{
				BaseURL: base, Client: client, Tokens: tokens, Fixtures: fixtures,
				Probe: probe, AllowHTTP: allowHTTP,
			})
			if err != nil {
				return err
			}
			if err := writeConformanceReport(cmd, report, asJSON); err != nil {
				return err
			}
			if report.Failed() {
				cmd.SilenceUsage = true
				_, fail, _ := report.Counts()
				return fmt.Errorf("identity-sync conformance: %d check(s) failed", fail)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&app, "app", "", "Expected application client ID (overrides the fixture file)")
	f.StringVar(&origin, "origin", "", "Application origin, for example https://clims.test.example")
	f.StringVar(&baseURL, "base-url", "", "Full base URL (default <origin>"+identitySyncBasePath+")")
	f.StringVar(&fixturesPath, "fixtures", "", "JSON fixture file: application, issuer, linkableAccounts[2], ineligibleAccount, optional subjects[3]")
	f.BoolVar(&probe, "probe", false, "Use the test-only session probe (the app must run with IDENTITY_SYNC_TEST_PROBE=1)")
	f.StringVar(&caFile, "ca-file", "", "PEM CA bundle to trust in addition to the system roots")
	f.BoolVar(&asJSON, "json", false, "Print the report as JSON")
	f.BoolVar(&allowHTTP, "allow-http", false, "Accept an http:// base URL for local runs (HTTPS is then reported as not verified)")
	f.DurationVar(&timeout, "timeout", 5*time.Minute, "Overall time limit")
	f.StringVar(&envNames.sync, "sync-token-env", "IDENTITY_SYNC_TOKEN", "Env var holding a valid identity-sync client token (required)")
	f.StringVar(&envNames.expired, "expired-token-env", "IDENTITY_SYNC_EXPIRED_TOKEN", "Env var holding an expired or unverifiable token")
	f.StringVar(&envNames.wrongAudience, "wrong-audience-token-env", "IDENTITY_SYNC_WRONG_AUDIENCE_TOKEN", "Env var holding a token without the application in aud")
	f.StringVar(&envNames.user, "user-token-env", "IDENTITY_SYNC_USER_TOKEN", "Env var holding a user access token for the application")
	f.StringVar(&envNames.otherClient, "other-client-token-env", "IDENTITY_SYNC_OTHER_CLIENT_TOKEN", "Env var holding another service client's token")
	f.StringVar(&envNames.missingRole, "missing-role-token-env", "IDENTITY_SYNC_MISSING_ROLE_TOKEN", "Env var holding a sync-client token without the identity_sync role")
	_ = cmd.MarkFlagRequired("fixtures")
	return cmd
}

func conformanceBaseURL(baseURL, origin string) (string, error) {
	switch {
	case baseURL != "" && origin != "":
		return "", errors.New("use either --base-url or --origin, not both")
	case baseURL != "":
		return baseURL, nil
	case origin != "":
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.User != nil {
			return "", errors.New("--origin must be scheme and host only")
		}
		return u.Scheme + "://" + u.Host + identitySyncBasePath, nil
	default:
		return "", errors.New("--base-url or --origin is required")
	}
}

func readConformanceFixtures(path string) (conformance.Fixtures, error) {
	var fixtures conformance.Fixtures
	raw, err := os.ReadFile(path)
	if err != nil {
		return fixtures, fmt.Errorf("read fixtures: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fixtures); err != nil {
		return fixtures, fmt.Errorf("parse fixtures %s: %w", path, err)
	}
	return fixtures, nil
}

func conformanceTokens(names conformanceTokenEnv, getenv func(string) string) (conformance.Tokens, error) {
	optional := func(name string) conformance.TokenSource {
		if name == "" {
			return nil
		}
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return conformance.StaticToken(v)
		}
		return nil
	}
	tokens := conformance.Tokens{
		Sync:             optional(names.sync),
		ExpiredOrGarbage: optional(names.expired),
		WrongAudience:    optional(names.wrongAudience),
		User:             optional(names.user),
		OtherClient:      optional(names.otherClient),
		MissingRole:      optional(names.missingRole),
	}
	if tokens.Sync == nil {
		return tokens, fmt.Errorf("set %s to a valid identity-sync client access token", names.sync)
	}
	return tokens, nil
}

func conformanceHTTPClient(caFile string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("CA file contains no PEM certificates")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: transport}, nil
}

func writeConformanceReport(cmd *cobra.Command, report conformance.Report, asJSON bool) error {
	out := cmd.OutOrStdout()
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range report.Results {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", strings.ToUpper(string(r.Status)), r.ID, r.Detail)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	pass, fail, skip := report.Counts()
	_, err := fmt.Fprintf(out, "\n%d passed, %d failed, %d skipped against %s\n", pass, fail, skip, report.BaseURL)
	return err
}
