package cmd

import (
	"fmt"
	"os"

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
			// Check if we should only download the zip
			downloadOnly, _ := cmd.Flags().GetBool("download-only")

			// fetch integration definition
			def, err := fetcher.FetchIntegration(integration)
			if err != nil {
				return err
			}

			// create a temporary directory for staging
			tmpDir, err := os.MkdirTemp("", "lcf-"+integration+"-*")
			if err != nil {
				return fmt.Errorf("creating temp dir: %w", err)
			}

			// Only clean up if we're not in download-only mode
			if !downloadOnly {
				defer os.RemoveAll(tmpDir)
			}

			// download assets to the temp directory
			zipPath, err := fetcher.DownloadPackage(def, tmpDir)
			if err != nil {
				return err
			}

			// If download-only flag is set, just print the path and exit
			if downloadOnly {
				fmt.Printf("\nDownload complete. Zip file saved to: %s\n", zipPath)
				fmt.Printf("Temporary directory: %s\n", tmpDir)
				return nil
			}

			// Extract the zip file
			if err := fetcher.ExtractZip(zipPath, tmpDir); err != nil {
				return err
			}

			// generate stubs
			switch lang {
			case "python":
				return python.GenerateStubs(def, tmpDir, outDir)
			default:
				return fmt.Errorf("unsupported language: %s", lang)
			}
		},
	}
	down.Flags().StringVarP(&integration, "integration", "", "", "Integration name (e.g. AWS)")
	down.Flags().StringVarP(&lang, "lang", "", "python", "Target language (python)")
	down.Flags().StringVarP(&outDir, "out", "", ".", "Output directory")
	down.Flags().BoolP("download-only", "", false, "Only download the zip file and print its path")
	down.MarkFlagRequired("integration")
	// down.MarkFlagRequired("lang")
	// down.MarkFlagRequired("out")
	// down.MarkFlagRequired("download-only")
	rootCmd.AddCommand(down)
}
