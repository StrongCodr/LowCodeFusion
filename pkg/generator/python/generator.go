// File: pkg/generator/python/generator.go

package python

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strongcodr/lowcodefusion/pkg/fetcher"
)

// GenerateStubs scaffolds Python modules for the integration
func GenerateStubs(def *fetcher.IntegrationDef, targetDir string) error {
	// load schema JSON
	schemaPath := filepath.Join(targetDir, "schema.json")
	schemaData, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	// parse schema (omitted: unmarshal into operations)
	ops := parseOperations(schemaData)

	// prepare output dir
	outPkg := filepath.Join(targetDir, def.Name)
	if err := os.MkdirAll(outPkg, 0755); err != nil {
		return err
	}

	// generate init file
	if err := ioutil.WriteFile(filepath.Join(outPkg, "__init__.py"), []byte(""), 0644); err != nil {
		return err
	}

	// load templates
	funcTmpl := template.Must(template.ParseFiles("templates/python_func.tmpl"))

	// generate code per operation
	for _, op := range ops {
		buf := &bytes.Buffer{}
		if err := funcTmpl.Execute(buf, map[string]interface{}{
			"Op":  op,
			"Def": def,
		}); err != nil {
			return err
		}
		file := filepath.Join(outPkg, fmt.Sprintf("%s.py", op.Name))
		if err := ioutil.WriteFile(file, buf.Bytes(), 0644); err != nil {
			return err
		}
	}

	return nil
}
