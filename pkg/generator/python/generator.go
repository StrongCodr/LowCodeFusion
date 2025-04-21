// File: pkg/generator/python/generator.go

package python

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strongcodr/lowcodefusion/pkg/fetcher"
)

// Operation represents a single integration operation
type Operation struct {
	Name        string
	Parameters  []Parameter
	Description string
	ModulePath  string // Path to the module (e.g., "AWS.ec2")
}

// Parameter represents an input to an operation
type Parameter struct {
	Name string
	Type string
}

// parseOperations scans the directory structure and returns operations
func parseOperations(srcDir string, integrationName string) ([]Operation, error) {
	var operations []Operation

	// Find the flows directory
	flowsDir := filepath.Join(srcDir, "flows")
	if _, err := os.Stat(flowsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("flows directory not found in %s", srcDir)
	}

	// Find the integration directory (e.g., AWS)
	integrationDir := filepath.Join(flowsDir, integrationName)
	if _, err := os.Stat(integrationDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("integration directory %s not found in flows", integrationName)
	}

	// Walk through the directory structure
	err := filepath.Walk(integrationDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process JSON files
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil
		}

		// Get the relative path from the integration directory
		relPath, err := filepath.Rel(integrationDir, path)
		if err != nil {
			return err
		}

		// Convert file path to module path
		// e.g., "ec2/DescribeIdFormat.json" -> "AWS.ec2"
		dir := filepath.Dir(relPath)
		if dir == "." {
			dir = ""
		}

		modulePath := strings.ReplaceAll(integrationName, " ", "_")
		if dir != "" {
			dirPath := strings.ReplaceAll(dir, string(filepath.Separator), ".")
			dirPath = strings.ReplaceAll(dirPath, " ", "_")
			modulePath = fmt.Sprintf("%s.%s", modulePath, dirPath)
		}

		// Get operation name from filename without extension
		opName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		opName = strings.ReplaceAll(opName, " ", "_")

		// Create operation
		op := Operation{
			Name:        opName,
			Parameters:  []Parameter{}, // We'll parse these later
			Description: fmt.Sprintf("Operation from %s", relPath),
			ModulePath:  modulePath,
		}

		operations = append(operations, op)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return operations, nil
}

// GenerateStubs scaffolds Python modules for the integration
func GenerateStubs(def *fetcher.IntegrationDef, srcDir, outDir string) error {
	// Parse operations from directory structure
	ops, err := parseOperations(srcDir, def.Name)
	if err != nil {
		return err
	}

	// Print the paths as they would appear in the final Python library
	fmt.Println("Python module paths:")
	moduleMap := make(map[string]bool)
	for _, op := range ops {
		modulePath := op.ModulePath
		if !moduleMap[modulePath] {
			moduleMap[modulePath] = true
			fmt.Printf("- %s\n", modulePath)
		}
		fmt.Printf("  - %s.%s\n", modulePath, op.Name)
	}

	// For now, we're just outputting the paths, not generating the actual files
	fmt.Println("\nTotal operations found:", len(ops))

	return nil
}
