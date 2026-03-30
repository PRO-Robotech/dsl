package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

type goTemplateData struct {
	Module       string
	DBSchema     string
	Resources    []ir.Resource
	Restrictions map[string]ir.Restriction
}

type goResTemplateData struct {
	Module string
	Res    *ir.Resource
}

func (g *Generator) goData() goTemplateData {
	return goTemplateData{
		Module:       g.Schema.Module,
		DBSchema:     g.Schema.DBSchema,
		Resources:    g.Schema.Resources,
		Restrictions: g.Schema.Restrictions,
	}
}

func (g *Generator) loadGoTemplate(name string) (string, error) {
	p := filepath.Join(g.tmplDir, "go", name)
	data, err := readFileBytes(p)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}
	return string(data), nil
}

// GenerateGo produces all Go source files.
func (g *Generator) GenerateGo() error {
	if err := g.generateGoDomain(); err != nil {
		return err
	}
	if err := g.generateGoRepository(); err != nil {
		return err
	}
	if err := g.generateGoTransport(); err != nil {
		return err
	}
	if err := g.generateGoRBAC(); err != nil {
		return err
	}
	if err := g.generateGoMain(); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateGoMain() error {
	tmpl, err := g.loadGoTemplate("scaffold_main.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("scaffold_main", tmpl, g.goData())
	if err != nil {
		return err
	}
	return g.writeScaffold("cmd/sgroups-backend/main.go", out)
}

func (g *Generator) generateGoDomain() error {
	data := g.goData()

	genFiles := []struct {
		tmpl string
		out  string
	}{
		{"domain_resource.go.tmpl", "internal/shared/domains/sgroups/resources_gen.go"},
		{"domain_types.go.tmpl", "internal/shared/domains/sgroups/types_gen.go"},
		{"res_type.go.tmpl", "internal/shared/domains/sgroups/res_type_gen.go"},
	}

	for _, gf := range genFiles {
		tmpl, err := g.loadGoTemplate(gf.tmpl)
		if err != nil {
			return err
		}
		out, err := g.execTemplate(gf.tmpl, tmpl, data)
		if err != nil {
			return err
		}
		if err := g.writeGen(gf.out, out); err != nil {
			return err
		}
	}

	// Generated validators (restrictions-based)
	if len(g.Schema.Restrictions) > 0 {
		tmpl, err := g.loadGoTemplate("validators_gen.go.tmpl")
		if err != nil {
			return err
		}
		vData := g.buildValidatorData()
		out, err := g.execTemplate("validators_gen", tmpl, vData)
		if err != nil {
			return err
		}
		if err := g.writeGen("internal/shared/domains/sgroups/validators_gen.go", out); err != nil {
			return err
		}
	}

	// Composite resource types
	var composites []ir.Resource
	for _, r := range g.Schema.Resources {
		if r.IsComposite() {
			composites = append(composites, r)
		}
	}
	if len(composites) > 0 {
		compData := struct {
			CompositeResources []ir.Resource
		}{CompositeResources: composites}
		tmplComp, err := g.loadGoTemplate("domain_composite.go.tmpl")
		if err != nil {
			return err
		}
		out, err := g.execTemplate("domain_composite", tmplComp, compData)
		if err != nil {
			return err
		}
		if err := g.writeGen("internal/shared/domains/sgroups/composite_gen.go", out); err != nil {
			return err
		}
	}

	// scaffold: validators.go (custom business logic)
	tmpl, err := g.loadGoTemplate("scaffold_validators.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("validators", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeScaffold("internal/shared/domains/sgroups/validators.go", out)
}

// validatorTemplateData is used for validators_gen.go.tmpl.
type validatorTemplateData struct {
	RestrictionList []restrictionEntry
	Resources       []ir.Resource
	restrictionMap  map[string]bool
}

type restrictionEntry struct {
	Name      string
	TitleName string
	Pattern   string
	MaxLength int
}

func (v validatorTemplateData) HasRestriction(name string) bool {
	return v.restrictionMap[name]
}

func (g *Generator) buildValidatorData() validatorTemplateData {
	vd := validatorTemplateData{
		Resources:      g.Schema.Resources,
		restrictionMap: make(map[string]bool),
	}
	for name, r := range g.Schema.Restrictions {
		vd.RestrictionList = append(vd.RestrictionList, restrictionEntry{
			Name:      name,
			TitleName: strings.Title(name), //nolint:staticcheck
			Pattern:   r.Pattern,
			MaxLength: r.MaxLength,
		})
		vd.restrictionMap[name] = true
	}
	return vd
}

type goRBACData struct {
	Module string
	Roles  []ir.Role
}

func (g *Generator) generateGoRBAC() error {
	if len(g.Schema.Roles) == 0 {
		return nil
	}
	data := goRBACData{Module: g.Schema.Module, Roles: g.Schema.Roles}
	tmpl, err := g.loadGoTemplate("rbac.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("rbac", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("internal/shared/rbac/rbac_gen.go", out)
}

func (g *Generator) generateGoRepository() error {
	data := g.goData()

	genFiles := []struct {
		tmplFile string
		outFile  string
	}{
		{"abstract.go.tmpl", "internal/sg-server/repository/abstract_gen.go"},
		// PG reader/writer stubs disabled until pgRepo is implemented:
		// {"reader_list.go.tmpl", "internal/sg-server/repository/pg-db-reader_list_gen.go"},
		// {"reader_watch.go.tmpl", "internal/sg-server/repository/pg-db-reader_watch_gen.go"},
		// {"writer_sync.go.tmpl", "internal/sg-server/repository/pg-db-writer_gen.go"},
		{"syncer_decl.go.tmpl", "internal/sg-server/repository/pg/syncer/syncers_gen.go"},
		{"pg_domain.go.tmpl", "internal/sg-server/repository/pg/domain/domain_gen.go"},
		{"pg_dto.go.tmpl", "internal/sg-server/repository/pg/dto/dto_gen.go"},
	}

	for _, gf := range genFiles {
		tmpl, err := g.loadGoTemplate(gf.tmplFile)
		if err != nil {
			return err
		}
		out, err := g.execTemplate(gf.tmplFile, tmpl, data)
		if err != nil {
			return err
		}
		if err := g.writeGen(gf.outFile, out); err != nil {
			return err
		}
	}

	// scaffold
	tmpl, err := g.loadGoTemplate("scaffold_pg_domain_custom.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("pg_domain_custom", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeScaffold("internal/sg-server/repository/pg/domain/domain_custom.go", out)
}

func (g *Generator) generateGoTransport() error {
	genTemplates := []struct {
		tmpl    string
		fileFmt string
	}{
		{"grpc_service.go.tmpl", "internal/sg-server/transport/grpc/service/%s/service_gen.go"},
		{"grpc_list.go.tmpl", "internal/sg-server/transport/grpc/service/%s/list_gen.go"},
		{"grpc_upsert.go.tmpl", "internal/sg-server/transport/grpc/service/%s/upsert_gen.go"},
		{"grpc_delete.go.tmpl", "internal/sg-server/transport/grpc/service/%s/delete_gen.go"},
		{"grpc_watch.go.tmpl", "internal/sg-server/transport/grpc/service/%s/watch_gen.go"},
		{"dto_domain2proto.go.tmpl", "internal/sg-server/transport/grpc/service/%s/dto/domain2proto_gen.go"},
		{"dto_proto2domain.go.tmpl", "internal/sg-server/transport/grpc/service/%s/dto/proto2domain_gen.go"},
	}

	scaffoldTemplates := []struct {
		tmpl    string
		fileFmt string
	}{
		{"scaffold_hooks.go.tmpl", "internal/sg-server/transport/grpc/service/%s/hooks.go"},
		{"scaffold_dto_domain2proto.go.tmpl", "internal/sg-server/transport/grpc/service/%s/dto/domain2proto.go"},
		{"scaffold_dto_proto2domain.go.tmpl", "internal/sg-server/transport/grpc/service/%s/dto/proto2domain.go"},
	}

	for i := range g.Schema.Resources {
		res := &g.Schema.Resources[i]
		if res.IsComposite() {
			continue
		}

		resDir := strings.ToLower(res.Name)
		data := goResTemplateData{Module: g.Schema.Module, Res: res}

		for _, gt := range genTemplates {
			tmpl, err := g.loadGoTemplate(gt.tmpl)
			if err != nil {
				return err
			}
			out, err := g.execTemplate(gt.tmpl+"-"+res.Name, tmpl, data)
			if err != nil {
				return err
			}
			outPath := fmt.Sprintf(gt.fileFmt, resDir)
			if err := g.writeGen(outPath, out); err != nil {
				return err
			}
		}

		for _, st := range scaffoldTemplates {
			tmpl, err := g.loadGoTemplate(st.tmpl)
			if err != nil {
				return err
			}
			out, err := g.execTemplate(st.tmpl+"-"+res.Name, tmpl, data)
			if err != nil {
				return err
			}
			outPath := fmt.Sprintf(st.fileFmt, resDir)
			if err := g.writeScaffold(outPath, out); err != nil {
				return err
			}
		}
	}

	return nil
}
