package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

// Generator orchestrates code generation from IR.
type Generator struct {
	Schema    *ir.Schema
	OutputDir string
	tmplDir   string
	funcMap   template.FuncMap
}

// New creates a Generator with embedded template helpers.
func New(schema *ir.Schema, outputDir, tmplDir string) *Generator {
	g := &Generator{
		Schema:    schema,
		OutputDir: outputDir,
		tmplDir:   tmplDir,
	}
	g.funcMap = template.FuncMap{
		"lower":            strings.ToLower,
		"upper":            strings.ToUpper,
		"title":            strings.Title, //nolint:staticcheck
		"snakeCase":        toSnakeCase,
		"camelCase":        toCamelCase,
		"kebab":            toKebabCase,
		"quote":            func(s string) string { return fmt.Sprintf("'%s'", s) },
		"joinQuoted":       joinQuoted,
		"repeat":           strings.Repeat,
		"sub":              func(a, b int) int { return a - b },
		"add":              func(a, b int) int { return a + b },
		"mod":              func(a, b int) int { return a % b },
		"seq":              seq,
		"placeholders":     placeholders,
		"dict":             dict,
		"contains":         strings.Contains,
		"trimPrefix":       strings.TrimPrefix,
		"resPlural":        resPlural,
		"resGoPlural":      resGoPlural,
		"httpPathRes":      httpPathRes,
		"httpPathComposite": httpPathComposite,
		"hasPrefix":        strings.HasPrefix,
		"hasSuffix":        strings.HasSuffix,
		"join":             strings.Join,
		"k8sPlural":        k8sResourcePlural,
		"k8sSingular":      k8sResourceSingular,
		"goField":          goFieldName,
		"isEnumType": func(typeName string) bool {
			typeName = strings.TrimSuffix(typeName, "[]")
			for _, ct := range schema.Types {
				if ct.Name == typeName && ct.Kind == "enum" {
					return true
				}
			}
			return false
		},
		"isCompositeType": func(typeName string) bool {
			typeName = strings.TrimSuffix(typeName, "[]")
			for _, ct := range schema.Types {
				if ct.Name == typeName && ct.Kind == "composite" {
					return true
				}
			}
			return false
		},
		"resolveComposite": func(typeName string) *ir.CustomType {
			typeName = strings.TrimSuffix(typeName, "[]")
			for i := range schema.Types {
				if schema.Types[i].Name == typeName {
					return &schema.Types[i]
				}
			}
			return nil
		},
		"k8sTypeName": func(typeName string) string {
			typeName = strings.TrimSuffix(typeName, "[]")
			for _, ct := range schema.Types {
				if ct.Name == typeName {
					return ct.K8sName
				}
			}
			return goFieldName(typeName)
		},
		"protoWrapperName": func(fieldName string) string {
			wrapperMap := map[string]string{
				"types": "ICMPTypes",
			}
			if name, ok := wrapperMap[fieldName]; ok {
				return name
			}
			return goFieldName(fieldName) + "Wrapper"
		},
		"jsonFieldName": func(f ir.SpecField) string {
			if f.JSONName != "" {
				return f.JSONName
			}
			return toCamelCase(f.Name)
		},
		"testSpecJSON": func(res ir.Resource) string {
			var parts []string
			for _, f := range res.Spec.Fields {
				if f.OutputOnly {
					continue
				}
				jsonName := f.JSONName
				if jsonName == "" {
					jsonName = toCamelCase(f.Name)
				}
				switch {
				case f.GoType == "bool":
					parts = append(parts, fmt.Sprintf(",\"%s\":true", jsonName))
				case f.GoType == "PolicyAction":
					parts = append(parts, fmt.Sprintf(",\"%s\":\"DENY\"", jsonName))
				case f.GoType == "IPNet":
					parts = append(parts, fmt.Sprintf(",\"%s\":\"10.0.0.0/8\"", jsonName))
				default:
					parts = append(parts, fmt.Sprintf(",\"%s\":\"test\"", jsonName))
				}
			}
			return strings.Join(parts, "")
		},
	}
	return g
}

