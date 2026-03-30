package parser

import (
	"fmt"
	"os"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
	"gopkg.in/yaml.v3"
)

// yamlSchema mirrors the YAML DSL structure for unmarshalling.
type yamlSchema struct {
	Version      string                      `yaml:"version"`
	Module       string                      `yaml:"module"`
	Schema       string                      `yaml:"schema"`
	Restrictions map[string]yamlRestriction  `yaml:"restrictions"`
	Types        map[string]yamlType         `yaml:"types"`
	Resources    []yamlResource              `yaml:"resources"`
	Roles        map[string]yamlRole         `yaml:"roles"`
}

type yamlRestriction struct {
	Pattern     string `yaml:"pattern"`
	MaxLength   int    `yaml:"max_length"`
	Description string `yaml:"description"`
}

type yamlType struct {
	Kind           string              `yaml:"kind"`
	Values         []string            `yaml:"values"`
	Fields         []yamlTypeField     `yaml:"fields"`
	OneOfBy        string              `yaml:"oneof_by"`
	OneOfVariants  map[string][]string `yaml:"oneof_variants"`
	BaseType       string              `yaml:"base_type"`
	Constraints    []string            `yaml:"constraints"`
}

type yamlTypeField struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	OneOfGroup string `yaml:"oneof_group"`
}

type yamlResource struct {
	Name             string             `yaml:"name"`
	Scope            string             `yaml:"scope"`
	Kind             string             `yaml:"kind"`
	Table            string             `yaml:"table"`
	IndexPrefix      string             `yaml:"index_prefix"`
	HTTPPath         string             `yaml:"http_path"`
	Spec             *yamlSpec          `yaml:"spec"`
	Immutable        []string           `yaml:"immutable"`
	HasBindingRev    bool               `yaml:"has_binding_rev"`
	Constraints      []yamlConstraint   `yaml:"constraints"`
	Triggers         []yamlTrigger      `yaml:"triggers"`
	Refs             []yamlRef          `yaml:"refs"`
	CascadeRev       []yamlCascadeRev   `yaml:"cascade_rev"`
	List             yamlList           `yaml:"list"`
	Events           yamlEvents         `yaml:"events"`
	ExtraSyncers     []yamlExtraSyncer  `yaml:"extra_syncers"`
	ExtraGRPCMethods  []yamlExtraGRPC      `yaml:"extra_grpc_methods"`
	K8sSubresources   []yamlK8sSubresource `yaml:"k8s_subresources"`
	Subtypes          []yamlSubtype        `yaml:"subtypes"`
}

type yamlSpec struct {
	Fields []yamlSpecField `yaml:"fields"`
}

type yamlSpecField struct {
	Name       string `yaml:"name"`
	SQLType    string `yaml:"sql_type"`
	SQLColumn  string `yaml:"sql_column"`
	GoType     string `yaml:"go_type"`
	ProtoType  string `yaml:"proto_type"`
	ProtoField string `yaml:"proto_field"`
	Default    string `yaml:"default"`
	Validate   string `yaml:"validate"`
	OutputOnly bool   `yaml:"output_only"`
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
	SQLColumn  string `yaml:"sql_column"`
	SQLFKTable string `yaml:"sql_fk_table"`
}

type yamlCascadeRev struct {
	ParentTable string `yaml:"parent_table"`
	RefColumn   string `yaml:"ref_column"`
}

type yamlList struct {
	Selectors string   `yaml:"selectors"`
	HasRefs   bool     `yaml:"has_refs"`
	Parallel  bool     `yaml:"parallel"`
	RefTypes  []string `yaml:"ref_types"`
}

type yamlEvents struct {
	Channel  string `yaml:"channel"`
	Parallel bool   `yaml:"parallel"`
}

type yamlExtraSyncer struct {
	Name    string   `yaml:"name"`
	SQLFunc string   `yaml:"sql_func"`
	Fields  []string `yaml:"fields"`
}

type yamlExtraGRPC struct {
	Name     string `yaml:"name"`
	Scaffold bool   `yaml:"scaffold"`
	HTTPPath string `yaml:"http_path"`
	HTTPVerb string `yaml:"http_verb"`
}

type yamlK8sSubresource struct {
	Name       string `yaml:"name"`
	Field      string `yaml:"field"`
	GoType     string `yaml:"go_type"`
	K8sType    string `yaml:"k8s_type"`
	GRPCMethod string `yaml:"grpc_method"`
}

type yamlSubtype struct {
	Name  string `yaml:"name"`
	Table string `yaml:"table"`
}

type yamlRole struct {
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

// ParseFile reads a YAML DSL file and returns the IR Schema.
func ParseFile(path string) (*ir.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema file: %w", err)
	}
	return Parse(data)
}

// Parse unmarshals YAML bytes into an IR Schema.
func Parse(data []byte) (*ir.Schema, error) {
	var raw yamlSchema
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	return convert(raw)
}

func convert(raw yamlSchema) (*ir.Schema, error) {
	s := &ir.Schema{
		Version:      raw.Version,
		Module:       raw.Module,
		DBSchema:     raw.Schema,
		Restrictions: make(map[string]ir.Restriction),
	}

	for name, r := range raw.Restrictions {
		s.Restrictions[name] = ir.Restriction{
			Pattern:     r.Pattern,
			MaxLength:   r.MaxLength,
			Description: r.Description,
		}
	}

	for name, t := range raw.Types {
		ct := ir.CustomType{
			Name:          name,
			Kind:          t.Kind,
			Values:        t.Values,
			OneOfBy:       t.OneOfBy,
			OneOfVariants: t.OneOfVariants,
			BaseType:      t.BaseType,
			Constraints:   t.Constraints,
		}
		for _, f := range t.Fields {
			ct.Fields = append(ct.Fields, ir.TypeField{
				Name:       f.Name,
				Type:       f.Type,
				OneOfGroup: f.OneOfGroup,
			})
		}
		s.Types = append(s.Types, ct)
	}

	for _, r := range raw.Resources {
		res := ir.Resource{
			Name:          r.Name,
			Scope:         r.Scope,
			Kind:          r.Kind,
			Table:         r.Table,
			IndexPrefix:   r.IndexPrefix,
			HTTPPath:      r.HTTPPath,
			Immutable:     r.Immutable,
			HasBindingRev: r.HasBindingRev,
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
				res.Spec.Fields = append(res.Spec.Fields, ir.SpecField{
					Name:       f.Name,
					SQLType:    f.SQLType,
					SQLColumn:  f.SQLColumn,
					GoType:     f.GoType,
					ProtoType:  f.ProtoType,
					ProtoField: f.ProtoField,
					Default:    f.Default,
					Validate:   f.Validate,
					OutputOnly: f.OutputOnly,
				})
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
			})
		}
		for _, ks := range r.K8sSubresources {
			res.K8sSubresources = append(res.K8sSubresources, ir.K8sSubresource{
				Name: ks.Name, Field: ks.Field,
				GoType: ks.GoType, K8sType: ks.K8sType,
				GRPCMethod: ks.GRPCMethod,
			})
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
