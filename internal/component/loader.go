package component

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type componentFile struct {
	Components []Component `yaml:"components"`
}

// LoadDir reads all .yaml files from dir and populates a Registry.
func LoadDir(dir string) (*Registry, error) {
	reg := NewRegistry()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read components dir %s: %w", dir, err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read component file %s: %w", e.Name(), err)
		}
		var cf componentFile
		if err := yaml.Unmarshal(data, &cf); err != nil {
			return nil, fmt.Errorf("parse component file %s: %w", e.Name(), err)
		}
		for i := range cf.Components {
			c := &cf.Components[i]
			if c.Name != "" {
				reg.ByGoType[c.Name] = c
			}
			if c.Kind != "" {
				if _, exists := reg.ByKind[c.Kind]; !exists {
					reg.ByKind[c.Kind] = c
				}
			}
		}
	}

	return reg, nil
}

// Resolve finds the best Component for a given goType and typeKind.
// Priority: exact goType match > kind match > nil.
func (r *Registry) Resolve(goType, typeKind string) *Component {
	if c, ok := r.ByGoType[goType]; ok {
		return c
	}
	if c, ok := r.ByKind[typeKind]; ok {
		return c
	}
	return nil
}

// FieldContext is passed into component template expressions for evaluation.
type FieldContext struct {
	GoType     string
	Name       string
	EnumValues []string
}

// ResolveField finds the matching Component and evaluates its template
// expressions with field-specific context, returning a Resolved struct.
// pkg is the domain package alias for qualified expressions.
func (r *Registry) ResolveField(goType, typeKind string, ctx FieldContext, pkg string) *Resolved {
	c := r.Resolve(goType, typeKind)
	if c == nil {
		return nil
	}

	unqualified := &FieldContext{GoType: ctx.GoType, Name: ctx.Name, EnumValues: ctx.EnumValues}
	qualified := &FieldContext{GoType: QualifyGoType(ctx.GoType, pkg), Name: ctx.Name, EnumValues: ctx.EnumValues}

	isPrimitive := IsPrimitive(ctx.GoType)
	if isPrimitive {
		qualified = unqualified
	}

	return &Resolved{
		Kind:         c.Kind,
		GoImport:     c.GoImport,
		ValidExpr:    evalExpr(c.Test.ValidExpr, *unqualified),
		ZeroExpr:     evalExpr(c.Test.ZeroExpr, *unqualified),
		InvalidExpr:  evalExpr(c.Test.InvalidExpr, *unqualified),
		QValidExpr:   evalExpr(c.Test.ValidExpr, *qualified),
		QZeroExpr:    evalExpr(c.Test.ZeroExpr, *qualified),
		QInvalidExpr: evalExpr(c.Test.InvalidExpr, *qualified),
	}
}

// IsPrimitive returns true for Go built-in types that don't need package qualification.
func IsPrimitive(t string) bool {
	switch t {
	case "bool", "string", "int", "int64", "float64", "int32", "uint", "uint64", "byte", "rune":
		return true
	}
	return false
}

// EvalExprPublic evaluates a template expression with field context (exported for parser use).
func EvalExprPublic(expr string, ctx FieldContext) string {
	return evalExpr(expr, ctx)
}

func evalExpr(expr string, ctx FieldContext) string {
	if expr == "" {
		return ""
	}
	if !strings.Contains(expr, "{{") {
		return expr
	}
	fm := template.FuncMap{
		"goField":    goFieldName,
		"goBaseType": goBaseType,
	}
	t, err := template.New("expr").Funcs(fm).Parse(expr)
	if err != nil {
		return expr
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return expr
	}
	return buf.String()
}

func goFieldName(name string) string {
	parts := strings.Split(name, "_")
	var result strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		result.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return result.String()
}

func goBaseType(t string) string {
	t = strings.TrimPrefix(t, "[]")
	t = strings.TrimPrefix(t, "*")
	return t
}

// QualifyGoType prepends the package qualifier to a Go type, handling
// slice/pointer prefixes: "[]Foo" → "[]pkg.Foo", "*Foo" → "*pkg.Foo".
func QualifyGoType(goType, pkg string) string {
	prefix := ""
	base := goType
	for strings.HasPrefix(base, "[]") || strings.HasPrefix(base, "*") {
		if strings.HasPrefix(base, "[]") {
			prefix += "[]"
			base = base[2:]
		} else {
			prefix += "*"
			base = base[1:]
		}
	}
	return prefix + pkg + "." + base
}
