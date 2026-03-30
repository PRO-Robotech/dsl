package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
	"gopkg.in/yaml.v3"
)

// ── YAML structs ────────────────────────────────────────────────

type yamlSchema struct {
	Version      string                     `yaml:"version"`
	Project      yamlProject                `yaml:"project"`
	Restrictions map[string]yamlRestriction `yaml:"restrictions"`
	Types        map[string]yamlType        `yaml:"types"`
	Resources    []yamlResource             `yaml:"resources"`
	Roles        map[string]yamlRole        `yaml:"roles"`
}

type yamlProject struct {
	Name         string `yaml:"name"`
	Module       string `yaml:"module"`
	APIGroup     string `yaml:"apiGroup"`
	APIVersion   string `yaml:"apiVersion"`
	ProtoPackage string `yaml:"protoPackage"`
	DBSchema     string `yaml:"dbSchema"`
}

type yamlRestriction struct {
	Pattern     string `yaml:"pattern"`
	MaxLength   int    `yaml:"maxLength"`
	Description string `yaml:"description"`
}

type yamlType struct {
	Kind          string              `yaml:"kind"`
	Values        []string            `yaml:"values"`
	Fields        []yamlTypeField     `yaml:"fields"`
	OneOfBy       string              `yaml:"oneofBy"`
	OneOfVariants map[string][]string `yaml:"oneofVariants"`
	BaseType      string              `yaml:"baseType"`
	Constraints   []string            `yaml:"constraints"`
	Mapping       yamlTypeMapping     `yaml:"mapping"`
	K8sName       string              `yaml:"k8sName"`
	ProtoName     string              `yaml:"protoName"`
	ProtoMessages []yamlProtoMessage  `yaml:"protoMessages"`
	ProtoEnums    []yamlProtoEnum     `yaml:"protoEnums"`
	GoFields      []yamlGoField       `yaml:"goFields"`
	K8sFields     []yamlGoField       `yaml:"k8sFields"`
}

type yamlGoField struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`
	JSONName  string `yaml:"jsonName"`
	ProtoName string `yaml:"protoName"`
	OmitEmpty bool   `yaml:"omitEmpty"`
}

type yamlTypeMapping struct {
	SQL   string `yaml:"sql"`
	Go    string `yaml:"go"`
	Proto string `yaml:"proto"`
}

type yamlTypeField struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	OneOfGroup string `yaml:"oneofGroup"`
}

type yamlProtoMessage struct {
	Name      string           `yaml:"name"`
	Fields    []yamlProtoField `yaml:"fields"`
	OneOfName string           `yaml:"oneofName"`
}

type yamlProtoField struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Number     int    `yaml:"number"`
	Repeated   bool   `yaml:"repeated"`
	JSONName   string `yaml:"jsonName"`
	OneOfGroup string `yaml:"oneofGroup"`
}

type yamlProtoEnum struct {
	Name   string               `yaml:"name"`
	Values []yamlProtoEnumValue `yaml:"values"`
}

type yamlProtoEnumValue struct {
	Name   string `yaml:"name"`
	Number int    `yaml:"number"`
}

type yamlResource struct {
	Name             string               `yaml:"name"`
	Scope            string               `yaml:"scope"`
	Kind             string               `yaml:"kind"`
	Table            string               `yaml:"table"`
	IndexPrefix      string               `yaml:"indexPrefix"`
	HTTPPath         string               `yaml:"httpPath"`
	Spec             *yamlSpec            `yaml:"spec"`
	Immutable        []string             `yaml:"immutable"`
	HasBindingRev    bool                 `yaml:"hasBindingRev"`
	CrossNamespace   bool                 `yaml:"crossNamespace"`
	Constraints      []yamlConstraint     `yaml:"constraints"`
	Triggers         []yamlTrigger        `yaml:"triggers"`
	Refs             []yamlRef            `yaml:"refs"`
	CascadeRev       []yamlCascadeRev     `yaml:"cascadeRev"`
	List             yamlList             `yaml:"list"`
	Events           yamlEvents           `yaml:"events"`
	ExtraSyncers     []yamlExtraSyncer    `yaml:"extraSyncers"`
	ExtraGRPCMethods []yamlExtraGRPC      `yaml:"extraGrpcMethods"`
	K8sSubresources  []yamlK8sSubresource `yaml:"k8sSubresources"`
	Subtypes         []yamlSubtype        `yaml:"subtypes"`
}

type yamlSpec struct {
	Fields []yamlSpecField `yaml:"fields"`
}

type yamlSpecField struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type"`
	SQLColumn    string `yaml:"sqlColumn"`
	JSONName     string `yaml:"jsonName"`
	Default      string `yaml:"default"`
	Validate     string `yaml:"validate"`
	Repeated     bool   `yaml:"repeated"`
	Selector     bool   `yaml:"selector"`
	OutputOnly   bool   `yaml:"outputOnly"`
	TestSQLValue string `yaml:"testSqlValue"`
}

