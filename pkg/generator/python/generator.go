// File: pkg/generator/python/generator.go

package python

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/strongcodr/lowcodefusion/pkg/fetcher"
)

// Operation represents a single integration operation
type Operation struct {
	Name        string
	Parameters  []Parameter
	ReturnType  string
	Description string
	ModulePath  string // Path to the module (e.g., "AWS.ec2")
	FilePath    string // Path to the original JSON file
}

// Parameter represents an input to an operation
type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// FlowFile represents the JSON structure of a flow file
type FlowFile struct {
	Name      string    `json:"name"`
	Processes []Process `json:"processes"`
	Meta      FlowMeta  `json:"meta"`
}

// Process represents a process within a flow
type Process struct {
	Name      string     `json:"name"`
	Variables []Variable `json:"variables"`
}

// Variable represents a variable within a process
type Variable struct {
	Name     string       `json:"name"`
	IsInput  bool         `json:"isInput"`
	IsOutput bool         `json:"isOutput"`
	Required bool         `json:"required"`
	Meta     VariableMeta `json:"meta"`
	Type     interface{}  `json:"type"`
}

// VariableMeta contains metadata for a variable
type VariableMeta struct {
	Description string `json:"description"`
}

// FlowMeta contains metadata for a flow
type FlowMeta struct {
	Info string `json:"info"`
}

// TypeDefinition represents a complex type definition
type TypeDefinition struct {
	Name          string
	PythonType    string
	Description   string
	FilePath      string // Path to the file that defines this type
	ModulePath    string // Module path where this type is used (e.g., "AWS.ec2")
	OperationName string // Name of the operation that uses this type (e.g., "RunInstances")
}

// TypeFingerprint represents the structural essence of a type definition
type TypeFingerprint struct {
	BaseType      string
	PropertyNames []string
	PropertyTypes map[string]string
	EnumValues    []string
	Format        string
}

// TypeLocation indicates where a type should be defined
type TypeLocation int

const (
	ServiceSpecific TypeLocation = iota // Type specific to a service
	CommonType                          // Type shared across services
)

// TypeRegistry tracks and manages complex type definitions using a multi-level hierarchy:
// 1. Service-specific common types (shared within a service)
// 2. Operation-specific types (one file per operation)
type TypeRegistry struct {
	// All registered types by name
	Types map[string]TypeDefinition
	// Type fingerprints for deduplication - fingerprint -> canonical type name
	Fingerprints map[string]string
	// Service-level common types (used across multiple operations in a service)
	ServiceCommonTypes map[string]map[string]TypeDefinition // map[serviceName]map[typeName]TypeDefinition
	// Operation-specific types - map[operationName]map[typeName]TypeDefinition
	OperationTypes map[string]map[string]TypeDefinition
	// Track which types are used by which operations
	TypeUsage map[string]map[string]bool // typeName -> map[operationName]bool
	// Track type dependencies
	TypeDependencies map[string]map[string]bool // typeName -> map[dependsOnTypeName]bool
	// Map operation to service - map[operationName]serviceName
	OperationToService map[string]string
	// Initial dir for the registry
	Dir string
}

// NewTypeRegistry creates a new TypeRegistry
func NewTypeRegistry(dir string) *TypeRegistry {
	return &TypeRegistry{
		Types:              make(map[string]TypeDefinition),
		Fingerprints:       make(map[string]string),
		ServiceCommonTypes: make(map[string]map[string]TypeDefinition),
		OperationTypes:     make(map[string]map[string]TypeDefinition),
		TypeUsage:          make(map[string]map[string]bool),
		TypeDependencies:   make(map[string]map[string]bool),
		OperationToService: make(map[string]string),
		Dir:                dir,
	}
}

// RegisterType adds a type to the registry
func (tr *TypeRegistry) RegisterType(
	name string,
	pythonType string,
	description string,
	filePath string,
	modulePath string,
	operationName string,
) TypeDefinition {
	// Normalize the type name
	normalizedName := sanitizeName(name)

	// Check if the type is already registered
	if existing, exists := tr.Types[normalizedName]; exists {
		return existing
	}

	// Create a new type definition
	typeDef := TypeDefinition{
		Name:          normalizedName,
		PythonType:    pythonType,
		Description:   description,
		FilePath:      filePath,
		ModulePath:    modulePath,
		OperationName: operationName,
	}

	// Add to the registry
	tr.Types[normalizedName] = typeDef

	// Get service name from module path
	serviceName := modulePath
	if parts := strings.Split(modulePath, "."); len(parts) > 1 {
		serviceName = parts[1] // Get the service name (e.g., "ec2" from "AWS.ec2")
	}

	// Initialize type usage tracking if needed
	if tr.TypeUsage[normalizedName] == nil {
		tr.TypeUsage[normalizedName] = make(map[string]bool)
	}

	// Mark this type as used by this service
	tr.TypeUsage[normalizedName][serviceName] = true

	return typeDef
}

