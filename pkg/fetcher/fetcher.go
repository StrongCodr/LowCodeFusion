// File: pkg/fetcher/fetcher.go

package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// IntegrationDef holds metadata for an integration
type IntegrationDef struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	SchemaURL   string `json:"schemaUrl"`
}

// FetchIntegration retrieves the integration definition JSON
func FetchIntegration(name string) (*IntegrationDef, error) {
	url := fmt.Sprintf("https://automation-library.ibm.com/integrations/%s/latest", name)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var def IntegrationDef
	if err := json.NewDecoder(resp.Body).Decode(&def); err != nil {
		return nil, err
	}
	return &def, nil
}

// DownloadPackage downloads and extracts the integration package
func DownloadPackage(def *IntegrationDef, targetDir string) error {
	rsp, err := http.Get(def.DownloadURL)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	// create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// assume a zip archive
	zipPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%s.zip", def.Name, def.Version))
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, rsp.Body); err != nil {
		return err
	}
	// unzip the file
	if err := Unzip(zipPath, targetDir); err != nil {
		return err
	}
	return nil
}

// Unzip is a helper to extract zip archives
func Unzip(src, dest string) error {
	// implementation omitted for brevity; use archive/zip
	return nil
}
