package ir

// ProjectConfig holds project-level constants derived from schema.
type ProjectConfig struct {
	Name         string // e.g. "sgroups"
	Module       string // e.g. "github.com/PRO-Robotech/sgroups"
	APIGroup     string // e.g. "sgroups.io"
	APIVersion   string // e.g. "v1alpha1"
	ProtoPackage string // e.g. "sgroups.v1"
	DBSchema     string // e.g. "sgroups"
}

// Schema is the top-level internal representation parsed from YAML DSL.
type Schema struct {
	Version      string
	Module       string // shortcut for Project.Module
	DBSchema     string // shortcut for Project.DBSchema
	Project      ProjectConfig
	Types        []CustomType
	Resources    []Resource
	Roles        []Role
	Restrictions map[string]Restriction
}

// Restriction describes metadata field validation rules (name, uid, namespace, etc.).
type Restriction struct {
	Pattern     string
	MaxLength   int
	Description string
}

// TypeMapping stores the SQL, Go, and Proto representations of a type.
type TypeMapping struct {
	SQL   string // e.g. "cidr", "jsonb", "sgroups.policy_action"
	Go    string // e.g. "IPNet", "PolicyAction", "bool"
	Proto string // e.g. "string", "DualStackIPs"
}

// CustomType represents a user-defined type (enum, composite, scalar).
type CustomType struct {
	Name          string
	Kind          string              // enum, composite, scalar
	Values        []string            // for enums
	Fields        []TypeField         // for composites
	OneOfBy       string              // for composites: discriminator field
	OneOfVariants map[string][]string // group -> discriminator values
	BaseType      string
	Constraints   []string
	Mapping       TypeMapping // sql/go/proto mapping
	K8sName       string      // PascalCase K8s type name
	ProtoName     string      // proto message name
}

type TypeField struct {
	Name       string
	Type       string
	OneOfGroup string
}

// Resource is the central IR node -- one per entity in the system.
type Resource struct {
	Name        string
	Scope       string // cluster, namespaced
	Kind        string // resource (default), binding, composite
	Table       string
	IndexPrefix string
	HTTPPath    string

	Spec             ResourceSpec
	Immutable        []string
	HasBindingRev    bool
	CrossNamespace   bool // for bindings that allow cross-namespace refs (e.g. ServiceBinding)
	Constraints      []Constraint
	Triggers         []TriggerDef
	Refs             []RefDef
	CascadeRev       []CascadeRevDef
	List             ListConfig
	Events           EventConfig
	ExtraSyncers     []ExtraSyncer
	ExtraGRPCMethods []ExtraGRPCMethod
	K8sSubresources  []K8sSubresource
	Subtypes         []Subtype
}

func (r *Resource) IsCluster() bool        { return r.Scope == "cluster" }
func (r *Resource) IsNamespaced() bool      { return r.Scope == "namespaced" }
func (r *Resource) IsBinding() bool         { return r.Kind == "binding" }
func (r *Resource) IsComposite() bool       { return r.Kind == "composite" }
func (r *Resource) IsStandard() bool        { return r.Kind == "" || r.Kind == "resource" }
func (r *Resource) HasK8sSubresources() bool { return len(r.K8sSubresources) > 0 }
func (r *Resource) ResourceType() string    { return r.Name }

type ResourceSpec struct {
	Fields []SpecField
}

// SpecField describes a field in a resource's spec.
// Type refers to a name in the types registry (e.g. "policyAction", "cidr", "bool").
// The parser resolves the type and populates SQLType/GoType/ProtoType from the mapping.
type SpecField struct {
	Name       string
	Type       string // reference to type name: "policyAction", "cidr", "bool"
	SQLType    string // resolved from TypeMapping.SQL
	SQLColumn  string // override SQL column name
	GoType     string // resolved from TypeMapping.Go
	ProtoType  string // resolved from TypeMapping.Proto
	ProtoField string // override proto field name (e.g. "CIDR", "IPs")
	JSONName   string // override JSON field name (e.g. "CIDR", "IPs")
	Default    string
	Validate   string
	Repeated     bool
	Selector     bool
	OutputOnly   bool
	TestSQLValue string   // override SQL value for test data generation
	EnumValues   []string // populated from enum type values
}

// ColumnName returns the SQL column name (falls back to snake_case of Name).
func (f *SpecField) ColumnName() string {
	if f.SQLColumn != "" {
		return f.SQLColumn
	}
	return toSnakeCase(f.Name)
}

func toSnakeCase(s string) string {
	var result []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(r+('a'-'A')))
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

type Constraint struct {
	Name  string
	Check string
}

type TriggerDef struct {
	Name     string
	Timing   string
	Function string
}

type RefDef struct {
	Name       string
	Target     string
	SQLColumn  string
	SQLFKTable string
	Selector   bool
}

type CascadeRevDef struct {
	ParentTable string
	RefColumn   string
}

type ListConfig struct {
	Selectors string
	HasRefs   bool
	Parallel  bool
	RefTypes  []string
}

type EventConfig struct {
	Channel  string
	Parallel bool
}

type ExtraSyncer struct {
	Name    string
	SQLFunc string
	Fields  []string
}

type ExtraGRPCMethod struct {
	Name     string
	Scaffold bool
	HTTPPath string
	HTTPVerb string
}

// K8sSubresource defines an AGL sub-resource (e.g. hosts/{name}/metadata).
// GoType and K8sType are resolved from the linked spec field's type mapping.
type K8sSubresource struct {
	Name       string // URL path segment: "ips", "metadata"
	SpecField  string // spec field name: "ips", "metaInfo"
	GoType     string // resolved: domain Go type
	K8sType    string // resolved: K8s API type
	GRPCMethod string // backend gRPC method: "UpdIPs", "UpdMetaInfo"
}

type Subtype struct {
	Name  string
	Table string
}

type Role struct {
	Name      string
	Resources []string
	Verbs     []string
}
