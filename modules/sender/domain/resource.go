package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cnb.cool/mliev/push/message-push/app/model"
)

type ResourceKind string
type ResourceAction string

const (
	ResourceTemplates  ResourceKind   = "templates"
	ResourceSignatures ResourceKind   = "signatures"
	ResourceQuery      ResourceAction = "query"
	ResourceCreate     ResourceAction = "create"
	ResourceUpdate     ResourceAction = "update"
	ResourceDelete     ResourceAction = "delete"
)

var ErrResourceUnsupported = errors.New("provider resource operation is not supported")

// ResourceField describes the normalized form, never an upstream credential.
type ResourceField struct {
	Name         string            `json:"name"`
	Label        string            `json:"label"`
	Type         string            `json:"type"`
	Required     bool              `json:"required"`
	Options      []FieldOption     `json:"options,omitempty"`
	Sensitive    bool              `json:"sensitive,omitempty"`
	ReadOnly     bool              `json:"read_only,omitempty"`
	Help         string            `json:"help,omitempty"`
	RequiredWhen map[string]string `json:"required_when,omitempty"`
}

// ResourceProtocol keeps operation-specific wire names and success semantics in code.
type ResourceProtocol struct {
	Path           string
	DetailPath     string
	DetailIDField  string
	RequestFields  map[string]string
	ResponseFields map[string][]string
	AuditStatuses  map[string]int8 // Upstream audit value -> normalized audit state; absent values stay unknown.
	SuccessField   string
	SuccessValue   string
	DataField      string
	ErrorField     string // SDK APIs report success by the absence of this error envelope.
}

type ResourceInput struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name"`
	Content        string            `json:"content"`
	Category       string            `json:"category"`
	Description    string            `json:"description"`
	ProviderFields map[string]string `json:"provider_fields,omitempty"`
}

func (i ResourceInput) FieldValue(name string) string {
	if key, ok := strings.CutPrefix(name, "provider_fields."); ok {
		return i.ProviderFields[key]
	}
	return map[string]string{"id": i.ID, "name": i.Name, "content": i.Content, "category": i.Category, "description": i.Description}[name]
}

// PublicCopy excludes submission-only material from mirrors, previews and hashes.
func (i ResourceInput) PublicCopy(fields []ResourceField) ResourceInput {
	copy := i
	copy.ProviderFields = nil
	for _, field := range fields {
		key, provider := strings.CutPrefix(field.Name, "provider_fields.")
		if provider && !field.Sensitive && i.ProviderFields[key] != "" {
			if copy.ProviderFields == nil {
				copy.ProviderFields = map[string]string{}
			}
			copy.ProviderFields[key] = i.ProviderFields[key]
		}
	}
	return copy
}

// RemoteResource contains provider facts. Content is native, not canonical text.
type RemoteResource struct {
	ResourceInput
	AuditStatus      int8           `json:"audit_status"`
	AuditReply       string         `json:"audit_reply"`
	ProviderMetadata map[string]any `json:"provider_metadata,omitempty"`
	RequestID        string         `json:"-"` // Diagnostic only; never part of an optimistic resource version.
}

// RemoteResourceError distinguishes a rejected request from an uncertain write.
type RemoteResourceError struct {
	Code       string             `json:"code"`
	Message    string             `json:"message"`
	Uncertain  bool               `json:"uncertain"`
	RequestID  string             `json:"request_id,omitempty"`
	Diagnostic ResourceDiagnostic `json:"-"` // Sanitized server-side diagnostics, never exposed in API responses.
}

func (e *RemoteResourceError) Error() string {
	if e.RequestID != "" {
		return e.Message + "（RequestId: " + e.RequestID + "）"
	}
	return e.Message
}

type ResourceHandler func(context.Context, *model.ProviderAccount, ResourceInput) ([]RemoteResource, error)

type ResourceOperation struct {
	Protocol      ResourceProtocol
	Fields        []ResourceField
	Handler       ResourceHandler
	ValidateInput func(ResourceInput) error
	// ValidateCurrent runs against freshly queried facts before suspending a mirror.
	ValidateCurrent    func(RemoteResource) error
	SuspendBeforeWrite bool
}

type ResourceDefinition struct {
	Operations   map[ResourceAction]*ResourceOperation
	MatchAliases func(string) []string // Provider-declared aliases for linking legacy local resources.
	// AuditOrderID identifies the review created by a write, when provided by the upstream API.
	AuditOrderID func(RemoteResource) string
}

func (d *ResourceDefinition) OperationErrors(resource RemoteResource) map[ResourceAction]string {
	result := map[ResourceAction]string{}
	for action, op := range d.Operations {
		if op != nil && op.ValidateCurrent != nil {
			if err := op.ValidateCurrent(resource); err != nil {
				result[action] = err.Error()
			}
		}
	}
	return result
}