// FingerprintType generates a unique fingerprint for a type based on its structure
func (tr *TypeRegistry) FingerprintType(typeDef TypeDefinition) (string, error) {
	// TODO: Implement a proper fingerprinting algorithm that considers the structure
	// For now, just use a simplified approach based on the Python type
	return fmt.Sprintf("%s:%s", typeDef.ModulePath, typeDef.PythonType), nil
}

// AnalyzeTypeUsage identifies which types are used across operations within a service
func (tr *TypeRegistry) AnalyzeTypeUsage() {
	// Map to track number of common types per service
	serviceCommonTypeCount := make(map[string]int)
	
	// First pass: identify which operations each type is used in and map operations to services
	fmt.Println("\n=== Type Analysis - First Pass ===")
	
	for typeName, typeDef := range tr.Types {
		// Extract service name from module path (second part only, not the integration name)
		// For example, from "AWS.ec2" we want just "ec2"
		var serviceName string
		parts := strings.Split(typeDef.ModulePath, ".")
		if len(parts) > 1 {
			serviceName = parts[1] // Get the service name (e.g., "ec2" from "AWS.ec2")
		} else {
			// If there's only one part, use it as the service name
			serviceName = parts[0]
		}

		// Initialize operation usage map if needed
		if tr.TypeUsage[typeName] == nil {
			tr.TypeUsage[typeName] = make(map[string]bool)
		}

		// Mark this type as used by this operation
		tr.TypeUsage[typeName][typeDef.OperationName] = true

		// Store the mapping from operation to service
		tr.OperationToService[typeDef.OperationName] = serviceName
		
		// Debug output
		fmt.Printf("- Type %s used by operation %s in service %s\n", typeName, typeDef.OperationName, serviceName)

		// Initialize service common types map if needed
		if tr.ServiceCommonTypes[serviceName] == nil {
			tr.ServiceCommonTypes[serviceName] = make(map[string]TypeDefinition)
		}

		// Initialize operation types map if needed
		if tr.OperationTypes[typeDef.OperationName] == nil {
			tr.OperationTypes[typeDef.OperationName] = make(map[string]TypeDefinition)
		}
	}
	
	fmt.Println("\n=== Type Analysis - Second Pass ===")
	
	// Second pass: determine if types should be in service common or operation-specific
	for typeName, operations := range tr.TypeUsage {
		typeDef := tr.Types[typeName]

		// Get all services that use this type
		serviceMap := make(map[string]bool)
		for operationName := range operations {
			serviceName := tr.OperationToService[operationName]
			serviceMap[serviceName] = true
		}

		// If used in multiple operations within the same service, it's a service-common type
		if len(operations) > 1 && len(serviceMap) == 1 {
			// Get the single service name
			var serviceName string
			for sName := range serviceMap {
				serviceName = sName
				break
			}

			// Initialize service common types map if not already done
			if tr.ServiceCommonTypes[serviceName] == nil {
				tr.ServiceCommonTypes[serviceName] = make(map[string]TypeDefinition)
			}

			// Add to service common types
			tr.ServiceCommonTypes[serviceName][typeName] = typeDef
			
			// Increment common type count for this service
			serviceCommonTypeCount[serviceName]++
			
			// List the operations this type is used in
			opList := make([]string, 0, len(operations))
			for op := range operations {
				opList = append(opList, op)
			}
			sort.Strings(opList) // Sort for consistent output
			
			fmt.Printf("- Common type: %s in service %s (used by %d operations: %s)\n", 
				typeName, serviceName, len(operations), strings.Join(opList, ", "))
		} else {
			// Type is specific to a single operation or used across multiple services
			// For operation-specific, add to that operation's types
			// For cross-service types, we'll still keep them operation-specific for now
			for operationName := range operations {
				// Initialize operation types map if not already done
				if tr.OperationTypes[operationName] == nil {
					tr.OperationTypes[operationName] = make(map[string]TypeDefinition)
				}

				tr.OperationTypes[operationName][typeName] = typeDef
			}
			
			// List the operations this type is used in
			opList := make([]string, 0, len(operations))
			for op := range operations {
				opList = append(opList, op)
			}
			sort.Strings(opList) // Sort for consistent output
			
			if len(operations) == 1 {
				singleOperation := ""
				for op := range operations {
					singleOperation = op
					break
				}
				fmt.Printf("- Operation-specific type: %s (used only by %s)\n", 
					typeName, singleOperation)
			} else {
				serviceList := make([]string, 0, len(serviceMap))
				for s := range serviceMap {
					serviceList = append(serviceList, s)
				}
				sort.Strings(serviceList) // Sort for consistent output
				
				fmt.Printf("- Cross-service type: %s (used by %d operations across %d services: %s)\n", 
					typeName, len(operations), len(serviceMap), strings.Join(serviceList, ", "))
			}
		}
	}
	
	// Print summary of common types per service
	fmt.Println("\n=== Common Types Summary ===")
	if len(serviceCommonTypeCount) == 0 {
		fmt.Println("No common types found across any services")
	} else {
		// Sort services for consistent output
		services := make([]string, 0, len(serviceCommonTypeCount))
		for service := range serviceCommonTypeCount {
			services = append(services, service)
		}
		sort.Strings(services)
		
		for _, service := range services {
			count := serviceCommonTypeCount[service]
			fmt.Printf("Service %s: %d common types identified\n", service, count)
		}
	}
	fmt.Println("===========================\n")
}