type yamlConstraint struct {
	Name  string `yaml:"name"`
	Check string `yaml:"check"`
}

type yamlTrigger struct {
	Name     string `yaml:"name"`
	Timing   string `yaml:"timing"`
	Function string `yaml:"function"`
}

type yamlRef struct {
	Name       string `yaml:"name"`
	Target     string `yaml:"target"`
	SQLColumn  string `yaml:"sqlColumn"`
	SQLFKTable string `yaml:"sqlFkTable"`
	Selector   bool   `yaml:"selector"`
}

type yamlCascadeRev struct {
	ParentTable string `yaml:"parentTable"`
	RefColumn   string `yaml:"refColumn"`
}

type yamlList struct {
	Selectors string   `yaml:"selectors"`
	HasRefs   bool     `yaml:"hasRefs"`
	Parallel  bool     `yaml:"parallel"`
	RefTypes  []string `yaml:"refTypes"`
}

type yamlEvents struct {
	Channel  string `yaml:"channel"`
	Parallel bool   `yaml:"parallel"`
}

type yamlExtraSyncer struct {
	Name    string   `yaml:"name"`
	SQLFunc string   `yaml:"sqlFunc"`
	Fields  []string `yaml:"fields"`
}

type yamlExtraGRPC struct {
	Name       string `yaml:"name"`
	Scaffold   bool   `yaml:"scaffold"`
	HTTPPath   string `yaml:"httpPath"`
	HTTPVerb   string `yaml:"httpVerb"`
	SyncerName string `yaml:"syncerName"`
}

type yamlK8sSubresource struct {
	Name       string `yaml:"name"`
	SpecField  string `yaml:"specField"`
	GRPCMethod string `yaml:"grpcMethod"`
}

type yamlSubtype struct {
	Name  string `yaml:"name"`
	Table string `yaml:"table"`
}

