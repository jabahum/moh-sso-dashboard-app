package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moh-sso-dashboard/docs"
	"github.com/moh-sso-dashboard/internal/apidocgen"
	"github.com/spf13/cobra"
)

var openAPIOutput string

var openAPICmd = &cobra.Command{
	Use:   "openapi",
	Short: "OpenAPI utilities",
}

var openAPIValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate complete route coverage, schemas, references and generation freshness",
	RunE: func(cmd *cobra.Command, args []string) error {
		generated, err := apidocgen.Generate(".")
		if err != nil {
			return err
		}
		actual, err := os.ReadFile("docs/openapi.yaml")
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, generated) {
			return fmt.Errorf("OpenAPI document is stale; run go run ./cmd/cli openapi generate")
		}
		contract, err := apidocgen.PlatformContract(generated)
		if err != nil {
			return err
		}
		published, err := os.ReadFile(apidocgen.PlatformContractFile)
		if err != nil || !bytes.Equal(published, contract) {
			return fmt.Errorf("platform API contract %s is stale; run go run ./cmd/cli openapi generate", apidocgen.PlatformContractFile)
		}
		fmt.Printf("OpenAPI valid and current: %d registered endpoints; platform contract current\n", len(apidocgen.Routes()))
		return nil
	},
}

var platformCompatBase, platformCompatCurrent string

var openAPIPlatformCompatCmd = &cobra.Command{
	Use:   "platform-compat",
	Short: "Fail when the platform API contract breaks clients of an earlier version",
	Long: "Compares the platform API contract with an earlier copy (for example the default branch's)\n" +
		"and fails on removed endpoints or success responses, removed or no longer guaranteed response\n" +
		"fields, new required inputs and changed types, unless the major version was raised.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		base, err := os.ReadFile(platformCompatBase)
		if err != nil {
			return err
		}
		current, err := os.ReadFile(platformCompatCurrent)
		if err != nil {
			return err
		}
		changes, err := apidocgen.BreakingChanges(base, current)
		if err != nil {
			return err
		}
		if len(changes) > 0 {
			for _, change := range changes {
				fmt.Fprintln(cmd.ErrOrStderr(), "breaking: "+change)
			}
			cmd.SilenceUsage = true
			return fmt.Errorf("%d breaking platform API changes; make them additive, or release a new major version under a new path", len(changes))
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Platform API contract is compatible with the base version")
		return nil
	},
}

var openAPIExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the embedded OpenAPI document",
	RunE: func(cmd *cobra.Command, args []string) error {
		data := docs.YAML()

		if openAPIOutput == "" || openAPIOutput == "-" {
			fmt.Print(string(data))
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(openAPIOutput), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(openAPIOutput, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("OpenAPI document exported to %s\n", openAPIOutput)
		return nil
	},
}

func init() {
	openAPICmd.AddCommand(&cobra.Command{Use: "generate", Short: "Generate complete OpenAPI documentation from registered routes and typed handlers", RunE: func(cmd *cobra.Command, args []string) error {
		data, err := apidocgen.Generate(".")
		if err != nil {
			return err
		}
		if err := os.WriteFile("docs/openapi.yaml", data, 0o644); err != nil {
			return err
		}
		contract, err := apidocgen.PlatformContract(data)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(apidocgen.PlatformContractFile), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(apidocgen.PlatformContractFile, contract, 0o644); err != nil {
			return err
		}
		fmt.Printf("Generated docs/openapi.yaml for %d endpoints and %s\n", len(apidocgen.Routes()), apidocgen.PlatformContractFile)
		return nil
	}})
	openAPICmd.AddCommand(openAPIPlatformCompatCmd)
	openAPIPlatformCompatCmd.Flags().StringVar(&platformCompatBase, "base", "", "Earlier platform contract to compare against (required)")
	openAPIPlatformCompatCmd.Flags().StringVar(&platformCompatCurrent, "current", apidocgen.PlatformContractFile, "Platform contract to check")
	_ = openAPIPlatformCompatCmd.MarkFlagRequired("base")
	openAPICmd.AddCommand(openAPIValidateCmd)
	openAPICmd.AddCommand(openAPIExportCmd)
	openAPIExportCmd.Flags().StringVarP(&openAPIOutput, "output", "o", "-", "Output path, or '-' for stdout")
}