// Run generates all targets.
func (g *Generator) Run(targets []string) error {
	targetSet := map[string]bool{}
	for _, t := range targets {
		targetSet[t] = true
	}
	all := len(targets) == 0

	if all || targetSet["sql"] {
		if err := g.GenerateSQL(); err != nil {
			return fmt.Errorf("sql generation: %w", err)
		}
	}
	if all || targetSet["go"] {
		if err := g.GenerateGo(); err != nil {
			return fmt.Errorf("go generation: %w", err)
		}
	}
	if all || targetSet["proto"] {
		if err := g.GenerateProto(); err != nil {
			return fmt.Errorf("proto generation: %w", err)
		}
	}
	if all || targetSet["docker"] {
		if err := g.GenerateDockerfiles(); err != nil {
			return fmt.Errorf("docker generation: %w", err)
		}
	}
	if all || targetSet["k8s"] {
		if err := g.GenerateK8sManifests(); err != nil {
			return fmt.Errorf("k8s generation: %w", err)
		}
	}
	if all || targetSet["agl"] {
		if err := g.GenerateAGL(); err != nil {
			return fmt.Errorf("agl generation: %w", err)
		}
	}
	if all || targetSet["tests"] {
		if err := g.GenerateTests(); err != nil {
			return fmt.Errorf("tests generation: %w", err)
		}
	}
	if all || targetSet["agl"] {
		if err := g.GenerateProjectFiles(); err != nil {
			return fmt.Errorf("project files generation: %w", err)
		}
	}
	return nil
}

// GenerateProjectFiles writes go.mod and Makefile for the generated project.
func (g *Generator) GenerateProjectFiles() error {
	gomod := fmt.Sprintf(`module %s

go 1.22.2

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.5.5
	github.com/Masterminds/squirrel v1.5.4
	google.golang.org/grpc v1.63.2
	google.golang.org/protobuf v1.34.1
	k8s.io/apimachinery v0.30.0
	k8s.io/apiserver v0.30.0
	k8s.io/component-base v0.30.0
	k8s.io/klog/v2 v2.120.1
)
`, g.Schema.Module)
	if err := g.writeGen("go.mod", gomod); err != nil {
		return err
	}

	data := struct {
		Module  string
		Project ir.ProjectConfig
	}{Module: g.Schema.Module, Project: g.Schema.Project}
	tmplStr, err := readFileBytes(filepath.Join(g.tmplDir, "project", "Makefile.tmpl"))
	if err != nil {
		return fmt.Errorf("load project Makefile template: %w", err)
	}
	out, err := g.execTemplate("project-makefile", string(tmplStr), data)
	if err != nil {
		return err
	}
	return g.writeGen("Makefile", out)
}

// writeGen writes a generated file (always overwrites).
func (g *Generator) writeGen(relPath, content string) error {
	p := filepath.Join(g.OutputDir, relPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// writeScaffold writes a scaffold file only if it does not already exist.
func (g *Generator) writeScaffold(relPath, content string) error {
	p := filepath.Join(g.OutputDir, relPath)
	if _, err := os.Stat(p); err == nil {
		return nil // file exists, skip
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// execTemplate parses and executes a template string with the given data.
func (g *Generator) execTemplate(name, tmplStr string, data any) (string, error) {
	t, err := template.New(name).Funcs(g.funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// Helper functions for templates.

func toCamelCase(s string) string {
	if !strings.Contains(s, "_") {
		// Already camelCase or single word — ensure first letter is lowercase.
		if s == "" {
			return s
		}
		runes := []rune(s)
		if runes[0] >= 'A' && runes[0] <= 'Z' {
			runes[0] = runes[0] + ('a' - 'A')
		}
		return string(runes)
	}
	parts := strings.Split(s, "_")
	var result strings.Builder
	result.WriteString(strings.ToLower(parts[0]))
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		result.WriteString(strings.Title(p)) //nolint:staticcheck
	}
	return result.String()
}

func toKebabCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(r + ('a' - 'A'))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + ('a' - 'A'))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func joinQuoted(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("'%s'", item)
	}
	return strings.Join(quoted, ", ")
}

func seq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i + 1
	}
	return s
}

func placeholders(count, offset int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", i+1+offset)
	}
	return strings.Join(parts, ", ")
}

// k8sResourcePlural returns the lowercase plural form for K8s resource names (no underscores).
func k8sResourcePlural(name string) string {
	s := strings.ToLower(name)
	if strings.HasSuffix(s, "s") {
		return s + "es"
	}
	return s + "s"
}

// k8sResourceSingular returns the lowercase singular form for K8s resource names.
func k8sResourceSingular(name string) string {
	return strings.ToLower(name)
}

// goFieldName converts a snake_case name to PascalCase Go field name.
func goFieldName(name string) string {
	if !strings.Contains(name, "_") {
		// camelCase → PascalCase: just uppercase first letter.
		if name == "" {
			return name
		}
		runes := []rune(name)
		if runes[0] >= 'a' && runes[0] <= 'z' {
			runes[0] = runes[0] - ('a' - 'A')
		}
		return string(runes)
	}
	parts := strings.Split(name, "_")
	var result strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		result.WriteString(strings.Title(p)) //nolint:staticcheck
	}
	return result.String()
}

func dict(pairs ...any) map[string]any {
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			continue
		}
		m[k] = pairs[i+1]
	}
	return m
}
