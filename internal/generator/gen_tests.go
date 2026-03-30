package generator

import (
	"fmt"
	"path/filepath"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

type testTemplateData struct {
	DBSchema  string
	Project   ir.ProjectConfig
	Resources []ir.Resource
}

func (g *Generator) loadTestTemplate(name string) (string, error) {
	p := filepath.Join(g.tmplDir, "tests", name)
	data, err := readFileBytes(p)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}
	return string(data), nil
}

func (g *Generator) GenerateTests() error {
	data := testTemplateData{
		DBSchema:  g.Schema.DBSchema,
		Project:   g.Schema.Project,
		Resources: g.Schema.Resources,
	}

	templates := []struct {
		tmplFile string
		outFile  string
	}{
		{"postman_collection.json.tmpl", "tests/postman_collection.json"},
		{"agl_postman_collection.json.tmpl", "tests/agl_postman_collection.json"},
		{"test_data.sql.tmpl", "tests/test-data.sql"},
	}

	for _, tf := range templates {
		tmplStr, err := g.loadTestTemplate(tf.tmplFile)
		if err != nil {
			return err
		}
		out, err := g.execTemplate(tf.tmplFile, tmplStr, data)
		if err != nil {
			return fmt.Errorf("exec %s: %w", tf.tmplFile, err)
		}
		if err := g.writeGen(tf.outFile, out); err != nil {
			return err
		}
	}
	return nil
}
