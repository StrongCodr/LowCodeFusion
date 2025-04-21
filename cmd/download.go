package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/strongcodr/lowcodefusion/pkg/fetcher"
	"github.com/strongcodr/lowcodefusion/pkg/generator/python"
)

var (
	integration string
	lang        string
	outDir      string
)

func init() {
	down := &cobra.Command{
		Use:   "download",
		Short: "Download and scaffold a Pliant integration SDK",
		RunE: func(cmd *cobra.Command, args []string) error {
			// fetch integration definition
			def, err := fetcher.FetchIntegration(integration)
			if err != nil {
				return err
			}
			// download assets
			target := filepath.Join(outDir, integration)
			if err := fetcher.DownloadPackage(def, target); err != nil {
				return err
			}
			// generate stubs
			switch lang {
			case "python":
				return python.GenerateStubs(def, target)
			default:
				return fmt.Errorf("unsupported language: %s", lang)
			}
		},
	}
	down.Flags().StringVarP(&integration, "integration", "i", "", "Integration name (e.g. AWS)")
	down.Flags().StringVarP(&lang, "lang", "l", "python", "Target language (python)")
	down.Flags().StringVarP(&outDir, "out", "o", ".", "Output directory")
	down.MarkFlagRequired("integration")
	rootCmd.AddCommand(down)
}
