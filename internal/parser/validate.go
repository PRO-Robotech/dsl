package parser

import (
	"fmt"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

// Validate checks the parsed IR for semantic errors.
func Validate(s *ir.Schema) []error {
	var errs []error
	add := func(msg string, args ...any) {
		errs = append(errs, fmt.Errorf(msg, args...))
	}

	if s.Version == "" {
		add("schema: version is required")
	}
	if s.Module == "" {
		add("schema: module is required")
	}
	if s.DBSchema == "" {
		add("schema: schema (database schema name) is required")
	}

	resNames := map[string]bool{}
	tableNames := map[string]bool{}

	for i, r := range s.Resources {
		prefix := fmt.Sprintf("resources[%d] (%s)", i, r.Name)

		if r.Name == "" {
			add("%s: name is required", prefix)
		}
		if resNames[r.Name] {
			add("%s: duplicate resource name", prefix)
		}
		resNames[r.Name] = true

		if r.Table == "" {
			add("%s: table is required", prefix)
		}
		if tableNames[r.Table] {
			add("%s: duplicate table name %q", prefix, r.Table)
		}
		tableNames[r.Table] = true

		if r.Scope == "" {
			add("%s: scope is required (cluster or namespaced)", prefix)
		} else if r.Scope != "cluster" && r.Scope != "namespaced" {
			add("%s: scope must be 'cluster' or 'namespaced', got %q", prefix, r.Scope)
		}

		if r.Kind != "" && r.Kind != "resource" && r.Kind != "binding" && r.Kind != "composite" {
			add("%s: kind must be 'resource', 'binding' or 'composite', got %q", prefix, r.Kind)
		}

		if r.IsBinding() {
			if len(r.Refs) == 0 {
				add("%s: binding resource must have at least one ref", prefix)
			}
			for j, ref := range r.Refs {
				if ref.Target == "" {
					add("%s: refs[%d]: target is required", prefix, j)
				}
				if ref.SQLColumn == "" {
					add("%s: refs[%d]: sql_column is required", prefix, j)
				}
			}
		}

		if r.IsComposite() {
			if len(r.Subtypes) == 0 {
				add("%s: composite resource must have at least one subtype", prefix)
			}
		}

		if len(r.Immutable) == 0 {
			add("%s: immutable fields list is required (at least [uid, name])", prefix)
		}

		if r.Events.Channel == "" && !r.IsComposite() {
			add("%s: events.channel is required", prefix)
		}

		for j, f := range r.Spec.Fields {
			fp := fmt.Sprintf("%s: spec.fields[%d] (%s)", prefix, j, f.Name)
			if f.Name == "" {
				add("%s: name is required", fp)
			}
			if f.SQLType == "" {
				add("%s: sql_type is required", fp)
			}
			if f.GoType == "" {
				add("%s: go_type is required", fp)
			}
		}
	}

	// Validate ref targets exist
	for _, r := range s.Resources {
		for _, ref := range r.Refs {
			if !resNames[ref.Target] {
				add("resources (%s): ref target %q not found in resources", r.Name, ref.Target)
			}
		}
	}

	// Validate roles reference existing resources
	for _, role := range s.Roles {
		for _, res := range role.Resources {
			if res != "*" && !resNames[res] {
				add("roles (%s): resource %q not found", role.Name, res)
			}
		}
		for _, v := range role.Verbs {
			valid := map[string]bool{"create": true, "read": true, "update": true, "delete": true, "watch": true}
			if !valid[strings.ToLower(v)] {
				add("roles (%s): unknown verb %q", role.Name, v)
			}
		}
	}

	return errs
}
