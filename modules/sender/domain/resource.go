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
	Name     string        `json:"name"`
	Label    string        `json:"label"`
	Type     string        `json:"type"`
	Required bool          `json:"required"`
	Options  []FieldOption `json:"options,omitempty"`
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
}

type ResourceInput struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Content     string `json:"content"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// RemoteResource contains provider facts. Content is native, not canonical text.
type RemoteResource struct {
	ResourceInput
	AuditStatus int8   `json:"audit_status"`
	AuditReply  string `json:"audit_reply"`
}

// RemoteResourceError distinguishes a rejected request from an uncertain write.
type RemoteResourceError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Uncertain bool   `json:"uncertain"`
}

func (e *RemoteResourceError) Error() string { return e.Message }

type ResourceHandler func(context.Context, *model.ProviderAccount, ResourceInput) ([]RemoteResource, error)

type ResourceOperation struct {
	Protocol ResourceProtocol
	Fields   []ResourceField
	Handler  ResourceHandler
}

type ResourceDefinition struct {
	Operations   map[ResourceAction]*ResourceOperation
	Codec        TemplateCodec
	MatchAliases func(string) []string // Provider-declared aliases for linking legacy local resources.
}

type ResourceCapability struct {
	Create       bool                               `json:"create"`
	Update       bool                               `json:"update"`
	Delete       bool                               `json:"delete"`
	Query        bool                               `json:"query"`
	Fields       map[ResourceAction][]ResourceField `json:"fields"`
	CodecVersion string                             `json:"codec_version,omitempty"`
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
	if d.Codec != nil {
		c.CodecVersion = d.Codec.Version()
	}
	return c
}

func (d *ResourceDefinition) Execute(ctx context.Context, account *model.ProviderAccount, action ResourceAction, input ResourceInput) ([]RemoteResource, error) {
	if d == nil {
		return nil, ErrResourceUnsupported
	}
	op := d.Operations[action]
	if op == nil || op.Handler == nil {
		return nil, ErrResourceUnsupported
	}
	if (action == ResourceUpdate || action == ResourceDelete) && strings.TrimSpace(input.ID) == "" {
		return nil, errors.New("remote resource ID is required")
	}
	values := map[string]string{"id": input.ID, "name": input.Name, "content": input.Content, "category": input.Category, "description": input.Description}
	for _, f := range op.Fields {
		if f.Required && strings.TrimSpace(values[f.Name]) == "" {
			return nil, fmt.Errorf("%s不能为空", f.Label)
		}
		if len(f.Options) > 0 && values[f.Name] != "" {
			valid := false
			for _, o := range f.Options {
				if o.Value == values[f.Name] {
					valid = true
				}
			}
			if !valid {
				return nil, fmt.Errorf("%s不合法", f.Label)
			}
		}
	}
	return op.Handler(ctx, account, input)
}

func (d *ResourceDefinition) Validate(kind ResourceKind) error {
	if d == nil || len(d.Operations) == 0 {
		return errors.New("resource declaration has no operations")
	}
	if kind != ResourceTemplates && kind != ResourceSignatures {
		return errors.New("unknown resource kind")
	}
	if kind == ResourceTemplates && (d.Codec == nil || d.Codec.Version() == "") {
		return errors.New("template resource requires a versioned codec")
	}
	for action, op := range d.Operations {
		if action != ResourceQuery && action != ResourceCreate && action != ResourceUpdate && action != ResourceDelete {
			return errors.New("unknown resource action")
		}
		if op == nil || op.Handler == nil || op.Protocol.Path == "" || op.Protocol.SuccessField == "" || op.Protocol.SuccessValue == "" {
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

// TemplateCodec separates logical names from native placeholders and send order.
type TemplateCodec interface {
	Version() string
	Compile(string) (*CompiledTemplate, error)
	Decode(string, []VariableSlot) (*CompiledTemplate, error)
	Bind([]VariableSlot, map[string]string) ([]string, error)
}

type VariableSlot struct {
	Native string `json:"native"`
	Name   string `json:"name"`
}

type CompiledTemplate struct {
	Content       string         `json:"content"`
	NativeContent string         `json:"native_content"`
	Variables     []string       `json:"variables"`
	Slots         []VariableSlot `json:"slots"`
	Version       string         `json:"version"`
}
