package domain

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"cnb.cool/mliev/push/message-push/app/model"
)

func TestResourceDiagnosticRedactsSecretsBeforeLogging(t *testing.T) {
	const secret = "private+secret/=&雪"
	const proof = "sensitive-proof-image"
	raw, _ := json.Marshal(map[string]any{"code": "9006", "msg": "超频 " + secret, "proof_image": proof, "nested": map[string]string{"AccessKey": "different-echoed-key"}, "id": json.Number("9007199254740993")})
	original := &RemoteResourceError{Code: "9006", Message: "超频 " + secret, RequestID: "upstream-id", Diagnostic: ResourceDiagnostic{HTTPStatus: 200, URL: "https://provider.invalid/list?secret=" + url.QueryEscape(secret), Method: "POST", ResponseBody: string(raw), Cause: "GET https://provider.invalid?secret=" + url.QueryEscape(secret)}}
	op := &ResourceOperation{Protocol: ResourceProtocol{Path: "/list"}, Fields: []ResourceField{{Name: "provider_fields.proof_image", Sensitive: true}}, Handler: func(context.Context, *model.ProviderAccount, ResourceInput) ([]RemoteResource, error) {
		return nil, original
	}}
	definition := &ResourceDefinition{Operations: map[ResourceAction]*ResourceOperation{ResourceQuery: op}}
	config, _ := json.Marshal(map[string]string{"secret": secret})
	_, err := definition.Execute(context.Background(), &model.ProviderAccount{ID: 1, ProviderCode: "test", Config: string(config)}, ResourceQuery, ResourceInput{ID: "11", ProviderFields: map[string]string{"proof_image": proof}})
	var remote *RemoteResourceError
	if !errors.As(err, &remote) {
		t.Fatalf("wrong error: %v", err)
	}
	if remote == original || original.Message != "超频 "+secret {
		t.Fatal("shared provider error was mutated")
	}
	combined := remote.Message + remote.Diagnostic.ResponseBody + remote.Diagnostic.Cause + remote.Diagnostic.URL
	for _, forbidden := range []string{secret, url.QueryEscape(secret), proof, "different-echoed-key"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("diagnostic contains secret %q", forbidden)
		}
	}
	if !strings.Contains(remote.Diagnostic.ResponseBody, "9007199254740993") {
		t.Fatal("diagnostic lost numeric precision")
	}
	if remote.Diagnostic.ProviderCode != "test" || remote.Diagnostic.AccountID != 1 || remote.Diagnostic.Operation != ResourceQuery || remote.Diagnostic.API != "/list" || remote.Diagnostic.ResourceID != "11" || remote.Diagnostic.URL != "https://provider.invalid/list" {
		t.Fatalf("wrong context: %+v", remote.Diagnostic)
	}
	encoded, _ := json.Marshal(remote)
	if strings.Contains(string(encoded), "ResponseBody") || strings.Contains(string(encoded), "HTTPStatus") || strings.Contains(string(encoded), "response_body") {
		t.Fatal("server diagnostics exposed in API JSON")
	}
}

func TestResourceDiagnosticBoundsMalformedAndLargeResponses(t *testing.T) {
	secret := "private-credential-value"
	for _, body := range []string{"<html>gateway error " + secret + "</html>", strings.Repeat("中", 2000) + secret} {
		remote := &RemoteResourceError{Diagnostic: ResourceDiagnostic{ResponseBody: body, Cause: secret}}
		redactResourceError(remote, &model.ProviderAccount{Config: `{"secret":"private-credential-value"}`}, nil, ResourceInput{})
		if len(remote.Diagnostic.ResponseBody) > ResourceDiagnosticLimit || !utf8.ValidString(remote.Diagnostic.ResponseBody) || strings.Contains(remote.Diagnostic.ResponseBody, secret) || strings.Contains(remote.Diagnostic.Cause, secret) {
			t.Fatalf("unsafe bounded diagnostic: %+v", remote.Diagnostic)
		}
	}
}
