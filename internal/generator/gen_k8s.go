package generator

import (
	"fmt"
	"path/filepath"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

type k8sTemplateData struct {
	Module    string
	Project   ir.ProjectConfig
	DBSchema  string
	Namespace string
}

func (g *Generator) loadK8sTemplate(name string) (string, error) {
	p := filepath.Join(g.tmplDir, "k8s", name)
	data, err := readFileBytes(p)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}
	return string(data), nil
}

func (g *Generator) GenerateK8sManifests() error {
	proj := g.Schema.Project
	data := k8sTemplateData{
		Module:    g.Schema.Module,
		Project:   proj,
		DBSchema:  g.Schema.DBSchema,
		Namespace: proj.Name + "-system",
	}

	files := []struct {
		tmpl string
		path string
	}{
		{"postgres.yaml.tmpl", "deploy/postgres.yaml"},
		{"backend.yaml.tmpl", "deploy/backend.yaml"},
		{"apiserver.yaml.tmpl", "deploy/apiserver.yaml"},
	}

	for _, f := range files {
		tmpl, err := g.loadK8sTemplate(f.tmpl)
		if err != nil {
			return err
		}
		out, err := g.execTemplate("k8s-"+f.tmpl, tmpl, data)
		if err != nil {
			return err
		}
		if err := g.writeGen(f.path, out); err != nil {
			return err
		}
	}
	return nil
}
