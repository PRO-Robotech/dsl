package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

// ProtoField represents a field in a proto message with its field number.
type ProtoField struct {
	Name     string
	Type     string
	Number   int
	IsMap    bool
	MapKey   string
	MapValue string
}

// ProtoResource is a per-resource data structure for proto templates.
type ProtoResource struct {
	ir.Resource
	SpecFields []ProtoField
	RefFields  []ProtoField
}

// ProtoCompositeResource represents a composite resource in proto.
type ProtoCompositeResource struct {
	ir.Resource
	Subtypes []ir.Subtype
}

// ProtoExtraMethod represents an extra gRPC method for proto generation.
type ProtoExtraMethod struct {
	ResourceName string
	MethodName   string
	HTTPPath     string
}

type protoTemplateData struct {
	Module             string
	Resources          []ProtoResource
	CompositeResources []ProtoCompositeResource
	ExtraMethods       []ProtoExtraMethod
}

func (g *Generator) buildProtoData() protoTemplateData {
	data := protoTemplateData{Module: g.Schema.Module}

	for i := range g.Schema.Resources {
		res := &g.Schema.Resources[i]
		if res.IsComposite() {
			data.CompositeResources = append(data.CompositeResources, ProtoCompositeResource{
				Resource: *res,
				Subtypes: res.Subtypes,
			})
			continue
		}

		pr := ProtoResource{Resource: *res}
		fieldNum := 10

		for _, ref := range res.Refs {
			pr.RefFields = append(pr.RefFields, ProtoField{
				Name:   toSnakeCase(ref.Name),
				Type:   "string",
				Number: fieldNum,
			})
			fieldNum++
		}

		for _, f := range res.Spec.Fields {
			pt := f.ProtoType
			if pt == "" {
				pt = "string"
			}
			pr.SpecFields = append(pr.SpecFields, ProtoField{
				Name:   toSnakeCase(f.Name),
				Type:   pt,
				Number: fieldNum,
			})
			fieldNum++
		}

		data.Resources = append(data.Resources, pr)

		for _, m := range res.ExtraGRPCMethods {
			data.ExtraMethods = append(data.ExtraMethods, ProtoExtraMethod{
				ResourceName: res.Name,
				MethodName:   m.Name,
				HTTPPath:     m.HTTPPath,
			})
		}
	}

	return data
}

func (g *Generator) loadProtoTemplate(name string) (string, error) {
	p := filepath.Join(g.tmplDir, "proto", name)
	data, err := readFileBytes(p)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}
	return string(data), nil
}

// GenerateProto produces all .proto files.
func (g *Generator) GenerateProto() error {
	data := g.buildProtoData()

	if err := g.generateProtoDomains(data); err != nil {
		return err
	}
	if err := g.generateProtoQueries(data); err != nil {
		return err
	}
	if err := g.generateProtoServices(data); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateProtoDomains(data protoTemplateData) error {
	tmpl, err := g.loadProtoTemplate("domain.proto.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("domain.proto", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("api/proto/sgroups/v1/domains_gen.proto", out)
}

func (g *Generator) generateProtoQueries(data protoTemplateData) error {
	tmpl, err := g.loadProtoTemplate("queries.proto.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("queries.proto", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("api/proto/sgroups/v1/queries_gen.proto", out)
}

func (g *Generator) generateProtoServices(data protoTemplateData) error {
	tmpl, err := g.loadProtoTemplate("services.proto.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("services.proto", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("api/proto/sgroups/v1/services_gen.proto", out)
}

// resPlural returns a lowercase plural form for proto field names.
func resPlural(name string) string {
	sn := toSnakeCase(name)
	if strings.HasSuffix(sn, "s") {
		return sn + "es"
	}
	return sn + "s"
}

// resGoPlural returns the CamelCase plural form used by protobuf Go getters.
func resGoPlural(name string) string {
	return goFieldName(resPlural(name))
}

// httpPathRes returns the REST v2 path for a resource.
func httpPathRes(res ProtoResource) string {
	if res.HTTPPath != "" {
		return res.HTTPPath
	}
	return strings.ToLower(res.Name) + "s"
}

// httpPathComposite returns the REST v2 path for a composite resource.
func httpPathComposite(res ProtoCompositeResource) string {
	if res.HTTPPath != "" {
		return res.HTTPPath
	}
	return strings.ToLower(res.Name) + "s"
}
