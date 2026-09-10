package domain

import (
	"encoding/json"
	"html"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"cnb.cool/mliev/push/message-push/app/model"
)

const ResourceDiagnosticLimit = 4096

// ResourceDiagnostic describes the failed upstream call without request forms or credentials.
type ResourceDiagnostic struct {
	ProviderCode string
	AccountID    uint
	Operation    ResourceAction
	ResourceID   string
	API          string
	URL          string // Origin and path only, without authentication query parameters.
	Method       string
	HTTPStatus   int
	ResponseBody string
	Cause        string
}

func sensitiveResourceKey(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	for _, part := range []string{"secret", "password", "token", "authorization", "accesskey", "apikey", "appkey", "proofimage", "commissionimage"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func redactResourceError(remote *RemoteResourceError, account *model.ProviderAccount, fields []ResourceField, input ResourceInput) {
	var secrets []string
	if account != nil {
		config, _ := account.GetConfig()
		for key, value := range config {
			if text, ok := value.(string); ok && text != "" && sensitiveResourceKey(key) {
				secrets = append(secrets, text)
			}
		}
	}
	for _, field := range fields {
		if field.Sensitive && input.FieldValue(field.Name) != "" {
			secrets = append(secrets, input.FieldValue(field.Name))
		}
	}
	// Replace longer overlapping credentials before their prefixes. Include the
	// representations commonly embedded in JSON, URL errors and HTML responses.
	variants := map[string]bool{}
	for _, secret := range secrets {
		encoded, _ := json.Marshal(secret)
		for _, value := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret), html.EscapeString(secret), string(encoded[1 : len(encoded)-1])} {
			if value != "" {
				variants[value] = true
			}
		}
	}
	values := make([]string, 0, len(variants))
	for value := range variants {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	replace := func(value string) string {
		for _, secret := range values {
			value = strings.ReplaceAll(value, secret, "[redacted]")
		}
		return value
	}
	sanitize := func(text string) string {
		var decoded any
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		if json.Valid([]byte(text)) && decoder.Decode(&decoded) == nil {
			var scrub func(any) any
			scrub = func(value any) any {
				switch value := value.(type) {
				case map[string]any:
					for key, item := range value {
						if sensitiveResourceKey(key) {
							value[key] = "[redacted]"
						} else {
							value[key] = scrub(item)
						}
					}
				case []any:
					for i, item := range value {
						value[i] = scrub(item)
					}
				case string:
					return replace(value)
				}
				return value
			}
			if encoded, err := json.Marshal(scrub(decoded)); err == nil {
				text = string(encoded)
			}
		}
		return replace(text)
	}
	remote.Code = replace(remote.Code)
	remote.Message = sanitize(remote.Message)
	remote.RequestID = replace(remote.RequestID)
	remote.Diagnostic.ResourceID = replace(remote.Diagnostic.ResourceID)
	remote.Diagnostic.ResponseBody = LimitResourceDiagnostic(sanitize(remote.Diagnostic.ResponseBody))
	remote.Diagnostic.Cause = LimitResourceDiagnostic(sanitize(remote.Diagnostic.Cause))
	// Never retain a URL query, even if a provider echoed unrecognized auth fields.
	if u, err := url.Parse(remote.Diagnostic.URL); err == nil {
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		u.User = nil
		remote.Diagnostic.URL = replace(u.String())
	} else {
		remote.Diagnostic.URL = ""
	}
}

func LimitResourceDiagnostic(value string) string {
	value = strings.ToValidUTF8(value, "�")
	if len(value) <= ResourceDiagnosticLimit {
		return value
	}
	const suffix = "…[truncated]"
	end := ResourceDiagnosticLimit - len(suffix)
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + suffix
}