// DeduplicateTypes identifies and merges duplicate types
func (tr *TypeRegistry) DeduplicateTypes() error {
	// Generate fingerprints for all types
	for typeName, typeDef := range tr.Types {
		fingerprint, err := tr.FingerprintType(typeDef)
		if err != nil {
			return fmt.Errorf("failed to fingerprint type %s: %w", typeName, err)
		}

		// Check if we've seen this fingerprint before
		if existingType, exists := tr.Fingerprints[fingerprint]; exists {
			// This is a duplicate - update dependency map to point to the canonical type
			if tr.TypeDependencies[typeName] == nil {
				tr.TypeDependencies[typeName] = make(map[string]bool)
			}
			tr.TypeDependencies[typeName][existingType] = true
		} else {
			// First time seeing this fingerprint - register it
			tr.Fingerprints[fingerprint] = typeName
		}
	}

	return nil
}

// WriteTypesFiles generates Python modules with type definitions organized in two levels:
// 1. Service-specific common types (shared within a service)
// 2. Operation-specific types (one file per operation)
func (tr *TypeRegistry) WriteTypesFiles(outDir string) error {
	if len(tr.Types) == 0 {
		return nil // No types to write
	}

	// Run analysis to organize types
	tr.AnalyzeTypeUsage()
	if err := tr.DeduplicateTypes(); err != nil {
		return err
	}

	// Create the types directory directly under the integration directory
	typesDir := filepath.Join(outDir, "_types")
	if err := os.MkdirAll(typesDir, 0755); err != nil {
		return fmt.Errorf("failed to create types directory: %v", err)
	}

	// Create __init__.py file in the types directory
	if err := createInitFile(typesDir); err != nil {
		return err
	}

	// Generate service-specific common types and operation-specific types
	fmt.Println("\n=== Generating Type Files ===")

	for serviceName, commonTypes := range tr.ServiceCommonTypes {
		// Create service directory
		serviceDir := filepath.Join(typesDir, serviceName)
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return fmt.Errorf("failed to create service types directory %s: %v", serviceDir, err)
		}

		// Create __init__.py in the service directory
		if err := createInitFile(serviceDir); err != nil {
			return err
		}

		// Generate common_types.py for service-specific common types
		// Always create the file even if there are no common types to prevent import errors
		commonTypesPath := filepath.Join(serviceDir, "common_types.py")
		if len(commonTypes) > 0 {
			if err := tr.writeTypesFile(commonTypesPath, commonTypes); err != nil {
				return fmt.Errorf("failed to write service common types file for %s: %w", serviceName, err)
			}
			fmt.Printf("- Generated service common types file: %s with %d common types\n", 
				commonTypesPath, len(commonTypes))
			
			// List the common types
			typeNames := make([]string, 0, len(commonTypes))
			for typeName := range commonTypes {
				typeNames = append(typeNames, typeName)
			}
			sort.Strings(typeNames)
			fmt.Printf("  Common types: %s\n", strings.Join(typeNames, ", "))
		} else {
			// Create an empty common_types.py file to prevent import errors
			emptyContent := "# Generated by LowCodeFusion\n# Empty common types file\n"
			
			if err := os.WriteFile(commonTypesPath, []byte(emptyContent), 0644); err != nil {
				return fmt.Errorf("failed to write empty common types file for %s: %w", serviceName, err)
			}
			fmt.Printf("- Generated empty common types file: %s\n", commonTypesPath)
		}
	}

	// Generate operation-specific types files
	for operationName, operationTypes := range tr.OperationTypes {
		if len(operationTypes) == 0 {
			continue
		}

		// Get the service name for this operation
		serviceName := tr.OperationToService[operationName]

		// Create service directory if it doesn't exist
		serviceDir := filepath.Join(typesDir, serviceName)
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return fmt.Errorf("failed to create service types directory %s: %v", serviceDir, err)
		}

		// Create __init__.py in the service directory if it doesn't exist
		if err := createInitFile(serviceDir); err != nil {
			return err
		}

		// Write the operation-specific types file
		operationTypesPath := filepath.Join(serviceDir, operationName+"_types.py")
		if err := tr.writeTypesFile(operationTypesPath, operationTypes); err != nil {
			return fmt.Errorf("failed to write operation types file for %s: %w", operationName, err)
		}

		fmt.Printf("- Generated operation types file: %s with %d types\n", 
			operationTypesPath, len(operationTypes))
		
		// List the operation-specific types that aren't already in common types
		typeNames := make([]string, 0, len(operationTypes))
		for typeName := range operationTypes {
			// Skip types that are already in the service's common types
			if tr.ServiceCommonTypes[serviceName] != nil && 
			   tr.ServiceCommonTypes[serviceName][typeName] != (TypeDefinition{}) {
				continue
			}
			typeNames = append(typeNames, typeName)
		}
		if len(typeNames) > 0 {
			sort.Strings(typeNames)
			fmt.Printf("  Operation-specific types: %s\n", strings.Join(typeNames, ", "))
		}
	}
	
	fmt.Println("===========================")

	return nil
}

