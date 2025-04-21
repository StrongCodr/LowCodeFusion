// File: pkg/fetcher/fetcher.go

package fetcher

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// IntegrationDef holds metadata for an integration
type IntegrationDef struct {
	Name        string // e.g. "AWS"
	Version     string // e.g. "1.1.118"
	DownloadURL string // full URL to the .zip package
}

// ExtractZip extracts the zip file to the target directory
func ExtractZip(zipPath string, targetDir string) error {
	// unzip the file
	if err := Unzip(zipPath, targetDir); err != nil {
		return err
	}
	return nil
}

// apiResponse wraps the JSON returned by the API
type apiResponse struct {
	Result struct {
		Name          string `json:"Name"`
		LatestVersion string `json:"LatestVersion"` // e.g. "AWS_1.1.118.ssi.zip"
	} `json:"result"`
}

// FetchIntegration retrieves the integration definition via the Pliant API
func FetchIntegration(name string) (*IntegrationDef, error) {
	// Call the JSON‑returning endpoint
	apiURL := fmt.Sprintf(
		"https://automation-library.ibm.com/api/getIntegrationDetails?Name=%s",
		name,
	)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	// --- DIAGNOSTIC LOGGING START ---
	/*
		fmt.Println("Fetching URL:", apiURL)
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		fmt.Println("Response body:", string(bodyBytes))
		// Reset the body for JSON decoding:
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		// --- DIAGNOSTIC LOGGING END ---
	*/

	// Ensure we got JSON, not HTML
	ct := resp.Header.Get("Content-Type")
	if ct == "" || ct[:16] != "application/json" {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"expected JSON response, got %q: %s",
			ct, string(body),
		)
	}

	// Decode the wrapper and pull out the version
	var apiResp apiResponse

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("invalid JSON from API: %w", err)
	}

	zipName := apiResp.Result.LatestVersion
	downloadURL := fmt.Sprintf(
		"https://automation-library.ibm.com/files/files/%s",
		zipName,
	)
	return &IntegrationDef{
		Name:        apiResp.Result.Name,
		Version:     zipName,
		DownloadURL: downloadURL,
	}, nil
}

// DownloadPackage downloads and extracts the integration package
// Returns the path to the downloaded zip file
func DownloadPackage(def *IntegrationDef, targetDir string) (string, error) {
	fmt.Printf("Downloading from URL: %s\n", def.DownloadURL)
	rsp, err := http.Get(def.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download from %s: %w", def.DownloadURL, err)
	}
	defer rsp.Body.Close()

	// Check response status code
	if rsp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(rsp.Body)
		return "", fmt.Errorf("download failed with status %d: %s", rsp.StatusCode, string(body))
	}

	// create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	// assume a zip archive
	// Note: def.Version already includes the .zip extension, so we don't add it again
	zipPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%s", def.Name, def.Version))
	fmt.Printf("Saving zip file to: %s\n", zipPath)
	f, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}

	// Copy the response body to the file
	bytesWritten, err := io.Copy(f, rsp.Body)
	if err != nil {
		f.Close()
		return zipPath, fmt.Errorf("failed to write zip file: %w", err)
	}
	f.Close()

	fmt.Printf("Downloaded %d bytes to %s\n", bytesWritten, zipPath)

	return zipPath, nil
}

// Unzip is a helper to extract zip archives
func Unzip(src, dest string) error {
	// Open the zip file
	reader, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract each file
	for _, file := range reader.File {
		err := extractFile(file, dest)
		if err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}

// extractFile extracts a single file from the zip archive
func extractFile(file *zip.File, dest string) error {
	// Prepare full path for the file
	filePath := filepath.Join(dest, file.Name)

	// Check for zip slip vulnerability
	if !filepath.IsLocal(file.Name) {
		return fmt.Errorf("illegal file path: %s", file.Name)
	}

	// If it's a directory, create it and return
	if file.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, file.Mode()); err != nil {
			return err
		}
		return nil
	}

	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// Create the file
	outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Open the file in the archive
	inFile, err := file.Open()
	if err != nil {
		return err
	}
	defer inFile.Close()

	// Copy the file content
	_, err = io.Copy(outFile, inFile)
	return err
}
