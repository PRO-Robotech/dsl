package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

// gooseWrap wraps raw SQL content with goose migration annotations.
// StatementBegin/StatementEnd is required for any SQL containing $$ (PL/pgSQL blocks).
func gooseWrap(sql string) string {
	return "-- +goose Up\n-- +goose StatementBegin\n" + sql + "\n-- +goose StatementEnd\n\n-- +goose Down\n-- (manual rollback)\n"
}

// cleanGenMigrations removes all *_gen.sql files from the migrations directory
// to prevent stale files from previous generations with different numbering.
func (g *Generator) cleanGenMigrations() error {
	dir := filepath.Join(g.OutputDir, "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_gen.sql") {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// GenerateSQL produces all SQL migration files with goose-compatible numbered prefixes.
func (g *Generator) GenerateSQL() error {
	if err := g.cleanGenMigrations(); err != nil {
		return err
	}
	if err := g.generateSQLSchema(); err != nil {
		return err
	}
	if err := g.generateSQLTypes(); err != nil {
		return err
	}
	if err := g.generateSQLTables(); err != nil {
		return err
	}
	if err := g.generateSQLViews(); err != nil {
		return err
	}
	if err := g.generateSQLListers(); err != nil {
		return err
	}
	if err := g.generateSQLSyncers(); err != nil {
		return err
	}
	if err := g.generateSQLRBAC(); err != nil {
		return err
	}
	return nil
}

func (g *Generator) loadSQLTemplate(name string) (string, error) {
	p := filepath.Join(g.tmplDir, "sql", name)
	data, err := readFileBytes(p)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}
	return string(data), nil
}

func (g *Generator) generateSQLSchema() error {
	sql := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %[1]s;

CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS hstore;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

SET LOCAL search_path TO %[1]s, public;

-- ── Domains ────────────────────────────────────────────────────

DO $$ BEGIN
  CREATE DOMAIN %[1]s.rname AS text
    CHECK (VALUE ~ '^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE DOMAIN %[1]s.dname AS text
    CHECK (char_length(VALUE) <= 255);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE DOMAIN %[1]s.port_ranges AS int4multirange
    CHECK (VALUE <@ int4multirange(int4range(1, 65536)));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE DOMAIN %[1]s.icmp_types AS int2[];
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE %[1]s.resource_id AS (name text, namespace text);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE %[1]s.resource_ref AS (name text, namespace text, res_type text);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE %[1]s.host_info AS (host_name text, os text, platform text);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE DOMAIN %[1]s.resource_op AS text
    CHECK (VALUE IN ('ADDED', 'MODIFIED', 'DELETED'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE DOMAIN %[1]s.sync_op AS text
    CHECK (VALUE IN ('ups', 'del'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE %[1]s.field_selector AS (name text, namespace text, refs text[]);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE %[1]s.res_selector AS (field %[1]s.field_selector, label_selector hstore);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE OR REPLACE FUNCTION %[1]s.match_res_selectors(
    _name %[1]s.rname,
    _namespace %[1]s.rname,
    _labels hstore,
    _selectors %[1]s.res_selector[]
) RETURNS boolean LANGUAGE sql IMMUTABLE AS $fn$
  SELECT EXISTS (
    SELECT 1 FROM unnest(_selectors) s
    WHERE (((s.field).name IS NULL) OR ((s.field).name = _name))
      AND (((s.field).namespace IS NULL) OR ((s.field).namespace = _namespace))
      AND (_labels @> COALESCE(s.label_selector, ''::hstore))
  );
$fn$;

-- ── Outbox table ───────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS %[1]s.tbl_outbox_resource_events (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    resource_type   text NOT NULL,
    resource_uid    uuid NOT NULL,
    operation       %[1]s.resource_op NOT NULL,
    payload         jsonb NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- ── Helper functions for triggers ──────────────────────────────

CREATE OR REPLACE FUNCTION %[1]s.forbid_fields_update_trg()
RETURNS trigger LANGUAGE plpgsql AS $fn$
DECLARE
  col text;
BEGIN
  FOREACH col IN ARRAY TG_ARGV LOOP
    IF to_jsonb(NEW)->>col IS DISTINCT FROM to_jsonb(OLD)->>col THEN
      RAISE EXCEPTION 'field "%%" is immutable', col;
    END IF;
  END LOOP;
  RETURN NEW;
END;
$fn$;

CREATE SEQUENCE IF NOT EXISTS %[1]s.global_resource_version START 1;

CREATE OR REPLACE FUNCTION %[1]s.inc_resource_version_cnt()
RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
  NEW.resource_version := nextval('%[1]s.global_resource_version');
  RETURN NEW;
END;
$fn$;

CREATE OR REPLACE FUNCTION %[1]s.capture_outbox_resource_events_trg()
RETURNS trigger LANGUAGE plpgsql AS $fn$
DECLARE
  res_type text := TG_ARGV[0];
  view_name text := TG_ARGV[1];
  uid_col text := TG_ARGV[2];
  op %[1]s.resource_op;
  uid_val uuid;
  payload jsonb;
BEGIN
  IF TG_OP = 'DELETE' THEN
    op := 'DELETED';
    uid_val := to_jsonb(OLD)->>uid_col;
    payload := to_jsonb(OLD);
  ELSIF TG_OP = 'INSERT' THEN
    op := 'ADDED';
    uid_val := to_jsonb(NEW)->>uid_col;
    payload := to_jsonb(NEW);
  ELSE
    op := 'MODIFIED';
    uid_val := to_jsonb(NEW)->>uid_col;
    payload := to_jsonb(NEW);
  END IF;
  INSERT INTO %[1]s.tbl_outbox_resource_events (resource_type, resource_uid, operation, payload)
    VALUES (res_type, uid_val, op, payload);
  IF TG_OP = 'DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$fn$;

CREATE OR REPLACE FUNCTION %[1]s.notify_outbox_resource_events_trg()
RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
  PERFORM pg_notify(TG_ARGV[0], NEW.id::text);
  RETURN NEW;
END;
$fn$;

CREATE OR REPLACE FUNCTION %[1]s.upd_binding_refs_trg()
RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
  RETURN NEW;
END;
$fn$;
`, g.Schema.DBSchema)
	return g.writeGen("migrations/00001_schema_gen.sql", gooseWrap(sql))
}

type sqlTypesData struct {
	Schema     string
	Enums      []ir.CustomType
	Composites []ir.CustomType
}

// sortComposites performs a topological sort on composite types so that
// types referenced by other composites are defined first.
func sortComposites(types []ir.CustomType) []ir.CustomType {
	nameSet := make(map[string]bool, len(types))
	for _, t := range types {
		nameSet[t.Name] = true
	}

	deps := make(map[string][]string, len(types))
	for _, t := range types {
		for _, f := range t.Fields {
			base := strings.TrimSuffix(f.Type, "[]")
			if nameSet[base] && base != t.Name {
				deps[t.Name] = append(deps[t.Name], base)
			}
		}
	}

	visited := make(map[string]bool)
	var result []ir.CustomType
	byName := make(map[string]ir.CustomType, len(types))
	for _, t := range types {
		byName[t.Name] = t
	}

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		for _, dep := range deps[name] {
			visit(dep)
		}
		result = append(result, byName[name])
	}

	for _, t := range types {
		visit(t.Name)
	}
	return result
}

func (g *Generator) generateSQLTypes() error {
	tmpl, err := g.loadSQLTemplate("types.sql.tmpl")
	if err != nil {
		return err
	}

	var enums, composites []ir.CustomType
	for _, t := range g.Schema.Types {
		switch t.Kind {
		case "enum":
			enums = append(enums, t)
		case "composite":
			composites = append(composites, t)
		}
	}
	composites = sortComposites(composites)

	data := sqlTypesData{
		Schema:     g.Schema.DBSchema,
		Enums:      enums,
		Composites: composites,
	}

	out, err := g.execTemplate("types.sql", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("migrations/00002_types_gen.sql", gooseWrap(out))
}

type sqlResourceData struct {
	Schema         string
	Res            *ir.Resource
	NamespaceTable string
}

func (g *Generator) sqlData(res *ir.Resource) sqlResourceData {
	nsTable := "tbl_tenant"
	for _, r := range g.Schema.Resources {
		if r.IsCluster() {
			nsTable = "tbl_" + r.Table
			break
		}
	}
	return sqlResourceData{Schema: g.Schema.DBSchema, Res: res, NamespaceTable: nsTable}
}

func (g *Generator) generateSQLTables() error {
	tableTmpl, err := g.loadSQLTemplate("table.sql.tmpl")
	if err != nil {
		return err
	}
	triggerTmpl, err := g.loadSQLTemplate("triggers.sql.tmpl")
	if err != nil {
		return err
	}
	compTableTmpl, err := g.loadSQLTemplate("table_composite.sql.tmpl")
	if err != nil {
		return err
	}
	compTriggerTmpl, err := g.loadSQLTemplate("triggers_composite.sql.tmpl")
	if err != nil {
		return err
	}

	var tablesBuf, triggersBuf strings.Builder
	tablesBuf.WriteString("-- DO NOT EDIT: generated by sgctl\n")
	tablesBuf.WriteString(fmt.Sprintf("SET LOCAL search_path TO %s, public;\n\n", g.Schema.DBSchema))
	triggersBuf.WriteString("-- DO NOT EDIT: generated by sgctl\n")
	triggersBuf.WriteString(fmt.Sprintf("SET LOCAL search_path TO %s, public;\n\n", g.Schema.DBSchema))

	for i := range g.Schema.Resources {
		res := &g.Schema.Resources[i]
		data := g.sqlData(res)

		if res.IsComposite() {
			tableOut, err := g.execTemplate("table-composite-"+res.Name, compTableTmpl, data)
			if err != nil {
				return err
			}
			tablesBuf.WriteString(tableOut)
			tablesBuf.WriteString("\n")

			trigOut, err := g.execTemplate("triggers-composite-"+res.Name, compTriggerTmpl, data)
			if err != nil {
				return err
			}
			triggersBuf.WriteString(trigOut)
			triggersBuf.WriteString("\n")
		} else {
			tableOut, err := g.execTemplate("table-"+res.Name, tableTmpl, data)
			if err != nil {
				return err
			}
			tablesBuf.WriteString(tableOut)
			tablesBuf.WriteString("\n")

			trigOut, err := g.execTemplate("triggers-"+res.Name, triggerTmpl, data)
			if err != nil {
				return err
			}
			triggersBuf.WriteString(trigOut)
			triggersBuf.WriteString("\n")
		}
	}

	if err := g.writeGen("migrations/00003_tables_gen.sql", gooseWrap(tablesBuf.String())); err != nil {
		return err
	}
	return g.writeGen("migrations/00004_triggers_gen.sql", gooseWrap(triggersBuf.String()))
}

func (g *Generator) generateSQLViews() error {
	tmpl, err := g.loadSQLTemplate("view.sql.tmpl")
	if err != nil {
		return err
	}
	compTmpl, err := g.loadSQLTemplate("view_composite.sql.tmpl")
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("-- DO NOT EDIT: generated by sgctl\n")
	buf.WriteString(fmt.Sprintf("SET LOCAL search_path TO %s, public;\n\n", g.Schema.DBSchema))

	for i := range g.Schema.Resources {
		res := &g.Schema.Resources[i]
		if !res.List.HasRefs {
			continue
		}

		if res.IsBinding() {
			// Binding refs: build from own ref columns (direct references).
			var unions []string
			for _, ref := range res.Refs {
				q := fmt.Sprintf(
					"  SELECT jsonb_build_object(\n"+
						"    'name', b.%s,\n"+
						"    'namespace', COALESCE(b.namespace,''),\n"+
						"    'resType', '%s'\n"+
						"  )\n"+
						"  FROM %s.tbl_%s b WHERE b.uid = uid_val",
					ref.SQLColumn,
					ref.Target,
					g.Schema.DBSchema, res.Table)
				unions = append(unions, q)
			}
			body := strings.Join(unions, "\n  UNION ALL\n")
			buf.WriteString(fmt.Sprintf(
				"CREATE OR REPLACE FUNCTION %s.resolve_%s_refs(uid_val uuid)\n"+
					"RETURNS jsonb LANGUAGE sql STABLE AS $fn$\n"+
					"  SELECT COALESCE(jsonb_agg(r), '[]'::jsonb) FROM (\n%s\n  ) r;\n"+
					"$fn$;\n\n",
				g.Schema.DBSchema, res.Table, body))
			continue
		}

		// Non-binding refs: find all bindings that reference this resource.
		var unions []string
		for j := range g.Schema.Resources {
			binding := &g.Schema.Resources[j]
			if !binding.IsBinding() {
				continue
			}
			for _, ref := range binding.Refs {
				if ref.Target != res.Name {
					continue
				}
				for _, otherRef := range binding.Refs {
					if otherRef.Name == ref.Name {
						continue
					}
					q := fmt.Sprintf(
						"  SELECT jsonb_build_object(\n"+
							"    'name', b.%s,\n"+
							"    'namespace', COALESCE(b.namespace,''),\n"+
							"    'resType', '%s'\n"+
							"  )\n"+
							"  FROM %s.tbl_%s b\n"+
							"  JOIN %s.tbl_%s t ON t.name = b.%s\n"+
							"  WHERE t.uid = uid_val",
						otherRef.SQLColumn,
						otherRef.Target,
						g.Schema.DBSchema, binding.Table,
						g.Schema.DBSchema, res.Table, ref.SQLColumn)
					unions = append(unions, q)
				}
			}
		}

		if len(unions) == 0 {
			buf.WriteString(fmt.Sprintf(
				"CREATE OR REPLACE FUNCTION %s.resolve_%s_refs(uid_val uuid)\n"+
					"RETURNS jsonb LANGUAGE sql STABLE AS $fn$\n"+
					"  SELECT '[]'::jsonb;\n"+
					"$fn$;\n\n",
				g.Schema.DBSchema, res.Table))
		} else {
			body := strings.Join(unions, "\n  UNION ALL\n")
			buf.WriteString(fmt.Sprintf(
				"CREATE OR REPLACE FUNCTION %s.resolve_%s_refs(uid_val uuid)\n"+
					"RETURNS jsonb LANGUAGE sql STABLE AS $fn$\n"+
					"  SELECT COALESCE(jsonb_agg(r), '[]'::jsonb) FROM (\n%s\n  ) r;\n"+
					"$fn$;\n\n",
				g.Schema.DBSchema, res.Table, body))
		}
	}

	for i := range g.Schema.Resources {
		res := &g.Schema.Resources[i]
		data := g.sqlData(res)

		if res.IsComposite() {
			out, err := g.execTemplate("view-composite-"+res.Name, compTmpl, data)
			if err != nil {
				return err
			}
			buf.WriteString(out)
		} else {
			out, err := g.execTemplate("view-"+res.Name, tmpl, data)
			if err != nil {
				return err
			}
			buf.WriteString(out)
		}
		buf.WriteString("\n")
	}
	return g.writeGen("migrations/00005_views_gen.sql", gooseWrap(buf.String()))
}

func (g *Generator) generateSQLListers() error {
	tmpl, err := g.loadSQLTemplate("lister.sql.tmpl")
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("-- DO NOT EDIT: generated by sgctl\n")
	buf.WriteString(fmt.Sprintf("SET LOCAL search_path TO %s, public;\n\n", g.Schema.DBSchema))

	for i := range g.Schema.Resources {
		res := &g.Schema.Resources[i]
		if res.IsComposite() {
			// Composite: generate lister per subtype
			for _, st := range res.Subtypes {
				subRes := &ir.Resource{
					Name:  res.Name + st.Name,
					Scope: res.Scope,
					Table: st.Table,
					List:  res.List,
				}
				data := g.sqlData(subRes)
				out, err := g.execTemplate("lister-"+st.Name, tmpl, data)
				if err != nil {
					return err
				}
				buf.WriteString(out)
				buf.WriteString("\n")
			}
			continue
		}
		data := g.sqlData(res)
		out, err := g.execTemplate("lister-"+res.Name, tmpl, data)
		if err != nil {
			return err
		}
		buf.WriteString(out)
		buf.WriteString("\n")
	}
	return g.writeGen("migrations/00006_listers_gen.sql", gooseWrap(buf.String()))
}

func (g *Generator) generateSQLSyncers() error {
	tmpl, err := g.loadSQLTemplate("syncer.sql.tmpl")
	if err != nil {
		return err
	}
	compTmpl, err := g.loadSQLTemplate("syncer_composite.sql.tmpl")
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("-- DO NOT EDIT: generated by sgctl\n")
	buf.WriteString(fmt.Sprintf("SET LOCAL search_path TO %s, public;\n\n", g.Schema.DBSchema))

	for i := range g.Schema.Resources {
		res := &g.Schema.Resources[i]
		data := g.sqlData(res)

		if res.IsComposite() {
			out, err := g.execTemplate("syncer-composite-"+res.Name, compTmpl, data)
			if err != nil {
				return err
			}
			buf.WriteString(out)
		} else {
			out, err := g.execTemplate("syncer-"+res.Name, tmpl, data)
			if err != nil {
				return err
			}
			buf.WriteString(out)
		}
		buf.WriteString("\n")
	}

	if err := g.writeGen("migrations/00007_syncers_gen.sql", gooseWrap(buf.String())); err != nil {
		return err
	}

	scaffold := gooseWrap(fmt.Sprintf(`-- Custom SQL constraints and functions for %s schema
-- This file is generated once; add your custom CHECK constraints, triggers, functions here.

SET LOCAL search_path TO %s, public;

-- Example:
-- CREATE OR REPLACE FUNCTION %s.my_custom_check()
-- RETURNS trigger LANGUAGE plpgsql AS $$$$ BEGIN ... END; $$$$;
`, g.Schema.DBSchema, g.Schema.DBSchema, g.Schema.DBSchema))

	return g.writeScaffold("migrations/00008_custom_constraints.sql", scaffold)
}

// generateSQLRBAC creates RBAC tables and policies from roles.
func (g *Generator) generateSQLRBAC() error {
	if len(g.Schema.Roles) == 0 {
		return nil
	}

	var buf strings.Builder
	buf.WriteString("-- DO NOT EDIT: generated by sgctl\n")
	buf.WriteString(fmt.Sprintf("-- RBAC role definitions for %s\n\n", g.Schema.DBSchema))
	buf.WriteString(fmt.Sprintf("SET LOCAL search_path TO %s, public;\n\n", g.Schema.DBSchema))

	buf.WriteString(`CREATE TABLE IF NOT EXISTS ` + g.Schema.DBSchema + `.tbl_rbac_role (
    name     text PRIMARY KEY,
    resources text[] NOT NULL DEFAULT '{}',
    verbs    text[] NOT NULL DEFAULT '{}'
);

`)

	for _, role := range g.Schema.Roles {
		resources := make([]string, len(role.Resources))
		for i, r := range role.Resources {
			resources[i] = fmt.Sprintf("'%s'", r)
		}
		verbs := make([]string, len(role.Verbs))
		for i, v := range role.Verbs {
			verbs[i] = fmt.Sprintf("'%s'", v)
		}
		buf.WriteString(fmt.Sprintf(`INSERT INTO %s.tbl_rbac_role (name, resources, verbs) VALUES
    ('%s', ARRAY[%s], ARRAY[%s])
ON CONFLICT (name) DO UPDATE SET
    resources = EXCLUDED.resources,
    verbs     = EXCLUDED.verbs;

`, g.Schema.DBSchema, role.Name, strings.Join(resources, ", "), strings.Join(verbs, ", ")))
	}

	return g.writeGen("migrations/00009_rbac_gen.sql", gooseWrap(buf.String()))
}