// writeTypesFile writes a collection of type definitions to a file
func (tr *TypeRegistry) writeTypesFile(filePath string, types map[string]TypeDefinition) error {
	// Generate file content
	content := "# Generated by LowCodeFusion\n"
	content += "from typing import Any, Dict, List, Optional, Union, TypedDict, Literal\n"
	content += "from datetime import datetime\n"

	// Add appropriate imports based on file type
	if strings.Contains(filepath.Base(filePath), "common_types.py") {
		// Service-level common types don't need to import other files
	} else {
		// For operation-specific types, import service-level common types
		serviceDir := filepath.Dir(filePath)
		_ = filepath.Base(serviceDir) // Get service name (unused)
		content += fmt.Sprintf("from .common_types import *  # Import service common types\n")
	}

	content += "\n"

	// Extract JSON schemas and generate rich type definitions
	generatedTypes := make(map[string]bool)

	// Process types in a deterministic order for consistency
	typeNames := make([]string, 0, len(types))
	for typeName := range types {
		typeNames = append(typeNames, typeName)
	}

	// Sort type names for consistent output
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		typeDef := types[typeName]

		// Skip if this type is already in the service's common types
		serviceName := tr.OperationToService[typeDef.OperationName]
		if !strings.Contains(filePath, "common_types.py") &&
			tr.ServiceCommonTypes[serviceName] != nil &&
			tr.ServiceCommonTypes[serviceName][typeName] != (TypeDefinition{}) {
			continue
		}

		// We need to parse the original JSON file to extract detailed schema information
		fileContent, err := os.ReadFile(typeDef.FilePath)
		if err != nil {
			fmt.Printf("Warning: Could not read file %s: %v\n", typeDef.FilePath, err)
			continue
		}

		var flowFile FlowFile
		if err := json.Unmarshal(fileContent, &flowFile); err != nil {
			fmt.Printf("Warning: Could not parse JSON from %s: %v\n", typeDef.FilePath, err)
			continue
		}

		// Process only the first process (should be the main one)
		if len(flowFile.Processes) == 0 {
			continue
		}
		process := flowFile.Processes[0]

		// Find the variable that matches this type
		for _, variable := range process.Variables {
			// Skip variables without types
			if variable.Type == nil {
				continue
			}

			// See if this is a parameter or return type that we're looking for
			isMatch := false
			if strings.HasSuffix(typeDef.Name, "_Result_Type") && variable.IsOutput {
				isMatch = true
			} else if strings.Contains(typeDef.Name, "_"+variable.Name+"_Type") && variable.IsInput {
				isMatch = true
			}

			if !isMatch {
				continue
			}

			// Process the type
			typeObj, ok := variable.Type.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract definitions if they exist
			definitions := make(map[string]interface{})
			if defs, ok := typeObj["definitions"].(map[string]interface{}); ok {
				definitions = defs
			}

			// Parse the schema
			schema := jsonTypeToSchemaType(typeDef.Name, typeObj, definitions)
			schema.IsRoot = true

			// Generate TypedDict classes for all complex types
			if schema.Type == "object" && len(schema.Properties) > 0 {
				// Generate TypedDict for the root object
				typeDictCode := generatePythonTypedDict(schema, generatedTypes)
				content += fmt.Sprintf("# %s\n", typeDef.Description)
				content += fmt.Sprintf("# From: %s\n", typeDef.FilePath)
				content += typeDictCode

				// Mark as generated
				generatedTypes[schema.Name] = true

				// Generate TypedDict classes for all nested definitions
				if schema.Definitions != nil {
					for defName, defSchema := range schema.Definitions {
						if defSchema.Type == "object" && len(defSchema.Properties) > 0 && !generatedTypes[defName] {
							typeDictCode := generatePythonTypedDict(defSchema, generatedTypes)
							content += typeDictCode
							generatedTypes[defName] = true
						}
					}
				}
			} else {
				// For non-object types, use the simplified representation
				content += fmt.Sprintf("# %s\n", typeDef.Description)
				content += fmt.Sprintf("# From: %s\n", typeDef.FilePath)
				content += fmt.Sprintf("%s = %s\n\n", typeDef.Name, typeDef.PythonType)
			}
		}
	}

	// Write to file
	return os.WriteFile(filePath, []byte(content), 0644)
}

// sanitizeName converts a name to a valid Python identifier
func sanitizeName(name string) string {
	// Replace spaces and other non-alphanumeric characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	return re.ReplaceAllString(name, "_")
}

