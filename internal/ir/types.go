package ir

// Schema is the top-level internal representation parsed from YAML DSL.
type Schema struct {
	Version      string
	Module       string
	DBSchema     string
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

// CustomType represents a user-defined SQL/PG type (enum, composite, domain).
type CustomType struct {
	Name           string
	Kind           string   // enum, composite, domain
	Values         []string // for enums
	Fields         []TypeField // for composites
	OneOfBy        string // for composites: discriminator field (e.g. "proto")
	OneOfVariants  map[string][]string // group → discriminator values, e.g. {"tcp_udp": ["TCP","UDP"], "icmp": ["ICMP"]}
	BaseType       string
	Constraints    []string
}

type TypeField struct {
	Name       string
	Type       string
	OneOfGroup string // e.g. "tcp_udp", "icmp" — mutually exclusive groups
}

// Resource is the central IR node — one per entity in the system.
type Resource struct {
	Name        string
	Scope       string // cluster, namespaced
	Kind        string // resource (default), binding, composite
	Table       string
	IndexPrefix string
	HTTPPath    string // REST v2 base path, e.g. "hosts", "ag", "network-bindings"

	Spec            ResourceSpec
	Immutable       []string
	HasBindingRev   bool
	Constraints     []Constraint
	Triggers        []TriggerDef
	Refs            []RefDef        // for binding resources
	CascadeRev      []CascadeRevDef // binding_rev triggers on parents
	List            ListConfig
	Events          EventConfig
	ExtraSyncers    []ExtraSyncer
	ExtraGRPCMethods  []ExtraGRPCMethod
	K8sSubresources   []K8sSubresource
	Subtypes          []Subtype // for composite resources (Rule)
}

// Derived helpers (computed during IR enrichment, not parsed from YAML).

func (r *Resource) IsCluster() bool   { return r.Scope == "cluster" }
func (r *Resource) IsNamespaced() bool { return r.Scope == "namespaced" }
func (r *Resource) IsBinding() bool    { return r.Kind == "binding" }
func (r *Resource) IsComposite() bool  { return r.Kind == "composite" }
func (r *Resource) IsStandard() bool         { return r.Kind == "" || r.Kind == "resource" }
func (r *Resource) HasK8sSubresources() bool { return len(r.K8sSubresources) > 0 }

// ResourceType returns the PG enum value for this resource (e.g. "Namespace", "AddressGroup").
func (r *Resource) ResourceType() string { return r.Name }

type ResourceSpec struct {
	Fields []SpecField
}

type SpecField struct {
	Name       string
	SQLType    string
	SQLColumn  string // override column name, defaults to Name
	GoType     string
	ProtoType  string
	ProtoField string // override proto field name
	Default    string
	Validate   string
	OutputOnly bool
}

// ColumnName returns the SQL column name (falls back to field Name).
func (f *SpecField) ColumnName() string {
	if f.SQLColumn != "" {
		return f.SQLColumn
	}
	return f.Name
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
	Target     string // target resource name (e.g. AddressGroup)
	SQLColumn  string
	SQLFKTable string
}

type CascadeRevDef struct {
	ParentTable string
	RefColumn   string
}

type ListConfig struct {
	Selectors string // res, custom
	HasRefs   bool
	Parallel  bool
	RefTypes  []string // resource types that can appear as refs (e.g. ["Host", "AddressGroup"])
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
	HTTPPath string // REST v2 endpoint override, e.g. "upd-iplist"
	HTTPVerb string // HTTP verb, defaults to POST
}

// K8sSubresource defines an AGL sub-resource (e.g. hosts/{name}/metadata).
type K8sSubresource struct {
	Name       string // URL path segment: "ips", "metadata"
	Field      string // spec field name: "ips", "meta_info"
	GoType     string // domain Go type: "DualStackIPs", "HostInfo"
	K8sType    string // K8s API type: "DualStackIPs", "HostMetaInfo"
	GRPCMethod string // backend gRPC method to call: "UpdIPs", "UpdMetaInfo"
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
