package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
)

type aglTemplateData struct {
	Module    string
	Resources []aglResource
}

type aglResource struct {
	ir.Resource
	Spec aglSpec
}

type aglSpec struct {
	Fields []aglSpecField
}

type aglSpecField struct {
	ir.SpecField
	K8sGoType string
	JSONName  string
}

type aglResTemplateData struct {
	Module string
	Res    *aglResource
}

type aglAPIServiceData struct {
	APIGroup   string
	APIVersion string
	Namespace  string
}

func (g *Generator) loadAGLTemplate(name string) (string, error) {
	p := filepath.Join(g.tmplDir, "agl", name)
	data, err := readFileBytes(p)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}
	return string(data), nil
}

func (g *Generator) buildAGLData() aglTemplateData {
	data := aglTemplateData{Module: g.Schema.Module}
	for _, res := range g.Schema.Resources {
		if res.IsComposite() {
			continue
		}
		ar := aglResource{Resource: res}
		for _, f := range res.Spec.Fields {
			k8sType := mapGoTypeToK8s(f.GoType)
			jsonName := f.Name
			if f.ProtoField != "" {
				jsonName = f.ProtoField
			}
			ar.Spec.Fields = append(ar.Spec.Fields, aglSpecField{
				SpecField: f,
				K8sGoType: k8sType,
				JSONName:  jsonName,
			})
		}
		data.Resources = append(data.Resources, ar)
	}
	return data
}

func mapGoTypeToK8s(goType string) string {
	switch goType {
	case "PolicyAction":
		return "Action"
	case "bool":
		return "bool"
	case "IPNet":
		return "string"
	case "DualStackIPs":
		return "DualStackIPs"
	case "HostInfo":
		return "HostMetaInfo"
	case "TransportSpec":
		return "Transport"
	default:
		return goType
	}
}

