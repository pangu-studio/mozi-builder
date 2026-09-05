package mozi

// ============================================================================
// ServiceIR — service contract definition (v2 phase 3)
// ============================================================================
//
// ServiceIR describes the operations a module exposes, alongside ModelIR which
// describes the data. HTTP routes render to go-zero .api files; RPC methods
// render to gRPC via .proto. Message field numbers are part of the wire
// contract and are stored explicitly — never derived from field position.

// ServiceIR is the intermediate representation of a service contract document.
type ServiceIR struct {
	SchemaVersion int           `yaml:"schema_version,omitempty" json:"schema_version"`
	Module        string        `yaml:"module" json:"module"`
	Name          string        `yaml:"service" json:"service"` // PascalCase, e.g. ContentService
	Label         string        `yaml:"label" json:"label"`
	Description   string        `yaml:"description,omitempty" json:"description,omitempty"`
	Domain        string        `yaml:"domain,omitempty" json:"domain,omitempty"` // error-code/audit domain
	Messages      []MessageIR   `yaml:"messages,omitempty" json:"messages,omitempty"`
	HTTP          []HTTPRouteIR `yaml:"http,omitempty" json:"http,omitempty"`
	RPC           []RPCMethodIR `yaml:"rpc,omitempty" json:"rpc,omitempty"`
}

// MessageIR is a named message with explicitly numbered fields.
type MessageIR struct {
	Name            string           `yaml:"message" json:"message"`
	Description     string           `yaml:"description,omitempty" json:"description,omitempty"`
	Fields          []MessageFieldIR `yaml:"fields" json:"fields"`
	ReservedNumbers []int32          `yaml:"reserved_numbers,omitempty" json:"reserved_numbers,omitempty"`
	ReservedNames   []string         `yaml:"reserved_names,omitempty" json:"reserved_names,omitempty"`
}

// Message field type prefixes for non-scalar references.
const (
	// MessageRefPrefix references another message in the same service,
	// e.g. "message:DeckSummary".
	MessageRefPrefix = "message:"
	// ModelRefPrefix projects a subset of a model's fields into the message,
	// e.g. "model:content/Deck".
	ModelRefPrefix = "model:"
)

// MessageFieldIR is a single numbered message field. Number is part of the
// proto wire contract: it must never change once assigned, and numbers of
// deleted fields move to ReservedNumbers instead of being reused.
type MessageFieldIR struct {
	RenamedFrom string `yaml:"renamed_from,omitempty" json:"renamed_from,omitempty"`
	Name        string `yaml:"name" json:"name"`
	// Type is a scalar FieldType ("string", "int", ...), or a reference
	// prefixed with MessageRefPrefix / ModelRefPrefix.
	Type     string `yaml:"type" json:"type"`
	Repeated bool   `yaml:"repeated,omitempty" json:"repeated,omitempty"`
	Number   int32  `yaml:"number" json:"number"`
}

// HTTPRouteIR is one HTTP operation, rendered to a go-zero .api route.
type HTTPRouteIR struct {
	Name        string   `yaml:"name" json:"name"`
	Method      string   `yaml:"method" json:"method"`                       // GET | POST | PUT | DELETE | PATCH
	Path        string   `yaml:"path" json:"path"`                           // e.g. /api/content/decks
	Request     string   `yaml:"request,omitempty" json:"request,omitempty"` // message name; empty for bodyless ops
	Response    string   `yaml:"response" json:"response"`                   // message name
	Auth        string   `yaml:"auth,omitempty" json:"auth,omitempty"`       // jwt | admin | public
	Idempotency string   `yaml:"idempotency,omitempty" json:"idempotency,omitempty"`
	ErrorCodes  []string `yaml:"error_codes,omitempty" json:"error_codes,omitempty"` // references ErrorCodeIR.Code
}

// RPCMethodIR is one RPC operation, rendered to a gRPC method via .proto.
type RPCMethodIR struct {
	Name        string `yaml:"name" json:"name"`
	Request     string `yaml:"request" json:"request"`   // message name
	Response    string `yaml:"response" json:"response"` // message name
	Idempotency string `yaml:"idempotency,omitempty" json:"idempotency,omitempty"`
}

// GetMessage returns the message with the given name, or nil.
func (s *ServiceIR) GetMessage(name string) *MessageIR {
	for i := range s.Messages {
		if s.Messages[i].Name == name {
			return &s.Messages[i]
		}
	}
	return nil
}

// GetField returns the message field with the given name, or nil.
func (m *MessageIR) GetField(name string) *MessageFieldIR {
	for i := range m.Fields {
		if m.Fields[i].Name == name {
			return &m.Fields[i]
		}
	}
	return nil
}