type ResourceCapability struct {
	Create bool                               `json:"create"`
	Update bool                               `json:"update"`
	Delete bool                               `json:"delete"`
	Query  bool                               `json:"query"`
	Fields map[ResourceAction][]ResourceField `json:"fields"`
}

func (d *ResourceDefinition) Capability() ResourceCapability {
	c := ResourceCapability{Fields: map[ResourceAction][]ResourceField{}}
	if d == nil {
		return c
	}
	for action, op := range d.Operations {
		if op == nil || op.Handler == nil {
			continue
		}
		c.Fields[action] = op.Fields
		switch action {
		case ResourceCreate:
			c.Create = true
		case ResourceUpdate:
			c.Update = true
		case ResourceDelete:
			c.Delete = true
		case ResourceQuery:
			c.Query = true
		}
	}
	return c
}

func (d *ResourceDefinition) Execute(ctx context.Context, account *model.ProviderAccount, action ResourceAction, input ResourceInput) ([]RemoteResource, error) {
	if err := d.ValidateInput(action, input); err != nil {
		return nil, err
	}
	op := d.Operations[action]
	rows, err := op.Handler(ctx, account, input)
	var remote *RemoteResourceError
	if errors.As(err, &remote) {
		copy := *remote
		copy.Diagnostic.Operation = action
		copy.Diagnostic.API = op.Protocol.Path
		copy.Diagnostic.ResourceID = input.ID
		if account != nil {
			copy.Diagnostic.ProviderCode = account.ProviderCode
			copy.Diagnostic.AccountID = account.ID
		}
		redactResourceError(&copy, account, op.Fields, input)
		err = &copy
	}
	return rows, err
}

// ValidateInput performs all form/capability validation before a write can suspend bindings.
func (d *ResourceDefinition) ValidateInput(action ResourceAction, input ResourceInput) error {
	if d == nil {
		return ErrResourceUnsupported
	}
	op := d.Operations[action]
	if op == nil || op.Handler == nil {
		return ErrResourceUnsupported
	}
	if (action == ResourceUpdate || action == ResourceDelete) && strings.TrimSpace(input.ID) == "" {
		return errors.New("remote resource ID is required")
	}
	for _, f := range op.Fields {
		value := input.FieldValue(f.Name)
		required := f.Required
		if len(f.RequiredWhen) > 0 {
			matches := true
			for name, expected := range f.RequiredWhen {
				if input.FieldValue(name) != expected {
					matches = false
				}
			}
			required = required || matches
		}
		if required && strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s不能为空", f.Label)
		}
		if len(f.Options) > 0 && value != "" {
			valid := false
			for _, o := range f.Options {
				if o.Value == value {
					valid = true
				}
			}
			if !valid {
				return fmt.Errorf("%s不合法", f.Label)
			}
		}
	}
	if op.ValidateInput != nil {
		return op.ValidateInput(input)
	}
	return nil
}

func (d *ResourceDefinition) Validate(kind ResourceKind) error {
	if d == nil || len(d.Operations) == 0 {
		return errors.New("resource declaration has no operations")
	}
	if kind != ResourceTemplates && kind != ResourceSignatures {
		return errors.New("unknown resource kind")
	}
	for action, op := range d.Operations {
		if action != ResourceQuery && action != ResourceCreate && action != ResourceUpdate && action != ResourceDelete {
			return errors.New("unknown resource action")
		}
		if op == nil || op.Handler == nil || op.Protocol.Path == "" || (op.Protocol.ErrorField == "" && (op.Protocol.SuccessField == "" || op.Protocol.SuccessValue == "")) {
			return fmt.Errorf("incomplete %s resource declaration", action)
		}
		if action != ResourceDelete && (op.Protocol.DataField == "" || len(op.Protocol.ResponseFields["id"]) == 0) {
			return fmt.Errorf("%s requires response identity mapping", action)
		}
		if (action == ResourceUpdate || action == ResourceDelete) && op.Protocol.RequestFields["id"] == "" {
			return fmt.Errorf("%s requires request identity mapping", action)
		}
		for _, field := range op.Fields {
			if field.Name == "" || op.Protocol.RequestFields[field.Name] == "" {
				return fmt.Errorf("missing wire mapping for field %s", field.Name)
			}
		}
	}
	return nil
}

func (p *ProviderMeta) ResourceCapabilities() map[ResourceKind]ResourceCapability {
	result := map[ResourceKind]ResourceCapability{}
	for kind, definition := range p.Resources {
		result[kind] = definition.Capability()
	}
	return result
}
