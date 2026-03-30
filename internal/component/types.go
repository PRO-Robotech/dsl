package component

// Component describes a field type's behavior for code generation.
// Components decouple templates from product-specific GoType knowledge.
type Component struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"` // scalar, enum, composite, ref, jsonb
	GoImport    string `yaml:"go_import,omitempty"`
	Test        Test   `yaml:"test"`
	ValidateTag string `yaml:"-"` // filled at resolve time from SpecField.Validate
}

// Test holds Go expressions used in generated test code.
// Expressions are Go text/template strings evaluated with SpecField context.
type Test struct {
	ValidExpr   string `yaml:"valid_expr"`
	ZeroExpr    string `yaml:"zero_expr"`
	InvalidExpr string `yaml:"invalid_expr,omitempty"`
}

// Resolved holds Go expressions for a specific field in two forms:
// - unqualified (for same-package usage): ValidExpr, ZeroExpr, InvalidExpr
// - qualified (for external-package usage): QValidExpr, QZeroExpr, QInvalidExpr
type Resolved struct {
	Kind        string // scalar, enum, composite, ref, jsonb
	GoImport    string
	ValidExpr   string // unqualified Go expression (same package)
	ZeroExpr    string
	InvalidExpr string
	QValidExpr   string // qualified Go expression (external package, with pkg prefix)
	QZeroExpr    string
	QInvalidExpr string
}

// Registry maps go_type names or component kinds to Component definitions.
type Registry struct {
	ByGoType map[string]*Component
	ByKind   map[string]*Component
}

func NewRegistry() *Registry {
	return &Registry{
		ByGoType: make(map[string]*Component),
		ByKind:   make(map[string]*Component),
	}
}
