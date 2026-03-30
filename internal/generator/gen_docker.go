package generator

import (
	"fmt"
	"path/filepath"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

type dockerTemplateData struct {
	Module   string
	DBSchema string
	Project  ir.ProjectConfig
}

func (g *Generator) loadDockerTemplate(name string) (string, error) {
	p := filepath.Join(g.tmplDir, "docker", name)
	data, err := readFileBytes(p)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}
	return string(data), nil
}

// GenerateDockerfiles produces Dockerfiles for server, agl, and goose.
func (g *Generator) GenerateDockerfiles() error {
	data := dockerTemplateData{
		Module:   g.Schema.Module,
		DBSchema: g.Schema.DBSchema,
		Project:  g.Schema.Project,
	}

	files := []struct {
		tmpl string
		path string
	}{
		{"server.Dockerfile.tmpl", "build/docker/server.Dockerfile"},
		{"goose.Dockerfile.tmpl", "build/docker/migration.Dockerfile"},
		{"agl.Dockerfile.tmpl", "build/docker/apiserver.Dockerfile"},
	}

	for _, f := range files {
		tmpl, err := g.loadDockerTemplate(f.tmpl)
		if err != nil {
			return err
		}
		out, err := g.execTemplate("docker-"+f.tmpl, tmpl, data)
		if err != nil {
			return err
		}
		if err := g.writeGen(f.path, out); err != nil {
			return err
		}
	}
	return nil
}