// SchemaType represents a type extracted from a JSON schema
type SchemaType struct {
	Name        string                // Name of the type
	Type        string                // Type (string, integer, object, array, etc.)
	Format      string                // Format (date-time, etc.)
	Description string                // Description of the type
	Properties  map[string]SchemaType // Object properties
	Items       *SchemaType           // Array item type
	Enum        []string              // Enum values
	Ref         string                // Reference to another type
	Required    []string              // Required properties
	OneOf       []SchemaType          // OneOf variants
	IsRoot      bool                  // Is this a root type (not a nested type)
	Definitions map[string]SchemaType // Type definitions (for root types)
}

// pathTracker is used to track the JSON schema reference path to detect circular references
type pathTracker struct {
	paths map[string]bool
}

// newPathTracker creates a new pathTracker
func newPathTracker() *pathTracker {
	return &pathTracker{
		paths: make(map[string]bool),
	}
}

// has checks if a path has been visited
func (p *pathTracker) has(path string) bool {
	return p.paths[path]
}

// add marks a path as visited
func (p *pathTracker) add(path string) {
	p.paths[path] = true
}

// remove marks a path as no longer visited
func (p *pathTracker) remove(path string) {
	delete(p.paths, path)
}

// jsonTypeToSchemaType converts a JSON schema object to a SchemaType
func jsonTypeToSchemaType(typeName string, typeInfo interface{}, definitions map[string]interface{}) SchemaType {
	return jsonTypeToSchemaTypeWithTracker(typeName, typeInfo, definitions, newPathTracker())
}

// jsonTypeToSchemaTypeWithTracker converts a JSON schema object to a SchemaType with path tracking to avoid circular references
func jsonTypeToSchemaTypeWithTracker(
	typeName string,
	typeInfo interface{},
	definitions map[string]interface{},
	tracker *pathTracker,
) SchemaType {
	schemaType := SchemaType{
		Name:       typeName,
		Properties: make(map[string]SchemaType),
	}

	// Handle simple string type
	if typeStr, ok := typeInfo.(string); ok {
		schemaType.Type = typeStr
		return schemaType
	}

	// Handle complex type (object with properties)
	if typeObj, ok := typeInfo.(map[string]interface{}); ok {
		// Get direct type property
		if typeType, ok := typeObj["type"].(string); ok {
			schemaType.Type = typeType
		}

		// Get format if available
		if format, ok := typeObj["format"].(string); ok {
			schemaType.Format = format
		}

		// Get description if available
		if desc, ok := typeObj["description"].(string); ok {
			schemaType.Description = desc
		}

		// Get required properties
		if req, ok := typeObj["required"].([]interface{}); ok {
			for _, r := range req {
				if reqStr, ok := r.(string); ok {
					schemaType.Required = append(schemaType.Required, reqStr)
				}
			}
		}

		// Handle array type
		if schemaType.Type == "array" {
			if items, ok := typeObj["items"].(map[string]interface{}); ok {
				// Check for circular reference
				itemPath := typeName + ".items"
				if !tracker.has(itemPath) {
					tracker.add(itemPath)
					itemType := jsonTypeToSchemaTypeWithTracker(typeName+"Item", items, definitions, tracker)
					schemaType.Items = &itemType
					tracker.remove(itemPath)
				} else {
					// Circular reference detected, use Any for items
					schemaType.Items = &SchemaType{Name: "Any", Type: "any"}
				}
			}
		}

		// Handle object type with properties
		if props, ok := typeObj["properties"].(map[string]interface{}); ok &&
			(schemaType.Type == "object" || schemaType.Type == "") {
			schemaType.Type = "object"

			// Process a limited number of properties to avoid stack overflow
			propCount := 0
			for propName, propType := range props {
				// Only process a reasonable number of properties (this is a safety measure)
				if propCount >= 100 {
					break
				}

				// Check for circular reference
				propPath := typeName + ".properties." + propName
				if !tracker.has(propPath) {
					tracker.add(propPath)
					schemaType.Properties[propName] = jsonTypeToSchemaTypeWithTracker(
						propName,
						propType,
						definitions,
						tracker,
					)
					tracker.remove(propPath)
				} else {
					// Circular reference detected, use Any for this property
					schemaType.Properties[propName] = SchemaType{Name: propName, Type: "any"}
				}
				propCount++
			}
		}

		// Handle enum values
		if enumValues, ok := typeObj["enum"].([]interface{}); ok {
			for _, val := range enumValues {
				if strVal, ok := val.(string); ok {
					schemaType.Enum = append(schemaType.Enum, strVal)
				} else if numVal, ok := val.(float64); ok {
					schemaType.Enum = append(schemaType.Enum, fmt.Sprintf("%v", numVal))
				} else if boolVal, ok := val.(bool); ok {
					schemaType.Enum = append(schemaType.Enum, fmt.Sprintf("%v", boolVal))
				}
			}
		}

		// Handle schema reference
		if ref, ok := typeObj["$ref"].(string); ok {
			schemaType.Ref = ref
			// Extract the referenced type name
			parts := strings.Split(ref, "/")
			if len(parts) > 0 {
				refTypeName := parts[len(parts)-1]

				// Check for circular reference
				refPath := fmt.Sprintf("$ref:%s", ref)
				if !tracker.has(refPath) && len(parts) > 1 && parts[1] == "definitions" && definitions != nil {
					// If it's a reference to a definition, try to resolve it
					if defType, ok := definitions[refTypeName]; ok {
						tracker.add(refPath)
						// Get the basic type information from the reference, but don't resolve nested references
						refSchema := jsonTypeToSchemaTypeWithTracker(refTypeName, defType, definitions, tracker)
						tracker.remove(refPath)

						// Just copy the essential info without deep nesting
						schemaType.Type = refSchema.Type
						schemaType.Format = refSchema.Format
						// Don't copy properties deeply to avoid circular refs
					}
				}
				// We'll just keep the reference name for later use
			}
		}

		// Handle oneOf - limit depth to avoid recursion
		if oneOfList, ok := typeObj["oneOf"].([]interface{}); ok && len(oneOfList) < 10 {
			oneOfPath := typeName + ".oneOf"
			if !tracker.has(oneOfPath) {
				tracker.add(oneOfPath)
				for i, oneOfType := range oneOfList {
					// Limit to 5 oneOf variants to avoid explosion
					if i >= 5 {
						break
					}
					oneOfSchema := jsonTypeToSchemaTypeWithTracker(typeName+"OneOf", oneOfType, definitions, tracker)
					schemaType.OneOf = append(schemaType.OneOf, oneOfSchema)
				}
				tracker.remove(oneOfPath)
			}
		}

		// Handle definitions (only for root types) - with limits
		if defs, ok := typeObj["definitions"].(map[string]interface{}); ok {
			schemaType.Definitions = make(map[string]SchemaType)
			defCount := 0

			for defName, defType := range defs {
				// Only process a reasonable number of definitions
				if defCount >= 50 {
					break
				}

				// Check for circular reference
				defPath := "definitions." + defName
				if !tracker.has(defPath) {
					tracker.add(defPath)
					schemaType.Definitions[defName] = jsonTypeToSchemaTypeWithTracker(defName, defType, defs, tracker)
					tracker.remove(defPath)
				} else {
					// Just create a placeholder for circular references
					schemaType.Definitions[defName] = SchemaType{Name: defName, Type: "any"}
				}
				defCount++
			}
		}
	}

	return schemaType
}