// GenerateAGL produces all AGL (Aggregated API Layer) files.
func (g *Generator) GenerateAGL() error {
	data := g.buildAGLData()

	if err := g.generateAGLDomainTypes(); err != nil {
		return err
	}
	if err := g.generateAGLTypes(data); err != nil {
		return err
	}
	if err := g.generateAGLRegister(data); err != nil {
		return err
	}
	if err := g.generateAGLKnownTypes(data); err != nil {
		return err
	}
	if err := g.generateAGLDeepCopy(data); err != nil {
		return err
	}
	if err := g.generateAGLOpenAPI(data); err != nil {
		return err
	}
	if err := g.generateAGLClient(data); err != nil {
		return err
	}
	if err := g.generateAGLResourcesMap(data); err != nil {
		return err
	}
	if err := g.generateAGLConvertHelpers(data); err != nil {
		return err
	}
	if err := g.generateAGLPerResource(data); err != nil {
		return err
	}
	if err := g.generateAGLAPIServiceManifest(); err != nil {
		return err
	}
	if err := g.generateAGLInfra(data); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateAGLDomainTypes() error {
	tmpl, err := g.loadAGLTemplate("domain_types.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-domain-types", tmpl, nil)
	if err != nil {
		return err
	}
	return g.writeGen("pkg/apis/sgroups/v1alpha1/domain_types_gen.go", out)
}

func (g *Generator) generateAGLTypes(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("types.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-types", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("pkg/apis/sgroups/v1alpha1/types_gen.go", out)
}

func (g *Generator) generateAGLRegister(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("register.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-register", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("pkg/apis/sgroups/v1alpha1/register_gen.go", out)
}

func (g *Generator) generateAGLKnownTypes(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("known_types.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-known-types", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("pkg/apis/sgroups/v1alpha1/known_types_gen.go", out)
}

func (g *Generator) generateAGLDeepCopy(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("deepcopy.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-deepcopy", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("pkg/apis/sgroups/v1alpha1/zz_generated.deepcopy.go", out)
}

func (g *Generator) generateAGLOpenAPI(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("openapi.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-openapi", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("pkg/apis/sgroups/v1alpha1/openapi_gen.go", out)
}

func (g *Generator) generateAGLClient(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("client.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-client", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("pkg/client/client_gen.go", out)
}

func (g *Generator) generateAGLResourcesMap(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("resources_gen.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-resources", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("internal/apiserver/resources_gen.go", out)
}

func (g *Generator) generateAGLConvertHelpers(data aglTemplateData) error {
	tmpl, err := g.loadAGLTemplate("convert_helpers.go.tmpl")
	if err != nil {
		return err
	}
	out, err := g.execTemplate("agl-convert-helpers", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("internal/registry/convert/helpers_gen.go", out)
}

func (g *Generator) generateAGLPerResource(data aglTemplateData) error {
	backendTmpl, err := g.loadAGLTemplate("backend.go.tmpl")
	if err != nil {
		return err
	}
	storageTmpl, err := g.loadAGLTemplate("storage.go.tmpl")
	if err != nil {
		return err
	}
	convertTmpl, err := g.loadAGLTemplate("convert.go.tmpl")
	if err != nil {
		return err
	}
	scaffoldConvertTmpl, err := g.loadAGLTemplate("scaffold_convert.go.tmpl")
	if err != nil {
		return err
	}

	var subresourceTmpl string
	subresourceTmplContent, subErr := g.loadAGLTemplate("subresource_storage.go.tmpl")
	if subErr == nil {
		subresourceTmpl = subresourceTmplContent
	}

	for i := range data.Resources {
		res := &data.Resources[i]
		resDir := strings.ToLower(res.Name)
		resData := aglResTemplateData{Module: g.Schema.Module, Res: res}

		genFiles := []struct {
			tmpl    string
			content string
			path    string
		}{
			{"backend-" + res.Name, backendTmpl,
				fmt.Sprintf("internal/registry/%s/backend_gen.go", resDir)},
			{"storage-" + res.Name, storageTmpl,
				fmt.Sprintf("internal/registry/%s/storage_gen.go", resDir)},
			{"convert-" + res.Name, convertTmpl,
				fmt.Sprintf("internal/registry/convert/%s_gen.go", resDir)},
		}

		for _, gf := range genFiles {
			out, err := g.execTemplate(gf.tmpl, gf.content, resData)
			if err != nil {
				return err
			}
			if err := g.writeGen(gf.path, out); err != nil {
				return err
			}
		}

		if len(res.K8sSubresources) > 0 && subresourceTmpl != "" {
			out, err := g.execTemplate("subresource-"+res.Name, subresourceTmpl, resData)
			if err != nil {
				return err
			}
			if err := g.writeGen(
				fmt.Sprintf("internal/registry/%s/subresource_gen.go", resDir),
				out,
			); err != nil {
				return err
			}
		}

		scaffoldOut, err := g.execTemplate("scaffold-convert-"+res.Name, scaffoldConvertTmpl, resData)
		if err != nil {
			return err
		}
		if err := g.writeScaffold(
			fmt.Sprintf("internal/registry/convert/%s.go", resDir),
			scaffoldOut,
		); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateAGLInfra(data aglTemplateData) error {
	genTemplates := []struct {
		tmpl string
		path string
	}{
		{"generic_storage.go.tmpl", "internal/registry/generic/storage_gen.go"},
		{"base.go.tmpl", "internal/registry/base/base_gen.go"},
		{"errors.go.tmpl", "internal/registry/errors/errors_gen.go"},
		{"options.go.tmpl", "internal/registry/options/options_gen.go"},
	}
	for _, gt := range genTemplates {
		tmpl, err := g.loadAGLTemplate(gt.tmpl)
		if err != nil {
			return err
		}
		out, err := g.execTemplate("agl-"+gt.tmpl, tmpl, data)
		if err != nil {
			return err
		}
		if err := g.writeGen(gt.path, out); err != nil {
			return err
		}
	}

	scaffoldTemplates := []struct {
		tmpl string
		path string
	}{
		{"scaffold_apiserver.go.tmpl", "internal/apiserver/apiserver.go"},
		{"scaffold_main.go.tmpl", "cmd/sgroups-k8s-apiserver/main.go"},
	}
	for _, st := range scaffoldTemplates {
		tmpl, err := g.loadAGLTemplate(st.tmpl)
		if err != nil {
			return err
		}
		out, err := g.execTemplate("agl-"+st.tmpl, tmpl, data)
		if err != nil {
			return err
		}
		if err := g.writeScaffold(st.path, out); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateAGLAPIServiceManifest() error {
	tmpl, err := g.loadAGLTemplate("apiservice.yaml.tmpl")
	if err != nil {
		return err
	}
	data := aglAPIServiceData{
		APIGroup:   "sgroups.io",
		APIVersion: "v1alpha1",
		Namespace:  "sgroups-system",
	}
	out, err := g.execTemplate("agl-apiservice", tmpl, data)
	if err != nil {
		return err
	}
	return g.writeGen("deploy/apiservice_gen.yaml", out)
}
