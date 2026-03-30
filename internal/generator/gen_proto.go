package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

type ProtoField struct {
	Name     string
	Type     string
	Number   int
	IsMap    bool
	MapKey   string
	MapValue string
}

type ProtoResource struct {
	ir.Resource
	SpecFields []ProtoField
	RefFields  []ProtoField
}

type ProtoCompositeResource struct {
	ir.Resource
	Subtypes []ir.Subtype
}

type ProtoExtraMethod struct {
	ResourceName string
	MethodName   string
	HTTPPath     string
}

type protoTemplateData struct {
	Module             string
	Project            ir.ProjectConfig
	Resources          []ProtoResource
	CompositeResources []ProtoCompositeResource
	ExtraMethods       []ProtoExtraMethod
}

func (g *Generator) buildProtoData() protoTemplateData {
	data := protoTemplateData{
		Module:  g.Schema.Module,
		Project: g.Schema.Project,
	}

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
			if f.Repeated {
				pt = "repeated " + pt
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
	if err := g.generateGoogleAPIProtos(); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateGoogleAPIProtos() error {
	annotations := `syntax = "proto3";
package google.api;
import "google/api/http.proto";
import "google/protobuf/descriptor.proto";
option go_package = "google.golang.org/genproto/googleapis/api/annotations;annotations";
extend google.protobuf.MethodOptions { HttpRule http = 72295728; }
`
	if err := g.writeGen("api/proto/google/api/annotations.proto", annotations); err != nil {
		return err
	}

	http := `syntax = "proto3";
package google.api;
option go_package = "google.golang.org/genproto/googleapis/api/annotations;annotations";
message Http { repeated HttpRule rules = 1; bool fully_decode_reserved_expansion = 2; }
message HttpRule {
  string selector = 1;
  oneof pattern { string get = 2; string put = 3; string post = 4; string delete = 5; string patch = 6; CustomHttpPattern custom = 8; }
  string body = 7; string response_body = 12; repeated HttpRule additional_bindings = 11;
}
message CustomHttpPattern { string kind = 1; string path = 2; }
`
	return g.writeGen("api/proto/google/api/http.proto", http)
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
	proj := g.Schema.Project
	return g.writeGen(fmt.Sprintf("api/proto/%s/v1/domains_gen.proto", proj.Name), out)
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
	proj := g.Schema.Project
	return g.writeGen(fmt.Sprintf("api/proto/%s/v1/queries_gen.proto", proj.Name), out)
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
	proj := g.Schema.Project
	return g.writeGen(fmt.Sprintf("api/proto/%s/v1/services_gen.proto", proj.Name), out)
}

func resPlural(name string) string {
	sn := toSnakeCase(name)
	if strings.HasSuffix(sn, "s") {
		return sn + "es"
	}
	return sn + "s"
}

func resGoPlural(name string) string {
	return goFieldName(resPlural(name))
}

func httpPathRes(res ProtoResource) string {
	if res.HTTPPath != "" {
		return res.HTTPPath
	}
	return strings.ToLower(res.Name) + "s"
}

func httpPathComposite(res ProtoCompositeResource) string {
	if res.HTTPPath != "" {
		return res.HTTPPath
	}
	return strings.ToLower(res.Name) + "s"
}