// schemaTypeToPythonType converts a SchemaType to a Python type string
func schemaTypeToPythonType(schema SchemaType, rootTypes map[string]bool) string {
	// Handle references first - they override the type
	if schema.Ref != "" {
		// Extract the referenced type name
		parts := strings.Split(schema.Ref, "/")
		if len(parts) > 0 {
			refTypeName := parts[len(parts)-1]
			return sanitizeName(refTypeName)
		}
	}

	// Handle different types
	switch schema.Type {
	case "string":
		// Handle string with enum values
		if len(schema.Enum) > 0 {
			// Use Literal["val1", "val2", ...] syntax
			enumValues := make([]string, 0, len(schema.Enum))
			for _, val := range schema.Enum {
				enumValues = append(enumValues, fmt.Sprintf("%q", val))
			}
			return fmt.Sprintf("Literal[%s]", strings.Join(enumValues, ", "))
		}
		if schema.Format == "date-time" {
			return "datetime"
		}
		return "str"
	case "integer", "number":
		return "int"
	case "boolean":
		return "bool"
	case "array":
		if schema.Items != nil {
			itemType := schemaTypeToPythonType(*schema.Items, rootTypes)
			return fmt.Sprintf("List[%s]", itemType)
		}
		return "List[Any]"
	case "object":
		// If this is a root type, it should have a registered TypedDict
		if rootTypes[schema.Name] {
			return schema.Name
		}
		return "Dict[str, Any]"
	default:
		// For oneOf, try to build a Union type
		if len(schema.OneOf) > 0 {
			types := make([]string, 0, len(schema.OneOf))
			for _, oneOfType := range schema.OneOf {
				types = append(types, schemaTypeToPythonType(oneOfType, rootTypes))
			}
			return fmt.Sprintf("Union[%s]", strings.Join(types, ", "))
		}
		return "Any"
	}
}