type yamlRole struct {
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

// ── Built-in type mappings ──────────────────────────────────────

var builtinTypes = map[string]ir.TypeMapping{
	"bool":    {SQL: "bool", Go: "bool", Proto: "bool"},
	"string":  {SQL: "text", Go: "string", Proto: "string"},
	"text":    {SQL: "text", Go: "string", Proto: "string"},
	"int":     {SQL: "integer", Go: "int", Proto: "int32"},
	"int32":   {SQL: "integer", Go: "int32", Proto: "int32"},
	"int64":   {SQL: "bigint", Go: "int64", Proto: "int64"},
	"float64": {SQL: "double precision", Go: "float64", Proto: "double"},
}

// ── Helpers ─────────────────────────────────────────────────────

func goFieldName(name string) string {
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

// toPascalCase converts camelCase to PascalCase.
func toPascalCase(s string) string {
	if s == "" {
		return s
	}
	// If already starts with uppercase, return as-is.
	if s[0] >= 'A' && s[0] <= 'Z' {
		return s
	}
	runes := []rune(s)
	runes[0] = runes[0] - ('a' - 'A')
	return string(runes)
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

// ── Parse API ───────────────────────────────────────────────────

func ParseFile(path string) (*ir.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*ir.Schema, error) {
	var raw yamlSchema
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	return convert(raw)
}

// ── Conversion ──────────────────────────────────────────────────

func convert(raw yamlSchema) (*ir.Schema, error) {
	proj := ir.ProjectConfig{
		Name:         raw.Project.Name,
		Module:       raw.Project.Module,
		APIGroup:     raw.Project.APIGroup,
		APIVersion:   raw.Project.APIVersion,
		ProtoPackage: raw.Project.ProtoPackage,
		DBSchema:     raw.Project.DBSchema,
	}

	s := &ir.Schema{
		Version:      raw.Version,
		Module:       proj.Module,
		DBSchema:     proj.DBSchema,
		Project:      proj,
		Restrictions: make(map[string]ir.Restriction),
	}

	for name, r := range raw.Restrictions {
		s.Restrictions[name] = ir.Restriction{
			Pattern:     r.Pattern,
			MaxLength:   r.MaxLength,
			Description: r.Description,
		}
	}

	// Build types registry (name -> mapping) for resolving spec field types.
	typeRegistry := make(map[string]ir.TypeMapping)
	enumValues := make(map[string][]string)

	for name, t := range raw.Types {
		k8sName := t.K8sName
		if k8sName == "" {
			k8sName = toPascalCase(name)
		}
		protoName := t.ProtoName
		if protoName == "" {
			if t.Mapping.Proto != "" && t.Mapping.Proto != "string" {
				protoName = t.Mapping.Proto
			} else {
				protoName = toPascalCase(name)
			}
		}

		mapping := ir.TypeMapping{
			SQL:   t.Mapping.SQL,
			Go:    t.Mapping.Go,
			Proto: t.Mapping.Proto,
		}

		// Derive defaults for missing mappings.
		if mapping.Go == "" {
			mapping.Go = toPascalCase(name)
		}
		if mapping.SQL == "" {
			if t.Kind == "enum" && raw.Project.DBSchema != "" {
				mapping.SQL = raw.Project.DBSchema + "." + toSnakeCase(name)
			} else {
				mapping.SQL = toSnakeCase(name)
			}
		}
		if mapping.Proto == "" {
			mapping.Proto = mapping.Go
		}

		ct := ir.CustomType{
			Name:          name,
			Kind:          t.Kind,
			Values:        t.Values,
			OneOfBy:       t.OneOfBy,
			OneOfVariants: t.OneOfVariants,
			BaseType:      t.BaseType,
			Constraints:   t.Constraints,
			Mapping:       mapping,
			K8sName:       k8sName,
			ProtoName:     protoName,
		}
		for _, f := range t.Fields {
			ct.Fields = append(ct.Fields, ir.TypeField{
				Name:       f.Name,
				Type:       f.Type,
				OneOfGroup: f.OneOfGroup,
			})
		}
		for _, pm := range t.ProtoMessages {
			msg := ir.ProtoMessageDef{
				Name:      pm.Name,
				OneOfName: pm.OneOfName,
			}
			for _, pf := range pm.Fields {
				msg.Fields = append(msg.Fields, ir.ProtoFieldDef{
					Name:       pf.Name,
					Type:       pf.Type,
					Number:     pf.Number,
					Repeated:   pf.Repeated,
					JSONName:   pf.JSONName,
					OneOfGroup: pf.OneOfGroup,
				})
			}
			ct.ProtoMessages = append(ct.ProtoMessages, msg)
		}
		for _, pe := range t.ProtoEnums {
			enumDef := ir.ProtoEnumDef{Name: pe.Name}
			for _, v := range pe.Values {
				enumDef.Values = append(enumDef.Values, ir.ProtoEnumValue{
					Name:   v.Name,
					Number: v.Number,
				})
			}
			ct.ProtoEnums = append(ct.ProtoEnums, enumDef)
		}
		for _, gf := range t.GoFields {
			ct.GoFields = append(ct.GoFields, ir.GoFieldDef{
				Name:      gf.Name,
				Type:      gf.Type,
				JSONName:  gf.JSONName,
				ProtoName: gf.ProtoName,
				OmitEmpty: gf.OmitEmpty,
			})
		}
		for _, kf := range t.K8sFields {
			ct.K8sFields = append(ct.K8sFields, ir.GoFieldDef{
				Name:      kf.Name,
				Type:      kf.Type,
				JSONName:  kf.JSONName,
				ProtoName: kf.ProtoName,
				OmitEmpty: kf.OmitEmpty,
			})
		}
		s.Types = append(s.Types, ct)

		typeRegistry[name] = mapping
		if t.Kind == "enum" {
			enumValues[name] = t.Values
		}
	}

	// Merge built-in types into registry (don't override user types).
	for name, m := range builtinTypes {
		if _, exists := typeRegistry[name]; !exists {
			typeRegistry[name] = m
		}
	}

	// Build index of custom types by name for resolving spec fields.
	customTypeIndex := make(map[string]*ir.CustomType, len(s.Types))
	for i := range s.Types {
		customTypeIndex[s.Types[i].Name] = &s.Types[i]
	}

	for _, r := range raw.Resources {
		res := ir.Resource{
			Name:           r.Name,
			Scope:          r.Scope,
			Kind:           r.Kind,
			Table:          r.Table,
			IndexPrefix:    r.IndexPrefix,
			HTTPPath:       r.HTTPPath,
			Immutable:      r.Immutable,
			HasBindingRev:  r.HasBindingRev,
			CrossNamespace: r.CrossNamespace,
			List: ir.ListConfig{
				Selectors: r.List.Selectors,
				HasRefs:   r.List.HasRefs,
				Parallel:  r.List.Parallel,
				RefTypes:  r.List.RefTypes,
			},
			Events: ir.EventConfig{
				Channel:  r.Events.Channel,
				Parallel: r.Events.Parallel,
			},
		}

		if r.Spec != nil {
			for _, f := range r.Spec.Fields {
				sf := ir.SpecField{
					Name:         f.Name,
					Type:         f.Type,
					SQLColumn:    f.SQLColumn,
					JSONName:     f.JSONName,
					Default:      f.Default,
					Validate:     f.Validate,
					Repeated:     f.Repeated,
					Selector:     f.Selector,
					OutputOnly:   f.OutputOnly,
					TestSQLValue: f.TestSQLValue,
				}

				// Resolve type mapping.
				if m, ok := typeRegistry[f.Type]; ok {
					sf.SQLType = m.SQL
					sf.GoType = m.Go
					sf.ProtoType = m.Proto
				} else {
					sf.SQLType = f.Type
					sf.GoType = f.Type
					sf.ProtoType = f.Type
				}

				// Populate enum values.
				if vals, ok := enumValues[f.Type]; ok {
					sf.EnumValues = vals
				}

				// Derive ProtoField from JSONName if set.
				if f.JSONName != "" {
					sf.ProtoField = f.JSONName
				}

				// Resolve ConversionKind and ResolvedType.
				if ct, ok := customTypeIndex[f.Type]; ok {
					sf.ResolvedType = ct
					switch {
					case ct.Kind == "enum":
						sf.ConversionKind = ir.ConvStringCast
					case sf.SQLType == "cidr":
						sf.ConversionKind = ir.ConvCIDR
					case sf.SQLType == "jsonb":
						sf.ConversionKind = ir.ConvJSONBStruct
					default:
						sf.ConversionKind = ir.ConvStringCast
					}
				} else {
					sf.ConversionKind = ir.ConvPassthrough
				}

				res.Spec.Fields = append(res.Spec.Fields, sf)
			}
		}

		for _, c := range r.Constraints {
			res.Constraints = append(res.Constraints, ir.Constraint{Name: c.Name, Check: c.Check})
		}
		for _, t := range r.Triggers {
			res.Triggers = append(res.Triggers, ir.TriggerDef{Name: t.Name, Timing: t.Timing, Function: t.Function})
		}
		for _, ref := range r.Refs {
			res.Refs = append(res.Refs, ir.RefDef{
				Name: ref.Name, Target: ref.Target,
				SQLColumn: ref.SQLColumn, SQLFKTable: ref.SQLFKTable,
				Selector: ref.Selector,
			})
		}
		for _, cr := range r.CascadeRev {
			res.CascadeRev = append(res.CascadeRev, ir.CascadeRevDef{
				ParentTable: cr.ParentTable, RefColumn: cr.RefColumn,
			})
		}
		for _, es := range r.ExtraSyncers {
			res.ExtraSyncers = append(res.ExtraSyncers, ir.ExtraSyncer{
				Name: es.Name, SQLFunc: es.SQLFunc, Fields: es.Fields,
			})
		}
		for _, eg := range r.ExtraGRPCMethods {
			res.ExtraGRPCMethods = append(res.ExtraGRPCMethods, ir.ExtraGRPCMethod{
				Name: eg.Name, Scaffold: eg.Scaffold,
				HTTPPath: eg.HTTPPath, HTTPVerb: eg.HTTPVerb,
				SyncerName: eg.SyncerName,
			})
		}
		for _, ks := range r.K8sSubresources {
			sub := ir.K8sSubresource{
				Name:       ks.Name,
				SpecField:  ks.SpecField,
				GRPCMethod: ks.GRPCMethod,
			}
			// Resolve GoType and K8sType from the linked spec field.
			for _, sf := range res.Spec.Fields {
				if sf.Name == ks.SpecField {
					sub.GoType = sf.GoType
					if m, ok := typeRegistry[sf.Type]; ok {
						sub.K8sType = m.Go
					} else {
						sub.K8sType = sf.GoType
					}
					// Check if there's a custom K8s name for this type.
					for _, ct := range s.Types {
						if ct.Name == sf.Type {
							sub.K8sType = ct.K8sName
							break
						}
					}
					break
				}
			}
			res.K8sSubresources = append(res.K8sSubresources, sub)
		}
		for _, st := range r.Subtypes {
			res.Subtypes = append(res.Subtypes, ir.Subtype{Name: st.Name, Table: st.Table})
		}

		s.Resources = append(s.Resources, res)
	}

	for name, role := range raw.Roles {
		s.Roles = append(s.Roles, ir.Role{
			Name: name, Resources: role.Resources, Verbs: role.Verbs,
		})
	}

	return s, nil
}
