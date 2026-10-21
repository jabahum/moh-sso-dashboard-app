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
		fmt.Printf("OpenAPI valid and current: %d registered endpoints\n", len(apidocgen.Routes()))
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
		fmt.Printf("Generated docs/openapi.yaml for %d endpoints\n", len(apidocgen.Routes()))
		return nil
	}})
	openAPICmd.AddCommand(openAPIValidateCmd)
	openAPICmd.AddCommand(openAPIExportCmd)
	openAPIExportCmd.Flags().StringVarP(&openAPIOutput, "output", "o", "-", "Output path, or '-' for stdout")
}