// generatePythonTypedDict generates Python TypedDict code for a SchemaType
func generatePythonTypedDict(schema SchemaType, rootTypes map[string]bool) string {
	result := ""

	// Generate docstring if description exists
	if schema.Description != "" {
		result += fmt.Sprintf("class %s(TypedDict, total=False):\n", schema.Name)
		result += fmt.Sprintf("    \"\"\"%s\"\"\"\n", schema.Description)
	} else {
		result += fmt.Sprintf("class %s(TypedDict, total=False):\n", schema.Name)
	}

	// Add properties
	if len(schema.Properties) > 0 {
		for propName, propType := range schema.Properties {
			pythonType := schemaTypeToPythonType(propType, rootTypes)

			// Check if property is required
			isRequired := false
			for _, req := range schema.Required {
				if req == propName {
					isRequired = true
					break
				}
			}

			// Add Optional wrapper if not required
			if !isRequired {
				pythonType = fmt.Sprintf("Optional[%s]", pythonType)
			}

			// Add property with type annotation
			if propType.Description != "" {
				// Format multi-line descriptions by replacing newlines with proper indentation
				description := strings.ReplaceAll(propType.Description, "\n", "\n    # ")
				result += fmt.Sprintf("    %s: %s  # %s\n", propName, pythonType, description)
			} else {
				result += fmt.Sprintf("    %s: %s\n", propName, pythonType)
			}
		}
	} else {
		// Empty TypedDict needs a pass
		result += "    pass\n"
	}

	return result + "\n"
}

// jsonTypeToGoPythonType converts a JSON schema type to a Python type
func jsonTypeToGoPythonType(typeInfo interface{}) string {
	// This is now a simplified version that returns basic types
	// The detailed type generation is handled by SchemaType and generatePythonTypedDict

	// Handle simple string type
	if typeStr, ok := typeInfo.(string); ok {
		switch typeStr {
		case "string":
			return "str"
		case "integer", "number":
			return "int"
		case "boolean":
			return "bool"
		case "array":
			return "List[Any]"
		case "object", "map":
			return "Dict[str, Any]"
		default:
			return "Any"
		}
	}

	// Handle complex type (object with properties)
	if typeObj, ok := typeInfo.(map[string]interface{}); ok {
		// Check for direct type property
		if typeType, ok := typeObj["type"].(string); ok {
			switch typeType {
			case "string":
				return "str"
			case "integer", "number":
				return "int"
			case "boolean":
				return "bool"
			case "array":
				return "List[Any]"
			case "object":
				return "Dict[str, Any]"
			}
		}

		// Check for schema reference
		if ref, ok := typeObj["$ref"].(string); ok {
			// Reference to another schema definition
			parts := strings.Split(ref, "/")
			if len(parts) > 0 {
				typeName := parts[len(parts)-1]
				return sanitizeName(typeName)
			}
		}

		// If we have a oneOf, use Any for now
		if _, ok := typeObj["oneOf"].([]interface{}); ok {
			return "Any"
		}
	}

	// Default to Any for complex or unknown types
	return "Any"
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
		// The prefix is used for import organization but doesn't affect the directory structure
		dir := filepath.Dir(relPath)
		if dir == "." {
			dir = ""
		}

		// Store the module path for imports and organization
		// We've fixed the directories elsewhere, but we keep the module path format
		// for compatibility with the rest of the code
		modulePath := strings.ReplaceAll(integrationName, " ", "_")
		if dir != "" {
			dirPath := strings.ReplaceAll(dir, string(filepath.Separator), ".")
			dirPath = strings.ReplaceAll(dirPath, " ", "_")
			modulePath = fmt.Sprintf("%s.%s", modulePath, dirPath)
		}

		// Read and parse the flow file
		fileContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %v", path, err)
		}

		var flowFile FlowFile
		if err := json.Unmarshal(fileContent, &flowFile); err != nil {
			return fmt.Errorf("error parsing JSON from %s: %v", path, err)
		}

		// Check if there's more than one process
		if len(flowFile.Processes) != 1 {
			return fmt.Errorf("file %s has %d processes, expected exactly 1", path, len(flowFile.Processes))
		}

		process := flowFile.Processes[0]

		// Get operation name from the flow name
		opName := sanitizeName(flowFile.Name)

		// Create operation
		op := Operation{
			Name:        opName,
			Parameters:  []Parameter{},
			ReturnType:  "None", // Default return type
			Description: flowFile.Meta.Info,
			ModulePath:  modulePath,
			FilePath:    path,
		}

		// Process variables
		for _, variable := range process.Variables {
			// Process input parameters
			if variable.IsInput {
				param := Parameter{
					Name:        sanitizeName(variable.Name),
					Type:        jsonTypeToGoPythonType(variable.Type),
					Required:    variable.Required,
					Description: variable.Meta.Description,
				}
				op.Parameters = append(op.Parameters, param)
			}

			// Process output (return type)
			if variable.IsOutput {
				op.ReturnType = jsonTypeToGoPythonType(variable.Type)
				// We could store more info about return type here if needed
			}
		}

		operations = append(operations, op)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return operations, nil
}

// generatePythonStub creates a Python stub file for an operation using a template
func generatePythonStub(op Operation, outPath string) error {
	// Read the template file
	tmplPath := "templates/python_func.tmpl"
	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %v", tmplPath, err)
	}

	// Create a new template
	tmpl, err := template.New("python_func").Funcs(template.FuncMap{
		"split": strings.Split,
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	// Create a template data structure
	data := struct {
		Op  Operation
		Def struct {
			Name string
		}
	}{
		Op: op,
	}

	// Get the integration name from the module path
	if parts := strings.Split(op.ModulePath, "."); len(parts) > 0 {
		data.Def.Name = parts[0]
	}

	// Create a buffer for the output
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Write to file
	if err := os.WriteFile(outPath, buffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %v", outPath, err)
	}

	return nil
}

// createInitFile creates __init__.py files in all parent directories
func createInitFile(dirPath string) error {
	// Create __init__.py file
	initPath := filepath.Join(dirPath, "__init__.py")

	// Check if file already exists
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		// Create empty __init__.py file
		if err := os.WriteFile(initPath, []byte("# Generated by LowCodeFusion\n"), 0644); err != nil {
			return fmt.Errorf("failed to create __init__.py in %s: %v", dirPath, err)
		}
	}

	return nil
}

// analyzeComplexTypes examines operation parameters and return types to identify complex types
func analyzeComplexTypes(ops []Operation, registry *TypeRegistry) error {
	for _, op := range ops {
		// Check for complex parameter types
		for _, param := range op.Parameters {
			// Only register Dict and List types that have specific formats
			if strings.HasPrefix(param.Type, "Dict") || strings.HasPrefix(param.Type, "List") {
				// Register this as a potential complex type
				typeName := fmt.Sprintf("%s_%s_Type", op.Name, param.Name)
				registry.RegisterType(
					typeName,
					param.Type,
					fmt.Sprintf("Type definition for parameter %s in %s", param.Name, op.Name),
					op.FilePath,
					op.ModulePath,
					op.Name, // Pass operation name
				)
			}
		}

		// Check for complex return type
		if strings.HasPrefix(op.ReturnType, "Dict") || strings.HasPrefix(op.ReturnType, "List") {
			// Register this as a potential complex type
			typeName := fmt.Sprintf("%s_Result_Type", op.Name)
			registry.RegisterType(
				typeName,
				op.ReturnType,
				fmt.Sprintf("Type definition for return value of %s", op.Name),
				op.FilePath,
				op.ModulePath,
				op.Name, // Pass operation name
			)
		}
	}

	return nil
}

// GenerateStubs scaffolds Python modules for the integration
func GenerateStubs(def *fetcher.IntegrationDef, srcDir, outDir string) error {
	// Parse operations from directory structure
	ops, err := parseOperations(srcDir, def.Name)
	if err != nil {
		return err
	}

	// Create a type registry
	typeRegistry := NewTypeRegistry(outDir)

	// Analyze operations for complex types
	if err := analyzeComplexTypes(ops, typeRegistry); err != nil {
		return err
	}

	// Create the base integration directory

	integrationDir := filepath.Join(outDir, def.Name)
	if err := os.MkdirAll(integrationDir, 0755); err != nil {
		return fmt.Errorf("failed to create integration directory %s: %v", integrationDir, err)
	}
	if err := createInitFile(integrationDir); err != nil {
		return err
	}

	// Debug the directory names to understand the issue
	fmt.Printf("Integration directory: %s\n", integrationDir)

	// Generate type definitions directly in the integration directory
	if err := typeRegistry.WriteTypesFiles(integrationDir); err != nil {
		return err
	}

	// Print the paths as they would appear in the final Python library
	fmt.Println("Generating Python stubs:")
	moduleMap := make(map[string]bool)

	for _, op := range ops {
		modulePath := op.ModulePath
		if !moduleMap[modulePath] {
			moduleMap[modulePath] = true
			fmt.Printf("- Module: %s\n", modulePath)
		}

		// Extract the service part from the module path (skip the integration name)
		// AWS.ec2 -> ec2
		parts := strings.Split(modulePath, ".")
		var servicePath string
		if len(parts) > 1 {
			// Skip the integration name, join the rest
			servicePath = strings.Join(parts[1:], string(filepath.Separator))
		} else {
			servicePath = "" // Root service
		}

		// Create full path for the output file directly under the integration dir
		// outDir/AWS/ec2/RunInstances.py instead of outDir/AWS/AWS/ec2/RunInstances.py
		opDirPath := filepath.Join(integrationDir, servicePath)
		opFilePath := filepath.Join(opDirPath, fmt.Sprintf("%s.py", op.Name))

		// Create __init__.py files in all parent directories
		dirPath := integrationDir
		for _, part := range strings.Split(servicePath, string(filepath.Separator)) {
			if part == "" {
				continue
			}
			dirPath = filepath.Join(dirPath, part)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", dirPath, err)
			}
			if err := createInitFile(dirPath); err != nil {
				return err
			}
		}

		// Generate Python stub file
		if err := generatePythonStub(op, opFilePath); err != nil {
			return err
		}

		fmt.Printf("  - Generated: %s\n", opFilePath)
	}

	fmt.Printf("\nSuccessfully generated %d Python stub files\n", len(ops))
	return nil
}
